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
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapr/kit/crypto/spiffe/trustanchors/fake"
)

type staticSVIDSource struct {
	svid *x509svid.SVID
	err  error
}

func (s *staticSVIDSource) GetX509SVID() (*x509svid.SVID, error) {
	return s.svid, s.err
}

func generateEd25519Cert(t *testing.T) ([]byte, *x509.Certificate, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ed25519"},
		NotBefore:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:     time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		URIs:         []*url.URL{{Scheme: "spiffe", Host: "example.org", Path: "/ns/default/app-a"}},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pub, priv)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)
	return certDER, cert, priv
}

func generateECDSACert(t *testing.T) ([]byte, *x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ecdsa"},
		NotBefore:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:     time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		URIs:         []*url.URL{{Scheme: "spiffe", Host: "example.org", Path: "/ns/default/app-b"}},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)
	return certDER, cert, priv
}

func generateRSACert(t *testing.T) ([]byte, *x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-rsa"},
		NotBefore:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:     time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		URIs:         []*url.URL{{Scheme: "spiffe", Host: "example.org", Path: "/ns/default/app-c"}},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)
	return certDER, cert, priv
}

func generateCA(t *testing.T) ([]byte, *x509.Certificate, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, template, template, pub, priv)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	return caDER, ca, priv
}

func generateLeafSignedByCA(t *testing.T, ca *x509.Certificate, caKey ed25519.PrivateKey) ([]byte, *x509.Certificate, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test leaf"},
		NotBefore:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:     time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		URIs:         []*url.URL{{Scheme: "spiffe", Host: "example.org", Path: "/ns/default/app-a"}},
	}

	leafDER, err := x509.CreateCertificate(rand.Reader, template, ca, pub, caKey)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(leafDER)
	require.NoError(t, err)
	return leafDER, leaf, priv
}

// testDigest returns a SHA256 digest of the input, which is the format used
// by the production SignatureInput function and required by RSA signing.
func testDigest(input string) []byte {
	h := sha256.Sum256([]byte(input))
	return h[:]
}

func newSVIDSource(cert *x509.Certificate, key crypto.Signer) *staticSVIDSource {
	id, _ := x509svid.IDFromCert(cert)
	return &staticSVIDSource{svid: &x509svid.SVID{
		ID:           id,
		Certificates: []*x509.Certificate{cert},
		PrivateKey:   key,
	}}
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("nil svidSource for verify-only", func(t *testing.T) {
		t.Parallel()
		s := New(nil, fake.New())
		require.NotNil(t, s)
	})

	t.Run("nil trustAnchors for sign-only", func(t *testing.T) {
		t.Parallel()
		certDER, cert, priv := generateEd25519Cert(t)
		s := New(newSVIDSource(cert, priv), nil)
		require.NotNil(t, s)

		err := s.VerifyCertChainOfTrust(certDER, time.Now())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no trust anchors configured")
	})

	t.Run("both present", func(t *testing.T) {
		t.Parallel()
		_, cert, priv := generateEd25519Cert(t)
		s := New(newSVIDSource(cert, priv), fake.New(cert))
		require.NotNil(t, s)
	})
}

