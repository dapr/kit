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
	"crypto/x509"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clocktesting "k8s.io/utils/clock/testing"

	"github.com/dapr/kit/crypto/test"
	"github.com/dapr/kit/logger"
)

func Test_renewalTime(t *testing.T) {
	now := time.Now()
	assert.Equal(t, now, renewalTime(now, now))

	in1Min := now.Add(time.Minute)
	in30 := now.Add(time.Second * 30)
	assert.Equal(t, in30, renewalTime(now, in1Min))
}

func Test_Run(t *testing.T) {
	t.Run("should return error multiple Runs are called", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{
			LeafID: spiffeid.RequireFromString("spiffe://example.com/foo/bar"),
		})
		ctx, cancel := context.WithCancel(context.Background())
		s := New(Options{
			Log: logger.NewLogger("test"),
			RequestSVIDFn: func(context.Context, []byte) (*SVIDResponse, error) {
				return &SVIDResponse{
					X509Certificates: []*x509.Certificate{pki.LeafCert},
				}, nil
			},
		})

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

		cancel()
		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "First Run should have returned and returned no error ")
		}
	})

	t.Run("should return error if initial fetch errors", func(t *testing.T) {
		s := New(Options{
			Log: logger.NewLogger("test"),
			RequestSVIDFn: func(context.Context, []byte) (*SVIDResponse, error) {
				return nil, errors.New("this is an error")
			},
		})

		require.Error(t, s.Run(context.Background()))
	})

	t.Run("should renew certificate when it has expired", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{
			LeafID: spiffeid.RequireFromString("spiffe://example.com/foo/bar"),
		})

		var fetches atomic.Int32
		s := New(Options{
			Log: logger.NewLogger("test"),
			RequestSVIDFn: func(context.Context, []byte) (*SVIDResponse, error) {
				fetches.Add(1)
				return &SVIDResponse{
					X509Certificates: []*x509.Certificate{pki.LeafCert},
				}, nil
			},
		})
		now := time.Now()
		clock := clocktesting.NewFakeClock(now)
		s.clock = clock

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error)
		go func() {
			errCh <- s.Run(ctx)
		}()

		select {
		case <-s.readyCh:
			assert.Fail(t, "readyCh should not be closed")
		default:
		}

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		assert.Equal(t, int32(1), fetches.Load())

		clock.Step(pki.LeafCert.NotAfter.Sub(now) / 2)
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			assert.Equal(c, int32(2), fetches.Load())
		}, time.Second, time.Millisecond)

		cancel()
		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "First Run should have returned and returned no error ")
		}
	})

	t.Run("if renewal failed, should try again in 10 seconds", func(t *testing.T) {
		pki := test.GenPKI(t, test.PKIOptions{
			LeafID: spiffeid.RequireFromString("spiffe://example.com/foo/bar"),
		})

		respCert := []*x509.Certificate{pki.LeafCert}
		var respErr error

		var fetches atomic.Int32
		s := New(Options{
			Log: logger.NewLogger("test"),
			RequestSVIDFn: func(context.Context, []byte) (*SVIDResponse, error) {
				fetches.Add(1)
				return &SVIDResponse{
					X509Certificates: respCert,
				}, respErr
			},
		})
		now := time.Now()
		clock := clocktesting.NewFakeClock(now)
		s.clock = clock

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error)
		go func() {
			errCh <- s.Run(ctx)
		}()

		select {
		case <-s.readyCh:
			assert.Fail(t, "readyCh should not be closed")
		default:
		}

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		assert.Equal(t, int32(1), fetches.Load())

		respCert = nil
		respErr = errors.New("this is an error")
		clock.Step(pki.LeafCert.NotAfter.Sub(now) / 2)
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			assert.Equal(c, int32(2), fetches.Load())
		}, time.Second, time.Millisecond)

		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(time.Second * 5)
		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		assert.Equal(t, int32(2), fetches.Load())

		clock.Step(time.Second * 5)
		assert.Eventually(t, clock.HasWaiters, time.Second, time.Millisecond)
		clock.Step(1)
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			assert.Equal(c, int32(3), fetches.Load())
		}, time.Second, time.Millisecond)

		cancel()
		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(time.Second):
			assert.Fail(t, "First Run should have returned and returned no error ")
		}
	})
}
