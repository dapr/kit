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
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapr/kit/crypto/test"
)

func TestFromStatic(t *testing.T) {
	t.Run("empty root should return error", func(t *testing.T) {
		_, err := FromStatic(OptionsStatic{})
		require.Error(t, err)
	})

	t.Run("garbage data should return error", func(t *testing.T) {
		_, err := FromStatic(OptionsStatic{Anchors: []byte("garbage data")})
		require.Error(t, err)
	})

	t.Run("just garbage data should return error", func(t *testing.T) {
		_, err := FromStatic(OptionsStatic{Anchors: []byte("garbage data")})
		require.Error(t, err)
	})

	t.Run("garbage data in root should return error", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		root := pki.RootCertPEM[10:]
		_, err := FromStatic(OptionsStatic{Anchors: root})
		require.Error(t, err)
	})

	t.Run("single root should be correctly parsed", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		ta, err := FromStatic(OptionsStatic{Anchors: pki.RootCertPEM})
		require.NoError(t, err)
		taPEM, err := ta.CurrentTrustAnchors(context.Background())
		require.NoError(t, err)
		assert.Equal(t, pki.RootCertPEM, taPEM)
	})

	t.Run("garbage data outside of root should be ignored", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		//nolint:gocritic
		root := append(pki.RootCertPEM, []byte("garbage data")...)
		ta, err := FromStatic(OptionsStatic{Anchors: root})
		require.NoError(t, err)
		taPEM, err := ta.CurrentTrustAnchors(context.Background())
		require.NoError(t, err)
		assert.Equal(t, root, taPEM)
	})

	t.Run("multiple roots should be correctly parsed", func(t *testing.T) {
		pki1, pki2 := test.GenPKI(t, test.PKIOptions{}), test.GenPKI(t, test.PKIOptions{})
		//nolint:gocritic
		roots := append(pki1.RootCertPEM, pki2.RootCertPEM...)
		ta, err := FromStatic(OptionsStatic{Anchors: roots})
		require.NoError(t, err)
		taPEM, err := ta.CurrentTrustAnchors(context.Background())
		require.NoError(t, err)
		assert.Equal(t, roots, taPEM)
	})
}

func TestStatic_GetX509BundleForTrustDomain(t *testing.T) {
	t.Run("Should return full PEM regardless given trust domain", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		//nolint:gocritic
		root := append(pki.RootCertPEM, []byte("garbage data")...)
		ta, err := FromStatic(OptionsStatic{Anchors: root})
		require.NoError(t, err)
		s, ok := ta.(*static)
		require.True(t, ok)

		trustDomain1, err := spiffeid.TrustDomainFromString("example.com")
		require.NoError(t, err)
		bundle, err := s.GetX509BundleForTrustDomain(trustDomain1)
		require.NoError(t, err)
		assert.Equal(t, s.x509Bundle, bundle)
		b1, err := bundle.Marshal()
		require.NoError(t, err)
		assert.Equal(t, pki.RootCertPEM, b1)

		trustDomain2, err := spiffeid.TrustDomainFromString("another-example.org")
		require.NoError(t, err)
		bundle, err = s.GetX509BundleForTrustDomain(trustDomain2)
		require.NoError(t, err)
		assert.Equal(t, s.x509Bundle, bundle)
		b2, err := bundle.Marshal()
		require.NoError(t, err)
		assert.Equal(t, pki.RootCertPEM, b2)
	})
}

func TestStatic_Run(t *testing.T) {
	t.Run("Run multiple times should return error", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		ta, err := FromStatic(OptionsStatic{Anchors: pki.RootCertPEM})
		require.NoError(t, err)
		s, ok := ta.(*static)
		require.True(t, ok)

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error)
		go func() {
			errCh <- s.Run(ctx)
		}()
		go func() {
			errCh <- s.Run(ctx)
		}()

		select {
		case err := <-errCh:
			require.Error(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "Expected error")
		}

		select {
		case <-s.closeCh:
			assert.Fail(t, "closeCh should not be closed")
		default:
		}

		cancel()

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "First Run should have returned and returned no error ")
		}
	})
}

func TestStatic_Watch(t *testing.T) {
	t.Run("should return when context is cancelled", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		ta, err := FromStatic(OptionsStatic{Anchors: pki.RootCertPEM})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		doneCh := make(chan struct{})

		go func() {
			ta.Watch(ctx, nil)
			close(doneCh)
		}()

		cancel()

		select {
		case <-doneCh:
		case <-time.After(time.Second):
			assert.Fail(t, "Expected doneCh to be closed")
		}
	})

	t.Run("should return when cancel is closed via closed Run", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		ta, err := FromStatic(OptionsStatic{Anchors: pki.RootCertPEM})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		doneCh := make(chan struct{})
		errCh := make(chan error)

		go func() {
			errCh <- ta.Run(ctx)
		}()

		go func() {
			ta.Watch(context.Background(), nil)
			close(doneCh)
		}()

		cancel()

		select {
		case <-doneCh:
		case <-time.After(time.Second):
			assert.Fail(t, "Expected doneCh to be closed")
		}

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "Expected Run to return no error")
		}
	})
}
