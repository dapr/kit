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

package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapr/kit/crypto/test"
	"github.com/dapr/kit/logger"
)

func TestFile_Run(t *testing.T) {
	t.Run("if Run multiple times, expect error", func(t *testing.T) {
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond

		ctx, cancel := context.WithCancel(t.Context())
		errCh := make(chan error)
		go func() {
			errCh <- f.Run(ctx)
		}()
		go func() {
			errCh <- f.Run(ctx)
		}()

		select {
		case err := <-errCh:
			require.Error(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "Expected error")
		}

		select {
		case <-f.closeCh:
			assert.Fail(t, "closeCh should not be closed")
		default:
		}

		cancel()

		select {
		case err := <-errCh:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(time.Second):
			assert.Fail(t, "First Run should have returned and returned no error ")
		}
	})

	t.Run("if file is not found and context cancelled, should return ctx.Err", func(t *testing.T) {
		tmp := filepath.Join(t.TempDir(), "ca.crt")

		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond

		ctx, cancel := context.WithCancel(t.Context())
		errCh := make(chan error)
		go func() {
			errCh <- f.Run(ctx)
		}()

		cancel()

		select {
		case err := <-errCh:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(time.Second):
			assert.Fail(t, "First Run should have returned and returned no error ")
		}
	})

	t.Run("if file found but is empty, should return error", func(t *testing.T) {
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(tmp, nil, 0o600))

		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond

		errCh := make(chan error)
		go func() {
			errCh <- f.Run(t.Context())
		}()

		select {
		case err := <-errCh:
			require.Error(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "expected error")
		}
	})

	t.Run("if file found but is only garbage data, expect	error", func(t *testing.T) {
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(tmp, []byte("garbage data"), 0o600))

		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond

		errCh := make(chan error)
		go func() {
			errCh <- f.Run(t.Context())
		}()

		select {
		case err := <-errCh:
			require.Error(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "expected error")
		}
	})

	t.Run("if file found but is only garbage data in root, expect	error", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		root := pki.RootCertPEM[10:]
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(tmp, root, 0o600))

		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond

		errCh := make(chan error)
		go func() {
			errCh <- f.Run(t.Context())
		}()

		select {
		case err := <-errCh:
			require.Error(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "expected error")
		}
	})

	t.Run("single root should be correctly parsed from file", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(tmp, pki.RootCertPEM, 0o600))

		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond

		errCh := make(chan error)
		go func() {
			errCh <- f.Run(t.Context())
		}()

		select {
		case <-f.readyCh:
		case <-time.After(time.Second):
			assert.Fail(t, "expected to be ready in time")
		}

		b, err := f.CurrentTrustAnchors(t.Context())
		require.NoError(t, err)
		assert.Equal(t, pki.RootCertPEM, b)
	})

	t.Run("garbage data outside of root should be ignored", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		//nolint:gocritic
		root := append(pki.RootCertPEM, []byte("garbage data")...)
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(tmp, root, 0o600))

		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond

		errCh := make(chan error)
		go func() {
			errCh <- f.Run(t.Context())
		}()

		select {
		case <-f.readyCh:
		case <-time.After(time.Second):
			assert.Fail(t, "expected to be ready in time")
		}

		b, err := f.CurrentTrustAnchors(t.Context())
		require.NoError(t, err)
		assert.Equal(t, root, b)
	})

	t.Run("multiple roots should be parsed", func(t *testing.T) {
		pki1, pki2 := test.GenPKI(t, test.PKIOptions{}), test.GenPKI(t, test.PKIOptions{})
		//nolint:gocritic
		roots := append(pki1.RootCertPEM, pki2.RootCertPEM...)
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(tmp, roots, 0o600))

		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond

		errCh := make(chan error)
		go func() {
			errCh <- f.Run(t.Context())
		}()

		select {
		case <-f.readyCh:
		case <-time.After(time.Second):
			assert.Fail(t, "expected to be ready in time")
		}

		b, err := f.CurrentTrustAnchors(t.Context())
		require.NoError(t, err)
		assert.Equal(t, roots, b)
	})

	t.Run("writing a bad root PEM file should make Run return error", func(t *testing.T) {
		pki1, pki2 := test.GenPKI(t, test.PKIOptions{}), test.GenPKI(t, test.PKIOptions{})
		//nolint:gocritic
		roots := append(pki1.RootCertPEM, pki2.RootCertPEM...)
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(tmp, roots, 0o600))

		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond
		f.fsWatcherInterval = time.Millisecond

		errCh := make(chan error)
		go func() {
			errCh <- f.Run(t.Context())
		}()

		select {
		case <-f.readyCh:
		case <-time.After(time.Second):
			assert.Fail(t, "expected to be ready in time")
		}

		require.NoError(t, os.WriteFile(tmp, []byte("garbage data"), 0o600))

		select {
		case err := <-errCh:
			require.Error(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "expected error to be returned from Run")
		}
	})
}

