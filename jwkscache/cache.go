/*
Copyright 2023 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package jwkscache contains utils to manage a cache of a JWK Set (via jwk.Set).
// It supports retrieving a JWKS from:
//
// - A path on the local disk. This is watched with fsnotify to automatically reload the JWKS when the file changes on disk.
// - A HTTP(S) URL. This is automatically refreshed if a caller requests a key that isn't in the cached set.
// - A JWKS passed during initialization, optionally base64-encoded.
package jwkscache

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lestrrat-go/httprc"
	"github.com/lestrrat-go/jwx/v2/jwk"

	"github.com/dapr/kit/fswatcher"
	"github.com/dapr/kit/logger"
)

const (
	// Timeout for network requests.
	defaultRequestTimeout = 30 * time.Second
	// Minimum interval for refreshing a JWKS from a URL if a key is not found in the cache.
	defaultMinRefreshInterval = 10 * time.Minute
)

// JWKSCache is a cache of JWKS objects.
// It fetches a JWKS object from a file on disk, a URL, or from a value passed as-is.
// TODO: Move this to dapr/kit and use it for the JWKS crypto component too
type JWKSCache struct {
	location           string
	requestTimeout     time.Duration
	minRefreshInterval time.Duration

	jwks    jwk.Set
	logger  logger.Logger
	lock    sync.RWMutex
	client  *http.Client
	running atomic.Bool
	initCh  chan error
}

// NewJWKSCache creates a new JWKSCache object.
func NewJWKSCache(location string, logger logger.Logger) *JWKSCache {
	return &JWKSCache{
		location: location,
		logger:   logger,

		requestTimeout:     defaultRequestTimeout,
		minRefreshInterval: defaultMinRefreshInterval,

		initCh: make(chan error, 1),
	}
}

// Start the JWKS cache.
// This method blocks until the context is canceled.
func (c *JWKSCache) Start(ctx context.Context) error {
	if !c.running.CompareAndSwap(false, true) {
		return errors.New("cache is already running")
	}
	defer c.running.Store(false)

	// Init the cache
	err := c.initCache(ctx)
	if err != nil {
		err = fmt.Errorf("failed to init cache: %w", err)
		// Store the error in the initCh, then close it
		c.initCh <- err
		close(c.initCh)
		return err
	}

	// Close initCh
	close(c.initCh)

	// Block until context is canceled
	<-ctx.Done()

	return nil
}

// SetRequestTimeout sets the timeout for network requests.
func (c *JWKSCache) SetRequestTimeout(requestTimeout time.Duration) {
	c.requestTimeout = requestTimeout
}

// SetMinRefreshInterval sets the minimum interval for refreshing a JWKS from a URL if a key is not found in the cache.
func (c *JWKSCache) SetMinRefreshInterval(minRefreshInterval time.Duration) {
	c.minRefreshInterval = minRefreshInterval
}

// SetHTTPClient sets the HTTP client object to use.
func (c *JWKSCache) SetHTTPClient(client *http.Client) {
	c.client = client
}

// KeySet returns the jwk.Set with the current keys.
func (c *JWKSCache) KeySet() jwk.Set {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.jwks
}

// WaitForCacheReady pauses until the cache is ready (the initial JWKS has been fetched) or the passed ctx is canceled.
// It will return the initialization error.
func (c *JWKSCache) WaitForCacheReady(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-c.initCh:
		return err
	}
}

// Init the cache from the given location.
func (c *JWKSCache) initCache(ctx context.Context) error {
	if len(c.location) == 0 {
		return errors.New("property 'location' must not be empty")
	}

	// If the location starts with "https://" or "http://", treat it as URL
	if strings.HasPrefix(c.location, "https://") {
		return c.initJWKSFromURL(ctx, c.location)
	} else if strings.HasPrefix(c.location, "http://") {
		c.logger.Warn("Loading JWK from an HTTP endpoint without TLS: this is not recommended on production environments.")
		return c.initJWKSFromURL(ctx, c.location)
	}

	// Check if the location is a valid path to a local file
	stat, err := os.Stat(c.location)
	if err == nil && stat != nil && !stat.IsDir() {
		return c.initJWKSFromFile(ctx, c.location)
	}

	// Treat the location as the actual JWKS
	// First, check if it's base64-encoded (remove trailing padding chars if present first)
	locationJSON, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(c.location, "="))
	if err != nil {
		// Assume it's already JSON, not encoded
		locationJSON = []byte(c.location)
	}

	// Try decoding from JSON
	c.jwks, err = jwk.Parse(locationJSON)
	if err != nil {
		return errors.New("failed to parse property 'location': not a URL, path to local file, or JSON value (optionally base64-encoded)")
	}

	return nil
}

func (c *JWKSCache) initJWKSFromURL(ctx context.Context, url string) error {
	// Create the JWKS cache
	cache := jwk.NewCache(ctx,
		jwk.WithErrSink(httprc.ErrSinkFunc(func(err error) {
			c.logger.Warnf("Error while refreshing JWKS cache: %v", err)
		})),
	)

	// We also need to create a custom HTTP client (if we don't have one already) because otherwise there's no timeout.
	if c.client == nil {
		c.client = &http.Client{
			Timeout: c.requestTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
		}
	}

	// Register the cache
	err := cache.Register(url,
		jwk.WithMinRefreshInterval(c.minRefreshInterval),
		jwk.WithHTTPClient(c.client),
	)
	if err != nil {
		return fmt.Errorf("failed to register JWKS cache: %w", err)
	}

	// Fetch the JWKS right away to start, so we can check it's valid and populate the cache
	refreshCtx, refreshCancel := context.WithTimeout(ctx, c.requestTimeout)
	_, err = cache.Refresh(refreshCtx, url)
	refreshCancel()
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}

	c.jwks = jwk.NewCachedSet(cache, url)
	return nil
}

func (c *JWKSCache) initJWKSFromFile(ctx context.Context, file string) error {
	// Get the path to the folder containing the file
	path := filepath.Dir(file)

	// Start watching for changes in the filesystem
	eventCh := make(chan struct{})
	loaded := make(chan error, 1) // Needs to be buffered to prevent a goroutine leak
	go func() {
		watchErr := fswatcher.Watch(ctx, path, eventCh)
		if watchErr != nil && !errors.Is(watchErr, context.Canceled) {
			// Log errors only
			c.logger.Errorf("Error while watching for changes to the local JWKS file: %v", watchErr)
		}
	}()
	go func() {
		var firstDone bool
		for {
			select {
			case <-eventCh:
				// When there's a change, reload the JWKS file
				if firstDone {
					c.logger.Debug("Reloading JWKS file from disk")
				} else {
					c.logger.Debug("Loading JWKS file from disk")
				}
				err := c.parseJWKSFile(file)
				if !firstDone {
					// The first time, signal that the initialization was complete and pass the error
					loaded <- err
					close(loaded)
					firstDone = true
				} else if err != nil {
					// Log errors only
					c.logger.Errorf("Error reading JWKS from disk: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Trigger a refresh immediately and wait for the first reload
	eventCh <- struct{}{}

	select {
	case err := <-loaded:
		// Error could be nil if everything is fine
		return err
	case <-time.After(5 * time.Second):
		// If we don't get a response in 5s, something bad's going on
		return errors.New("failed to initialize JWKS from file: no file loaded after 5s")
	case <-ctx.Done():
		return fmt.Errorf("failed to initialize JWKS from file: %w", ctx.Err())
	}
}

// Used by initJWKSFromFile to parse a JWKS file every time it's changed
func (c *JWKSCache) parseJWKSFile(file string) error {
	read, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read JWKS file: %v", err)
	}

	jwks, err := jwk.Parse(read)
	if err != nil {
		return fmt.Errorf("failed to parse JWKS file: %v", err)
	}

	c.lock.Lock()
	c.jwks = jwks
	c.lock.Unlock()

	return nil
}
