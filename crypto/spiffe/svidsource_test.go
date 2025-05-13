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
	"sync"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_svidSource(*testing.T) {
	var _ x509svid.Source = new(svidSource)
	var _ jwtsvid.Source = new(svidSource)
}

// createMockJWTSVID creates a mock JWT SVID for testing
func createMockJWTSVID(audiences []string) (*jwtsvid.SVID, error) {
	td, err := spiffeid.TrustDomainFromString("example.org")
	if err != nil {
		return nil, err
	}

	id, err := spiffeid.FromSegments(td, "workload")
	if err != nil {
		return nil, err
	}

	svid := &jwtsvid.SVID{
		ID:       id,
		Audience: audiences,
		Expiry:   time.Now().Add(time.Hour),
	}

	return svid, nil
}

func TestFetchJWTSVID(t *testing.T) {
	t.Run("should return error when audience is empty", func(t *testing.T) {
		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh: make(chan struct{}),
				lock:    sync.RWMutex{},
			},
		}
		close(s.spiffe.readyCh) // Mark as ready

		svid, err := s.FetchJWTSVID(context.Background(), jwtsvid.Params{
			Audience: "",
		})

		assert.Nil(t, svid)
		require.ErrorIs(t, err, errAudienceRequired)
	})

	t.Run("should return error when no JWT SVID available", func(t *testing.T) {
		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh:        make(chan struct{}),
				lock:           sync.RWMutex{},
				currentJWTSVID: nil,
			},
		}
		close(s.spiffe.readyCh) // Mark as ready

		svid, err := s.FetchJWTSVID(context.Background(), jwtsvid.Params{
			Audience: "test-audience",
		})

		assert.Nil(t, svid)
		require.ErrorIs(t, err, errNoJWTSVIDAvailable)
	})

	t.Run("should return error when audience doesn't match", func(t *testing.T) {
		// Create a mock SVID with a specific audience
		mockJWTSVID, err := createMockJWTSVID([]string{"actual-audience"})
		require.NoError(t, err)

		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh:        make(chan struct{}),
				lock:           sync.RWMutex{},
				currentJWTSVID: mockJWTSVID,
			},
		}
		close(s.spiffe.readyCh) // Mark as ready

		svid, err := s.FetchJWTSVID(context.Background(), jwtsvid.Params{
			Audience: "requested-audience",
		})

		assert.Nil(t, svid)
		require.Error(t, err)

		// Verify the specific error type and contents
		audienceErr, ok := err.(*audienceMismatchError)
		require.True(t, ok, "Expected audienceMismatchError")
		assert.Equal(t, "JWT SVID has different audiences than requested: expected requested-audience, got actual-audience", audienceErr.Error())
	})

	t.Run("should return JWT SVID when audience matches", func(t *testing.T) {
		mockJWTSVID, err := createMockJWTSVID([]string{"test-audience", "extra-audience"})
		require.NoError(t, err)

		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh:        make(chan struct{}),
				lock:           sync.RWMutex{},
				currentJWTSVID: mockJWTSVID,
			},
		}
		close(s.spiffe.readyCh) // Mark as ready

		svid, err := s.FetchJWTSVID(context.Background(), jwtsvid.Params{
			Audience: "test-audience",
		})

		assert.NoError(t, err)
		assert.Equal(t, mockJWTSVID, svid)
	})

	t.Run("should wait for readyCh before checking SVID", func(t *testing.T) {
		mockJWTSVID, err := createMockJWTSVID([]string{"test-audience"})
		require.NoError(t, err)

		readyCh := make(chan struct{})
		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh:        readyCh,
				lock:           sync.RWMutex{},
				currentJWTSVID: mockJWTSVID,
			},
		}

		// Start goroutine to fetch SVID
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		resultCh := make(chan struct {
			svid *jwtsvid.SVID
			err  error
		})

		go func() {
			svid, err := s.FetchJWTSVID(ctx, jwtsvid.Params{
				Audience: "test-audience",
			})
			resultCh <- struct {
				svid *jwtsvid.SVID
				err  error
			}{svid, err}
		}()

		// Assert that fetch is blocked
		select {
		case <-resultCh:
			t.Fatal("FetchJWTSVID should be blocked until readyCh is closed")
		case <-time.After(100 * time.Millisecond):
			// Expected behavior - fetch is blocked
		}

		// Close readyCh to unblock fetch
		close(readyCh)

		// Now fetch should complete
		select {
		case result := <-resultCh:
			require.NoError(t, result.err)
			assert.NotNil(t, result.svid)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("FetchJWTSVID should have completed after readyCh was closed")
		}
	})
}
