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

package spiffe

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"k8s.io/utils/clock"

	"github.com/dapr/kit/concurrency/dir"
	"github.com/dapr/kit/crypto/pem"
	"github.com/dapr/kit/crypto/spiffe/trustanchors"
	"github.com/dapr/kit/logger"
)

// SVIDResponse represents the response from the SVID request function,
// containing both X.509 certificates and a JWT token.
type SVIDResponse struct {
	X509Certificates []*x509.Certificate
	JWT              string
	Audiences        []string
}

// Identity contains both X.509 and JWT SVIDs for a workload.
type Identity struct {
	X509SVID *x509svid.SVID
	JWTSVID  *jwtsvid.SVID
}

type (
	// RequestSVIDFn is the function type that requests SVIDs from a SPIFFE server,
	// returning both X.509 certificates and a JWT token.
	RequestSVIDFn func(context.Context, []byte) (*SVIDResponse, error)
)

type Options struct {
	Log           logger.Logger
	RequestSVIDFn RequestSVIDFn

	// WriteIdentityToFile is used to write the identity private key and
	// certificate chain to file. The certificate chain and private key will be
	// written to the `tls.cert` and `tls.key` files respectively in the given
	// directory.
	WriteIdentityToFile *string

	TrustAnchors trustanchors.Interface
}

// SPIFFE is a readable/writeable store of SPIFFE SVID credentials.
// Used to manage workload SVIDs, and share read-only interfaces to consumers.
type SPIFFE struct {
	currentX509SVID *x509svid.SVID
	currentJWTSVID  *jwtsvid.SVID
	requestSVIDFn   RequestSVIDFn

	dir          *dir.Dir
	trustAnchors trustanchors.Interface

	log     logger.Logger
	lock    sync.RWMutex
	clock   clock.Clock
	running atomic.Bool
	readyCh chan struct{}
}

func New(opts Options) *SPIFFE {
	var sdir *dir.Dir
	if opts.WriteIdentityToFile != nil {
		sdir = dir.New(dir.Options{
			Log:    opts.Log,
			Target: *opts.WriteIdentityToFile,
		})
	}

	return &SPIFFE{
		requestSVIDFn: opts.RequestSVIDFn,
		dir:           sdir,
		trustAnchors:  opts.TrustAnchors,
		log:           opts.Log,
		clock:         clock.RealClock{},
		readyCh:       make(chan struct{}),
	}
}

func (s *SPIFFE) Run(ctx context.Context) error {
	if !s.running.CompareAndSwap(false, true) {
		return errors.New("already running")
	}

	s.lock.Lock()
	s.log.Info("Fetching initial identity")
	initialIdentity, err := s.fetchIdentity(ctx)
	if err != nil {
		close(s.readyCh)
		s.lock.Unlock()
		return fmt.Errorf("failed to retrieve the initial identity: %w", err)
	}

	s.currentX509SVID = initialIdentity.X509SVID
	s.currentJWTSVID = initialIdentity.JWTSVID
	close(s.readyCh)
	s.lock.Unlock()

	s.log.Infof("Security is initialized successfully")
	s.runRotation(ctx)

	return nil
}

// Ready blocks until SPIFFE is ready or the context is done which will return
// the context error.
func (s *SPIFFE) Ready(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.readyCh:
		return nil
	}
}

