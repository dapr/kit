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
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
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

const (
	// renewalDivisor represents the divisor for calculating renewal time.
	// A value of 2 means renewal at 50% of the validity period.
	renewalDivisor = 2

	// rsaKeyBits is the size, in bits, of generated RSA workload keys.
	rsaKeyBits = 2048
)

// KeyAlgorithm selects the algorithm used to generate the workload's
// private key when requesting an SVID.
type KeyAlgorithm int

const (
	// KeyAlgorithmEd25519 generates an Ed25519 private key. This is the
	// default when no algorithm is specified.
	KeyAlgorithmEd25519 KeyAlgorithm = iota

	// KeyAlgorithmRSA generates an RSA private key. Used by workloads whose
	// certificates are consumed by systems that do not accept Ed25519, such
	// as the Kubernetes API server when calling admission webhooks.
	KeyAlgorithmRSA
)

// SVIDResponse represents the response from the SVID request function,
// containing both X.509 certificates and JWT tokens (base and per-audience).
type SVIDResponse struct {
	X509Certificates []*x509.Certificate
	JWT              *string //nolint:gosec
	PerAudienceJWT   map[string]string
}

// Identity contains both X.509 and JWT SVIDs for a workload.
// It may include a base JWT SVID as well as per-audience JWT SVIDs.
type Identity struct {
	X509SVID           *x509svid.SVID
	JWTSVID            *jwtsvid.SVID
	PerAudienceJWTSVID map[string]*jwtsvid.SVID
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

	// KeyAlgorithm selects the algorithm used for the workload's private
	// key. When nil, defaults to Ed25519.
	KeyAlgorithm *KeyAlgorithm
}