func TestSign_NilSVIDSource(t *testing.T) {
	t.Parallel()
	s := New(nil, fake.New())
	_, _, err := s.Sign(testDigest("hello"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no SVID source configured")
}

func TestSign_SVIDSourceError(t *testing.T) {
	t.Parallel()
	source := &staticSVIDSource{err: errors.New("svid unavailable")}
	s := New(source, nil)
	_, _, err := s.Sign(testDigest("hello"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "svid unavailable")
}

func TestSign_NoCertificates(t *testing.T) {
	t.Parallel()
	source := &staticSVIDSource{svid: &x509svid.SVID{
		Certificates: nil,
		PrivateKey:   ed25519.NewKeyFromSeed(make([]byte, 32)),
	}}
	s := New(source, nil)
	_, _, err := s.Sign(testDigest("hello"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no certificates")
}

func TestSignAndVerify_Ed25519(t *testing.T) {
	t.Parallel()
	_, cert, priv := generateEd25519Cert(t)
	s := New(newSVIDSource(cert, priv), nil)

	digest := testDigest("test digest")
	sig, certChain, err := s.Sign(digest)
	require.NoError(t, err)
	require.NotEmpty(t, sig)
	require.NotEmpty(t, certChain)

	err = s.VerifySignature(digest, sig, certChain)
	require.NoError(t, err)
}

func TestSignAndVerify_ECDSA(t *testing.T) {
	t.Parallel()
	_, cert, priv := generateECDSACert(t)
	s := New(newSVIDSource(cert, priv), nil)

	digest := testDigest("test digest")
	sig, certChain, err := s.Sign(digest)
	require.NoError(t, err)

	err = s.VerifySignature(digest, sig, certChain)
	require.NoError(t, err)
}

func TestSignAndVerify_RSA(t *testing.T) {
	t.Parallel()
	_, cert, priv := generateRSACert(t)
	s := New(newSVIDSource(cert, priv), nil)

	digest := testDigest("test digest")
	sig, certChain, err := s.Sign(digest)
	require.NoError(t, err)

	err = s.VerifySignature(digest, sig, certChain)
	require.NoError(t, err)
}

func TestVerify_TamperedDigest(t *testing.T) {
	t.Parallel()
	_, cert, priv := generateEd25519Cert(t)
	s := New(newSVIDSource(cert, priv), nil)

	sig, certChain, err := s.Sign(testDigest("original"))
	require.NoError(t, err)

	err = s.VerifySignature(testDigest("tampered"), sig, certChain)
	require.Error(t, err)
}

func TestVerify_TamperedSignature(t *testing.T) {
	t.Parallel()
	_, cert, priv := generateEd25519Cert(t)
	s := New(newSVIDSource(cert, priv), nil)

	digest := testDigest("test")
	sig, certChain, err := s.Sign(digest)
	require.NoError(t, err)

	// Flip a byte in the signature.
	sig[0] ^= 0xff

	err = s.VerifySignature(digest, sig, certChain)
	require.Error(t, err)
}

func TestVerify_InvalidCertChain(t *testing.T) {
	t.Parallel()
	s := New(nil, fake.New())
	err := s.VerifySignature(testDigest("digest"), []byte("sig"), []byte("not-a-cert"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestVerify_EmptyCertChain(t *testing.T) {
	t.Parallel()
	s := New(nil, fake.New())
	// Valid DER but empty is not possible; just use nil.
	err := s.VerifySignature(testDigest("digest"), []byte("sig"), nil)
	require.Error(t, err)
}

func TestSign_ReturnsCertChainDER(t *testing.T) {
	t.Parallel()
	certDER, cert, priv := generateEd25519Cert(t)
	s := New(newSVIDSource(cert, priv), nil)

	_, certChain, err := s.Sign(testDigest("digest"))
	require.NoError(t, err)
	assert.Equal(t, certDER, certChain)
}

func TestVerifyCertChainOfTrust_SelfSigned(t *testing.T) {
	t.Parallel()
	certDER, cert, _ := generateEd25519Cert(t)
	ta := fake.New(cert)
	s := New(nil, ta)

	err := s.VerifyCertChainOfTrust(certDER, time.Now())
	require.NoError(t, err)
}

func TestVerifyCertChainOfTrust_CAChain(t *testing.T) {
	t.Parallel()
	caDER, ca, caKey := generateCA(t)
	leafDER, _, _ := generateLeafSignedByCA(t, ca, caKey)

	chainDER := slices.Concat(leafDER, caDER)
	ta := fake.New(ca)
	s := New(nil, ta)

	err := s.VerifyCertChainOfTrust(chainDER, time.Now())
	require.NoError(t, err)
}

func TestVerifyCertChainOfTrust_IntermediateChain(t *testing.T) {
	t.Parallel()
	_, rootCA, rootKey := generateCA(t)

	// Create intermediate CA signed by root.
	intermPub, intermPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	intermTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "Intermediate CA"},
		NotBefore:             time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	intermDER, err := x509.CreateCertificate(rand.Reader, intermTemplate, rootCA, intermPub, rootKey)
	require.NoError(t, err)
	intermCA, err := x509.ParseCertificate(intermDER)
	require.NoError(t, err)

	// Create leaf signed by intermediate.
	leafDER, _, _ := generateLeafSignedByCA(t, intermCA, intermPriv)

	chainDER := slices.Concat(leafDER, intermDER)
	ta := fake.New(rootCA)
	s := New(nil, ta)

	err = s.VerifyCertChainOfTrust(chainDER, time.Now())
	require.NoError(t, err)
}

func TestVerifyCertChainOfTrust_WrongTrustAnchor(t *testing.T) {
	t.Parallel()
	caDER, ca, caKey := generateCA(t)
	leafDER, _, _ := generateLeafSignedByCA(t, ca, caKey)
	chainDER := slices.Concat(leafDER, caDER)

	// Different CA as trust anchor.
	_, wrongCA, _ := generateCA(t)
	ta := fake.New(wrongCA)
	s := New(nil, ta)

	err := s.VerifyCertChainOfTrust(chainDER, time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chain-of-trust verification failed")
}

func TestVerifyCertChainOfTrust_EmptyChain(t *testing.T) {
	t.Parallel()
	ta := fake.New()
	s := New(nil, ta)

	err := s.VerifyCertChainOfTrust(nil, time.Now())
	require.Error(t, err)
}

func TestVerifyCertChainOfTrust_InvalidDER(t *testing.T) {
	t.Parallel()
	ta := fake.New()
	s := New(nil, ta)

	err := s.VerifyCertChainOfTrust([]byte("not-a-cert"), time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestSignAndVerify_RoundTrip_AllKeyTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		certGen func(t *testing.T) (*x509.Certificate, crypto.Signer)
	}{
		{
			name: "ed25519",
			certGen: func(t *testing.T) (*x509.Certificate, crypto.Signer) {
				_, cert, priv := generateEd25519Cert(t)
				return cert, priv
			},
		},
		{
			name: "ecdsa",
			certGen: func(t *testing.T) (*x509.Certificate, crypto.Signer) {
				_, cert, priv := generateECDSACert(t)
				return cert, priv
			},
		},
		{
			name: "rsa",
			certGen: func(t *testing.T) (*x509.Certificate, crypto.Signer) {
				_, cert, priv := generateRSACert(t)
				return cert, priv
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cert, priv := tc.certGen(t)
			s := New(newSVIDSource(cert, priv), fake.New(cert))

			digest := testDigest("round trip test for " + tc.name)
			sig, certChain, err := s.Sign(digest)
			require.NoError(t, err)

			err = s.VerifySignature(digest, sig, certChain)
			require.NoError(t, err)

			err = s.VerifyCertChainOfTrust(certChain, time.Now())
			require.NoError(t, err)
		})
	}
}