func TestFile_GetX509BundleForTrustDomain(t *testing.T) {
	t.Run("Should return full PEM regardless given trust domain", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		//nolint:gocritic
		root := append(pki.RootCertPEM, []byte("garbage data")...)
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(tmp, root, 0o600))
		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)

		errCh := make(chan error)
		ctx, cancel := context.WithCancel(t.Context())
		go func() {
			errCh <- ta.Run(ctx)
		}()
		t.Cleanup(func() {
			cancel()
			select {
			case err := <-errCh:
				require.NoError(t, err)
			case <-time.After(time.Second):
				assert.Fail(t, "expected Run to return")
			}
		})

		trustDomain1, err := spiffeid.TrustDomainFromString("example.com")
		require.NoError(t, err)
		bundle, err := f.GetX509BundleForTrustDomain(trustDomain1)
		require.NoError(t, err)
		assert.Equal(t, f.x509Bundle, bundle)
		b1, err := bundle.Marshal()
		require.NoError(t, err)
		assert.Equal(t, pki.RootCertPEM, b1)

		trustDomain2, err := spiffeid.TrustDomainFromString("another-example.org")
		require.NoError(t, err)
		bundle, err = f.GetX509BundleForTrustDomain(trustDomain2)
		require.NoError(t, err)
		assert.Equal(t, f.x509Bundle, bundle)
		b2, err := bundle.Marshal()
		require.NoError(t, err)
		assert.Equal(t, pki.RootCertPEM, b2)
	})
}

