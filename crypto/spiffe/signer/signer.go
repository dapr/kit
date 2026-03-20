/*
Copyright 2026 The Dapr Authors
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

package signer

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/spiffe/go-spiffe/v2/svid/x509svid"

	"github.com/dapr/kit/crypto/spiffe/trustanchors"
)

// Signer provides cryptographic signing and verification using the workload's
// X.509 identity and trust bundles. Callers use it for raw digest signing and
// certificate chain verification without needing direct access to SVID sources
// or trust anchors.
type Signer struct {
	svidSource   x509svid.Source
	trustAnchors trustanchors.Interface
}

// New creates a Signer from an SVID source and trust anchors.
// The svidSource may be nil for verify-only usage (Sign will return an error).
func New(svidSource x509svid.Source, trustAnchors trustanchors.Interface) *Signer {
	return &Signer{
		svidSource:   svidSource,
		trustAnchors: trustAnchors,
	}
}

// Sign signs the given digest using the current SVID's private key.
// Returns the signature bytes and the DER-encoded certificate chain (leaf +
// intermediates concatenated).
func (s *Signer) Sign(digest []byte) ([]byte, []byte, error) {
	if s.svidSource == nil {
		return nil, nil, errors.New("signing not available: no SVID source configured")
	}
	svid, err := s.svidSource.GetX509SVID()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get X.509 SVID: %w", err)
	}

	if len(svid.Certificates) == 0 {
		return nil, nil, errors.New("SVID has no certificates")
	}

	var certChainDER []byte
	for _, cert := range svid.Certificates {
		certChainDER = append(certChainDER, cert.Raw...)
	}

	sig, err := signWithKey(svid.PrivateKey, digest)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign: %w", err)
	}

	return sig, certChainDER, nil
}

// Verify verifies a cryptographic signature against the given digest using the
// public key from the provided DER-encoded certificate chain.
func (s *Signer) Verify(digest, sig, certChainDER []byte) error {
	leaf, err := parseLeafCert(certChainDER)
	if err != nil {
		return err
	}
	return verifyWithKey(leaf.PublicKey, digest, sig)
}

// VerifyCertChainOfTrust verifies that the given DER-encoded certificate chain
// is trusted by the current trust anchors. The trust domain is extracted from
// the leaf certificate's SPIFFE ID (URI SAN).
func (s *Signer) VerifyCertChainOfTrust(certChainDER []byte) error {
	if s.trustAnchors == nil {
		return errors.New("chain-of-trust verification not available: no trust anchors configured")
	}

	certs, err := x509.ParseCertificates(certChainDER)
	if err != nil {
		return fmt.Errorf("failed to parse certificate chain: %w", err)
	}
	if len(certs) == 0 {
		return errors.New("certificate chain is empty")
	}

	leaf := certs[0]

	spiffeID, err := x509svid.IDFromCert(leaf)
	if err != nil {
		return fmt.Errorf("failed to extract SPIFFE ID from certificate: %w", err)
	}

	bundle, err := s.trustAnchors.GetX509BundleForTrustDomain(spiffeID.TrustDomain())
	if err != nil {
		return fmt.Errorf("failed to get trust bundle for trust domain %q: %w", spiffeID.TrustDomain(), err)
	}

	authorities := bundle.X509Authorities()
	if len(authorities) == 0 {
		return fmt.Errorf("trust bundle for trust domain %q has no X.509 authorities", spiffeID.TrustDomain())
	}

	roots := x509.NewCertPool()
	for _, anchor := range authorities {
		roots.AddCert(anchor)
	}

	intermediates := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}

	_, err = leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		// Use the leaf's NotAfter minus one minute as the verification time.
		// This avoids failures from expired short-lived SVIDs and from
		// backdated NotBefore (sentry backdates SVIDs for clock-skew
		// tolerance, which can place NotBefore before the CA's NotBefore).
		CurrentTime: leaf.NotAfter.Add(-time.Minute),
	})
	if err != nil {
		return fmt.Errorf("certificate chain-of-trust verification failed: %w", err)
	}

	return nil
}

// signWithKey signs the given digest with the private key.
func signWithKey(key crypto.Signer, digest []byte) ([]byte, error) {
	switch k := key.(type) {
	case ed25519.PrivateKey:
		return ed25519.Sign(k, digest), nil
	case *ecdsa.PrivateKey:
		r, s, err := ecdsa.Sign(rand.Reader, k, digest)
		if err != nil {
			return nil, err
		}
		byteLen := (k.Curve.Params().BitSize + 7) / 8
		sig := make([]byte, 2*byteLen)
		rBytes := r.Bytes()
		sBytes := s.Bytes()
		copy(sig[byteLen-len(rBytes):byteLen], rBytes)
		copy(sig[2*byteLen-len(sBytes):], sBytes)
		return sig, nil
	case *rsa.PrivateKey:
		return rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, digest)
	default:
		return nil, fmt.Errorf("unsupported key type: %T", key)
	}
}

// verifyWithKey verifies a signature against the given digest and public key.
func verifyWithKey(pubKey crypto.PublicKey, digest, sig []byte) error {
	switch k := pubKey.(type) {
	case ed25519.PublicKey:
		if !ed25519.Verify(k, digest, sig) {
			return errors.New("ed25519 signature verification failed")
		}
		return nil
	case *ecdsa.PublicKey:
		byteLen := (k.Curve.Params().BitSize + 7) / 8
		if len(sig) != 2*byteLen {
			return fmt.Errorf("invalid ECDSA signature length: got %d, want %d", len(sig), 2*byteLen)
		}
		r := new(big.Int).SetBytes(sig[:byteLen])
		s := new(big.Int).SetBytes(sig[byteLen:])
		if !ecdsa.Verify(k, digest, r, s) {
			return errors.New("ecdsa signature verification failed")
		}
		return nil
	case *rsa.PublicKey:
		return rsa.VerifyPKCS1v15(k, crypto.SHA256, digest, sig)
	default:
		return fmt.Errorf("unsupported public key type: %T", pubKey)
	}
}

// parseLeafCert parses a DER-encoded certificate chain and returns the leaf.
func parseLeafCert(chainDER []byte) (*x509.Certificate, error) {
	certs, err := x509.ParseCertificates(chainDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate chain: %w", err)
	}
	if len(certs) == 0 {
		return nil, errors.New("certificate chain is empty")
	}
	return certs[0], nil
}
