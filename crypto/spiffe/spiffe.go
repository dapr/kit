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

	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"k8s.io/utils/clock"

	"github.com/dapr/kit/concurrency/dir"
	"github.com/dapr/kit/crypto/pem"
	"github.com/dapr/kit/crypto/spiffe/trustanchors"
	"github.com/dapr/kit/logger"
)

type (
	RequestSVIDFn func(context.Context, []byte) ([]*x509.Certificate, error)
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

// SPIFFE is a readable/writeable store of a SPIFFE X.509 SVID.
// Used to manage a workload SVID, and share read-only interfaces to consumers.
type SPIFFE struct {
	currentSVID   *x509svid.SVID
	requestSVIDFn RequestSVIDFn

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
	s.log.Info("Fetching initial identity certificate")
	initialCert, err := s.fetchIdentityCertificate(ctx)
	if err != nil {
		close(s.readyCh)
		s.lock.Unlock()
		return fmt.Errorf("failed to retrieve the initial identity certificate: %w", err)
	}

	s.currentSVID = initialCert
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
// certificate. Receives the initial certificate to calculate the next rotation
// time.
func (s *SPIFFE) runRotation(ctx context.Context) {
	defer s.log.Debug("stopping workload cert expiry watcher")
	s.lock.RLock()
	cert := s.currentSVID.Certificates[0]
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
			s.log.Infof("Renewing workload cert; current cert expires on: %s", cert.NotAfter.String())
			svid, err := s.fetchIdentityCertificate(ctx)
			if err != nil {
				s.log.Errorf("Error renewing identity certificate, trying again in 10 seconds: %s", err)
				select {
				case <-s.clock.After(10 * time.Second):
					continue
				case <-ctx.Done():
					return
				}
			}
			s.lock.Lock()
			s.currentSVID = svid
			cert = svid.Certificates[0]
			s.lock.Unlock()
			renewTime = renewalTime(cert.NotBefore, cert.NotAfter)
			s.log.Infof("Successfully renewed workload cert; new cert expires on: %s", cert.NotAfter.String())

		case <-ctx.Done():
			return
		}
	}
}

// fetchIdentityCertificate fetches a new SVID using the configured requester.
func (s *SPIFFE) fetchIdentityCertificate(ctx context.Context) (*x509svid.SVID, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, new(x509.CertificateRequest), key)
	if err != nil {
		return nil, fmt.Errorf("failed to create sidecar csr: %w", err)
	}

	workloadcert, err := s.requestSVIDFn(ctx, csrDER)
	if err != nil {
		return nil, err
	}

	if len(workloadcert) == 0 {
		return nil, errors.New("no certificates received from sentry")
	}

	spiffeID, err := x509svid.IDFromCert(workloadcert[0])
	if err != nil {
		return nil, fmt.Errorf("error parsing spiffe id from newly signed certificate: %w", err)
	}

	if s.dir != nil {
		pkPEM, err := pem.EncodePrivateKey(key)
		if err != nil {
			return nil, err
		}

		certPEM, err := pem.EncodeX509Chain(workloadcert)
		if err != nil {
			return nil, err
		}

		td, err := s.trustAnchors.CurrentTrustAnchors(ctx)
		if err != nil {
			return nil, err
		}

		if err := s.dir.Write(map[string][]byte{
			"key.pem":  pkPEM,
			"cert.pem": certPEM,
			"ca.pem":   td,
		}); err != nil {
			return nil, err
		}
	}

	return &x509svid.SVID{
		ID:           spiffeID,
		Certificates: workloadcert,
		PrivateKey:   key,
	}, nil
}

func (s *SPIFFE) SVIDSource() x509svid.Source {
	return &svidSource{spiffe: s}
}

// renewalTime is 50% through the certificate validity period.
func renewalTime(notBefore, notAfter time.Time) time.Time {
	return notBefore.Add(notAfter.Sub(notBefore) / 2)
}