// runRotation starts up the manager responsible for renewing the workload
// identity. Receives the initial identity to calculate the next rotation
// time.
func (s *SPIFFE) runRotation(ctx context.Context) {
	defer s.log.Debug("stopping workload cert expiry watcher")
	s.lock.RLock()
	cert := s.currentX509SVID.Certificates[0]
	s.lock.RUnlock()
	renewTime := renewalTime(cert.NotBefore, cert.NotAfter)
	s.log.Infof("Starting workload cert expiry watcher; current cert expires on: %s, renewing at %s",
		cert.NotAfter.String(), renewTime.String())

	for {
		select {
		case <-s.clock.After(min(time.Minute, renewTime.Sub(s.clock.Now()))):
			if s.clock.Now().Before(renewTime) {
				continue
			}
			s.log.Infof("Renewing workload identity; current cert expires on: %s", cert.NotAfter.String())
			identity, err := s.fetchIdentity(ctx)
			if err != nil {
				s.log.Errorf("Error renewing identity, trying again in 10 seconds: %s", err)
				select {
				case <-s.clock.After(10 * time.Second):
					continue
				case <-ctx.Done():
					return
				}
			}
			s.lock.Lock()
			s.currentX509SVID = identity.X509SVID
			s.currentJWTSVID = identity.JWTSVID
			cert = identity.X509SVID.Certificates[0]
			s.lock.Unlock()
			renewTime = renewalTime(cert.NotBefore, cert.NotAfter)

			msg := "Successfully renewed workload identity; new cert expires on: %s"
			args := []interface{}{cert.NotAfter.String()}
			if identity.JWTSVID != nil {
				msg += ", new jwt expires on: %s"
				args = append(args, identity.JWTSVID.Expiry.String())
			}
			s.log.Infof(msg, args...)

		case <-ctx.Done():
			return
		}
	}
}

// fetchIdentity fetches a new identity using the configured requester.
// Returns both X.509 SVID and JWT SVID (if available).
func (s *SPIFFE) fetchIdentity(ctx context.Context) (*Identity, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, new(x509.CertificateRequest), key)
	if err != nil {
		return nil, fmt.Errorf("failed to create sidecar csr: %w", err)
	}

	svidResponse, err := s.requestSVIDFn(ctx, csrDER)
	if err != nil {
		return nil, err
	}

	if len(svidResponse.X509Certificates) == 0 {
		return nil, errors.New("no certificates received from sentry")
	}

	spiffeID, err := x509svid.IDFromCert(svidResponse.X509Certificates[0])
	if err != nil {
		return nil, fmt.Errorf("error parsing spiffe id from newly signed certificate: %w", err)
	}

	identity := &Identity{
		X509SVID: &x509svid.SVID{
			ID:           spiffeID,
			Certificates: svidResponse.X509Certificates,
			PrivateKey:   key,
		},
	}

	// If we have a JWT token, parse it and include it in the identity
	if svidResponse.JWT != "" {
		jwtSvid, err := jwtsvid.ParseInsecure(svidResponse.JWT, svidResponse.Audiences)
		if err != nil {
			s.log.Warnf("Failed to parse JWT SVID: %v", err)
		} else {
			identity.JWTSVID = jwtSvid
			s.log.Infof("Successfully received JWT SVID with expiry: %s", jwtSvid.Expiry.String())
		}
	}

	if s.dir != nil {
		pkPEM, err := pem.EncodePrivateKey(key)
		if err != nil {
			return nil, err
		}

		certPEM, err := pem.EncodeX509Chain(svidResponse.X509Certificates)
		if err != nil {
			return nil, err
		}

		td, err := s.trustAnchors.CurrentTrustAnchors(ctx)
		if err != nil {
			return nil, err
		}

		files := map[string][]byte{
			"key.pem":  pkPEM,
			"cert.pem": certPEM,
			"ca.pem":   td,
		}

		if svidResponse.JWT != "" {
			files["token.jwt"] = []byte(svidResponse.JWT)
		}

		if err := s.dir.Write(files); err != nil {
			return nil, err
		}
	}

	return identity, nil
}

func (s *SPIFFE) X509SVIDSource() x509svid.Source {
	return &svidSource{spiffe: s}
}

func (s *SPIFFE) JWTSVIDSource() jwtsvid.Source {
	return &svidSource{spiffe: s}
}

// renewalTime is 50% through the certificate validity period.
func renewalTime(notBefore, notAfter time.Time) time.Time {
	return notBefore.Add(notAfter.Sub(notBefore) / 2)
}

// audiencesMatch checks if the SVID audiences contain all the requested audiences
func audiencesMatch(svidAudiences []string, requestedAudiences []string) bool {
	if len(requestedAudiences) == 0 {
		return true
	}

	// Create a map for faster lookup
	audienceMap := make(map[string]struct{}, len(svidAudiences))
	for _, audience := range svidAudiences {
		audienceMap[audience] = struct{}{}
	}

	// Check if all requested audiences are in the SVID
	for _, requested := range requestedAudiences {
		if _, ok := audienceMap[requested]; !ok {
			return false
		}
	}

	return true
}