// SPIFFE is a readable/writeable store of SPIFFE SVID credentials.
// Used to manage workload SVIDs, and share read-only interfaces to consumers.
type SPIFFE struct {
	currentX509SVID *x509svid.SVID

	currentBaseJWTSVID   *jwtsvid.SVID
	currentPerAudJWTSVID map[string]*jwtsvid.SVID

	requestSVIDFn RequestSVIDFn

	dir          *dir.Dir
	trustAnchors trustanchors.Interface

	keyAlgorithm KeyAlgorithm

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

	keyAlg := KeyAlgorithmEd25519
	if opts.KeyAlgorithm != nil {
		keyAlg = *opts.KeyAlgorithm
	}

	return &SPIFFE{
		requestSVIDFn: opts.RequestSVIDFn,
		dir:           sdir,
		trustAnchors:  opts.TrustAnchors,
		keyAlgorithm:  keyAlg,
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
	s.currentBaseJWTSVID = initialIdentity.JWTSVID
	s.currentPerAudJWTSVID = initialIdentity.PerAudienceJWTSVID
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

func (s *SPIFFE) X509SVIDSource() x509svid.Source {
	return &svidSource{spiffe: s}
}

func (s *SPIFFE) JWTSVIDSource() jwtsvid.Source {
	return &svidSource{spiffe: s}
}

// logIdentityInfo creates a log message with expiry details for both X.509 and JWT SVIDs
func (s *SPIFFE) logIdentityInfo(prefix string, cert *x509.Certificate, jwtSVID *jwtsvid.SVID, renewTime *time.Time) {
	msg := prefix + "; cert expires on: %s"
	args := []any{cert.NotAfter.String()}

	if jwtSVID != nil {
		msg += ", jwt expires on: %s"

		args = append(args, jwtSVID.Expiry.String())
	}

	if renewTime != nil {
		msg += ", renewal at: %s"

		args = append(args, renewTime.String())
	}

	s.log.Infof(msg, args...)
}

// runRotation starts up the manager responsible for renewing the workload identity
func (s *SPIFFE) runRotation(ctx context.Context) {
	defer s.log.Debug("stopping workload identity expiry watcher")

	s.lock.RLock()
	cert := s.currentX509SVID.Certificates[0]
	jwtSVID := s.currentBaseJWTSVID
	s.lock.RUnlock()

	renewTime := calculateRenewalTime(time.Now(), cert, jwtSVID)
	s.logIdentityInfo("Starting workload identity expiry watcher", cert, jwtSVID, renewTime)

	for {
		select {
		case <-s.clock.After(min(time.Minute, renewTime.Sub(s.clock.Now()))):
			if s.clock.Now().Before(*renewTime) {
				continue
			}

			s.logIdentityInfo("Renewing workload identity", cert, jwtSVID, nil)

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
			s.currentBaseJWTSVID = identity.JWTSVID
			s.currentPerAudJWTSVID = identity.PerAudienceJWTSVID
			cert = identity.X509SVID.Certificates[0]
			jwtSVID = identity.JWTSVID
			s.lock.Unlock()

			renewTime = calculateRenewalTime(time.Now(), cert, jwtSVID)
			s.logIdentityInfo("Successfully renewed workload identity", cert, jwtSVID, renewTime)

		case <-ctx.Done():
			return
		}
	}
}

// Returns both X.509 SVID and JWT SVID (if available).
func (s *SPIFFE) fetchIdentity(ctx context.Context) (*Identity, error) {
	key, err := generatePrivateKey(s.keyAlgorithm)
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
	if svidResponse.JWT != nil {
		// we are using ParseInsecure here as the expectation is that the
		// requestSVIDFn will have already parsed and validate the JWT SVID
		// before returning it.
		//
		// we are parsing the token using our SPIFFE ID's trust domain
		// as the audience as we expect the issuer to always include
		// that as an audience since that ensures that the token is
		// valid for us and our trust domain.
		audiences := []string{spiffeID.TrustDomain().Name()}

		jwtSvid, err := jwtsvid.ParseInsecure(*svidResponse.JWT, audiences)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JWT SVID: %w", err)
		}

		identity.JWTSVID = jwtSvid
		s.log.Infof("Successfully received JWT SVID with expiry: %s", jwtSvid.Expiry.String())
	}

	for aud, token := range svidResponse.PerAudienceJWT {
		jwtSvid, err := jwtsvid.ParseInsecure(token, []string{aud})
		if err != nil {
			return nil, fmt.Errorf("failed to parse per-audience JWT SVID for audience %q: %w", aud, err)
		}

		if identity.PerAudienceJWTSVID == nil {
			identity.PerAudienceJWTSVID = make(map[string]*jwtsvid.SVID)
		}

		identity.PerAudienceJWTSVID[aud] = jwtSvid
		s.log.Infof("Successfully received per-audience JWT SVID for audience %s with expiry: %s", aud, jwtSvid.Expiry.String())
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

		if svidResponse.JWT != nil {
			files["jwt_svid.token"] = []byte(*svidResponse.JWT)
		}

		err = s.dir.Write(files)
		if err != nil {
			return nil, err
		}
	}

	return identity, nil
}

// generatePrivateKey returns a freshly generated private key for the
// configured algorithm.
func generatePrivateKey(alg KeyAlgorithm) (crypto.Signer, error) {
	switch alg {
	case KeyAlgorithmRSA:
		return rsa.GenerateKey(rand.Reader, rsaKeyBits)
	case KeyAlgorithmEd25519:
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}

		return key, nil
	default:
		return nil, fmt.Errorf("unsupported key algorithm: %d", alg)
	}
}

// renewalTime is 50% through the certificate validity period.
func renewalTime(notBefore, notAfter time.Time) time.Time {
	return notBefore.Add(notAfter.Sub(notBefore) / renewalDivisor)
}

// calculateRenewalTime returns the earlier renewal time between the X.509 certificate
// and JWT SVID (if available) to ensure timely renewal.
func calculateRenewalTime(now time.Time, cert *x509.Certificate, jwtSVID *jwtsvid.SVID) *time.Time {
	certRenewal := renewalTime(cert.NotBefore, cert.NotAfter)

	if jwtSVID == nil {
		return &certRenewal
	}

	jwtRenewal := now.Add(jwtSVID.Expiry.Sub(now) / renewalDivisor)

	if jwtRenewal.Before(certRenewal) {
		return &jwtRenewal
	}

	return &certRenewal
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