func TestFile_Watch(t *testing.T) {
	t.Run("should return when Run context has been cancelled", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(tmp, pki.RootCertPEM, 0o600))

		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond

		errCh := make(chan error)
		ctx, cancel := context.WithCancel(t.Context())
		go func() {
			errCh <- f.Run(ctx)
		}()

		watchDone := make(chan struct{})
		go func() {
			ta.Watch(t.Context(), make(chan []byte))
			close(watchDone)
		}()

		cancel()

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "expected error to be returned from Run")
		}

		select {
		case <-watchDone:
		case <-time.After(time.Second):
			assert.Fail(t, "expected Watch to have returned")
		}
	})

	t.Run("should return when given context has been cancelled", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{})
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(tmp, pki.RootCertPEM, 0o600))

		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond

		errCh := make(chan error)
		ctx1, cancel1 := context.WithCancel(t.Context())
		go func() {
			errCh <- f.Run(ctx1)
		}()

		watchDone := make(chan struct{})
		ctx2, cancel2 := context.WithCancel(t.Context())
		go func() {
			ta.Watch(ctx2, make(chan []byte))
			close(watchDone)
		}()

		cancel2()

		select {
		case <-watchDone:
		case <-time.After(time.Second):
			assert.Fail(t, "expected Watch to have returned")
		}

		cancel1()

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "expected error to be returned from Run")
		}
	})

	t.Run("should update Watch subscribers when root PEM has been changed", func(t *testing.T) {
		pki1 := test.GenPKI(t, test.PKIOptions{})
		pki2 := test.GenPKI(t, test.PKIOptions{})
		pki3 := test.GenPKI(t, test.PKIOptions{})
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(tmp, pki1.RootCertPEM, 0o600))

		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond
		f.fsWatcherInterval = time.Millisecond

		errCh := make(chan error)
		ctx, cancel := context.WithCancel(t.Context())
		go func() {
			errCh <- f.Run(ctx)
		}()

		select {
		case <-f.readyCh:
		case <-time.After(time.Second):
			assert.Fail(t, "expected to be ready in time")
		}

		watchDone1, watchDone2 := make(chan struct{}), make(chan struct{})
		tCh1, tCh2 := make(chan []byte), make(chan []byte)
		go func() {
			ta.Watch(t.Context(), tCh1)
			close(watchDone1)
		}()
		go func() {
			ta.Watch(t.Context(), tCh2)
			close(watchDone2)
		}()

		//nolint:gocritic
		roots := append(pki1.RootCertPEM, pki2.RootCertPEM...)
		require.NoError(t, os.WriteFile(tmp, roots, 0o600))

		for _, ch := range []chan []byte{tCh1, tCh2} {
			select {
			case b := <-ch:
				assert.Equal(t, string(roots), string(b))
			case <-time.After(time.Second):
				assert.Fail(t, "failed to get subscribed file watch in time")
			}
		}

		//nolint:gocritic
		roots = append(pki1.RootCertPEM, append(pki2.RootCertPEM, pki3.RootCertPEM...)...)
		require.NoError(t, os.WriteFile(tmp, roots, 0o600))

		for _, ch := range []chan []byte{tCh1, tCh2} {
			select {
			case b := <-ch:
				assert.Equal(t, string(roots), string(b))
			case <-time.After(time.Second):
				assert.Fail(t, "failed to get subscribed file watch in time")
			}
		}

		cancel()

		for _, ch := range []chan struct{}{watchDone1, watchDone2} {
			select {
			case <-ch:
			case <-time.After(time.Second):
				assert.Fail(t, "expected Watch to have returned")
			}
		}

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "expected error to be returned from Run")
		}
	})
}

func TestFile_CurrentTrustAnchors(t *testing.T) {
	t.Run("returns trust anchors as they change", func(t *testing.T) {
		pki1, pki2, pki3 := test.GenPKI(t, test.PKIOptions{}), test.GenPKI(t, test.PKIOptions{}), test.GenPKI(t, test.PKIOptions{})
		tmp := filepath.Join(t.TempDir(), "ca.crt")
		require.NoError(t, os.WriteFile(tmp, pki1.RootCertPEM, 0o600))

		ta := From(Options{
			Log:    logger.NewLogger("test"),
			CAPath: tmp,
		})
		f, ok := ta.(*file)
		require.True(t, ok)
		f.initFileWatchInterval = time.Millisecond
		f.fsWatcherInterval = time.Millisecond

		ctx, cancel := context.WithCancel(t.Context())
		errCh := make(chan error)
		go func() {
			errCh <- f.Run(ctx)
		}()

		//nolint:gocritic
		roots := append(pki1.RootCertPEM, pki2.RootCertPEM...)
		require.NoError(t, os.WriteFile(tmp, roots, 0o600))
		time.Sleep(time.Millisecond * 10) // adding a small delay to ensure the file watcher has time to pick up the change
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			pem, err := ta.CurrentTrustAnchors(t.Context())
			require.NoError(t, err)
			assert.Equal(c, roots, pem)
		}, time.Second, time.Millisecond)

		//nolint:gocritic
		roots = append(pki1.RootCertPEM, append(pki2.RootCertPEM, pki3.RootCertPEM...)...)
		require.NoError(t, os.WriteFile(tmp, roots, 0o600))
		time.Sleep(time.Millisecond * 10) // adding a small delay to ensure the file watcher has time to pick up the change
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			pem, err := ta.CurrentTrustAnchors(t.Context())
			require.NoError(t, err)
			assert.Equal(c, roots, pem)
		}, time.Second, time.Millisecond)

		cancel()

		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "expected error to be returned from Run")
		}
	})
}
