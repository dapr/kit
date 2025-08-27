package pem

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeDecodePEMChain(t *testing.T) {
	chain, err := DecodePEMCertificatesChain([]byte(selfSignedRootCert))
	require.NoError(t, err)
	require.NotEmpty(t, chain)

	chainPEM, err := EncodeX509Chain(chain)
	require.NoError(t, err)
	require.NotEmpty(t, chainPEM)
	require.Equal(t, string(selfSignedRootCert), string(chainPEM))
}
