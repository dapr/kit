/*
Copyright 2024 The Dapr Authors
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

package trustanchors

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"k8s.io/utils/clock"

	"github.com/dapr/kit/concurrency"
	"github.com/dapr/kit/crypto/pem"
	"github.com/dapr/kit/fswatcher"
	"github.com/dapr/kit/logger"
)

type OptionsFile struct {
	Log  logger.Logger
	Path string
}

// file is a TrustAnchors implementation that uses a file as the source of trust
// anchors. The trust anchors will be updated when the file changes.
type file struct {
	log     logger.Logger
	path    string
	bundle  *x509bundle.Bundle
	rootPEM []byte

	// fswatcherInterval is the interval at which the trust anchors file changes
	// are batched. Used for testing only, and 500ms otherwise.
	fsWatcherInterval time.Duration

	// initFileWatchInterval is the interval at which the trust anchors file is
	// checked for the first time. Used for testing only, and 1 second otherwise.
	initFileWatchInterval time.Duration

	// subs is a list of channels to notify when the trust anchors are updated.
	subs []chan<- struct{}

	lock    sync.RWMutex
	clock   clock.Clock
	running atomic.Bool
	readyCh chan struct{}
	closeCh chan struct{}
	caEvent chan struct{}
}

func FromFile(opts OptionsFile) Interface {
	return &file{
		fsWatcherInterval:     time.Millisecond * 500,
		initFileWatchInterval: time.Second,

		log:     opts.Log,
		path:    opts.Path,
		clock:   clock.RealClock{},
		readyCh: make(chan struct{}),
		closeCh: make(chan struct{}),
		caEvent: make(chan struct{}),
	}
}

func (f *file) Run(ctx context.Context) error {
	if !f.running.CompareAndSwap(false, true) {
		return errors.New("trust anchors is already running")
	}

	defer close(f.closeCh)

	for {
		_, err := os.Stat(f.path)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		// Trust anchors file not be provided yet, wait.
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed to find trust anchors file '%s': %w", f.path, ctx.Err())
		case <-f.clock.After(f.initFileWatchInterval):
			f.log.Warnf("Trust anchors file '%s' not found, waiting...", f.path)
		}
	}

	f.log.Infof("Trust anchors file '%s' found", f.path)

	if err := f.updateAnchors(ctx); err != nil {
		return err
	}

	fs, err := fswatcher.New(fswatcher.Options{
		Targets:  []string{filepath.Dir(f.path)},
		Interval: &f.fsWatcherInterval,
	})
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	close(f.readyCh)

	f.log.Infof("Watching trust anchors file '%s' for changes", f.path)
	return concurrency.NewRunnerManager(
		func(ctx context.Context) error {
			return fs.Run(ctx, f.caEvent)
		},
		func(ctx context.Context) error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case <-f.caEvent:
					f.log.Info("Trust anchors file changed, reloading trust anchors")

					if err = f.updateAnchors(ctx); err != nil {
						return fmt.Errorf("failed to read trust anchors file '%s': %v", f.path, err)
					}
				}
			}
		},
	).Run(ctx)
}

func (f *file) CurrentTrustAnchors(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.closeCh:
		return nil, errors.New("trust anchors is closed")
	case <-f.readyCh:
	}

	f.lock.RLock()
	defer f.lock.RUnlock()
	rootPEM := make([]byte, len(f.rootPEM))
	copy(rootPEM, f.rootPEM)
	return rootPEM, nil
}

func (f *file) updateAnchors(ctx context.Context) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	rootPEMs, err := os.ReadFile(f.path)
	if err != nil {
		return fmt.Errorf("failed to read trust anchors file '%s': %w", f.path, err)
	}

	trustAnchorCerts, err := pem.DecodePEMCertificates(rootPEMs)
	if err != nil {
		return fmt.Errorf("failed to decode trust anchors: %w", err)
	}

	f.rootPEM = rootPEMs
	f.bundle = x509bundle.FromX509Authorities(spiffeid.TrustDomain{}, trustAnchorCerts)

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(len(f.subs))
	for _, ch := range f.subs {
		go func(chi chan<- struct{}) {
			defer wg.Done()
			select {
			case chi <- struct{}{}:
			case <-ctx.Done():
			}
		}(ch)
	}

	return nil
}

func (f *file) GetX509BundleForTrustDomain(_ spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	select {
	case <-f.closeCh:
		return nil, errors.New("trust anchors is closed")
	case <-f.readyCh:
	}

	f.lock.RLock()
	defer f.lock.RUnlock()
	bundle := f.bundle
	return bundle, nil
}

func (f *file) Watch(ctx context.Context, ch chan<- []byte) {
	f.lock.Lock()
	sub := make(chan struct{}, 5)
	f.subs = append(f.subs, sub)
	f.lock.Unlock()

	for {
		select {
		case <-ctx.Done():
			return
		case <-f.closeCh:
			return
		case <-sub:
			f.lock.RLock()
			rootPEM := make([]byte, len(f.rootPEM))
			copy(rootPEM, f.rootPEM)
			f.lock.RUnlock()

			select {
			case ch <- rootPEM:
			case <-ctx.Done():
			case <-f.closeCh:
			}
		}
	}
}
