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
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/stretchr/testify/require"
)

func Test_svidSource(*testing.T) {
	var (
		_ x509svid.Source = new(svidSource)
		_ jwtsvid.Source  = new(svidSource)
	)
}

func TestGetX509SVID(t *testing.T) {
	t.Run("should return error when no X509 SVID available", func(t *testing.T) {
		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh:         make(chan struct{}),
				currentX509SVID: nil,
			},
		}
		close(s.spiffe.readyCh)

		svid, err := s.GetX509SVID()

		require.Nil(t, svid)
		require.ErrorIs(t, err, errNoX509SVIDAvailable)
	})

	t.Run("should return X509 SVID when available", func(t *testing.T) {
		td, err := spiffeid.TrustDomainFromString("example.org")
		require.NoError(t, err)
		id, err := spiffeid.FromSegments(td, "workload")
		require.NoError(t, err)

		mockSVID := &x509svid.SVID{ID: id}
		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh:         make(chan struct{}),
				currentX509SVID: mockSVID,
			},
		}
		close(s.spiffe.readyCh)

		svid, err := s.GetX509SVID()

		require.NoError(t, err)
		require.Equal(t, mockSVID, svid)
	})

	t.Run("does not hold RLock while waiting for readyCh", func(t *testing.T) {
		readyCh := make(chan struct{})
		s := &svidSource{spiffe: &SPIFFE{readyCh: readyCh}}

		go func() { s.GetX509SVID() }() //nolint:errcheck

		time.Sleep(10 * time.Millisecond)

		writeLockFree := s.spiffe.lock.TryLock()
		if writeLockFree {
			s.spiffe.lock.Unlock()
		}

		close(readyCh)

		require.True(t, writeLockFree,
			"GetX509SVID held RLock while waiting for readyCh")
	})
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

		svid, err := s.FetchJWTSVID(t.Context(), jwtsvid.Params{
			Audience: "",
		})

		require.Nil(t, svid)
		require.ErrorIs(t, err, errAudienceRequired)
	})

	t.Run("should return error when no JWT SVID available", func(t *testing.T) {
		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh:            make(chan struct{}),
				lock:               sync.RWMutex{},
				currentBaseJWTSVID: nil,
			},
		}
		close(s.spiffe.readyCh) // Mark as ready

		svid, err := s.FetchJWTSVID(t.Context(), jwtsvid.Params{
			Audience: "test-audience",
		})

		require.Nil(t, svid)
		require.ErrorIs(t, err, errNoJWTSVIDAvailable)
	})

	t.Run("should return error when audience doesn't match", func(t *testing.T) {
		// Create a mock SVID with a specific audience
		mockJWTSVID, err := createMockJWTSVID([]string{"actual-audience"})
		require.NoError(t, err)

		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh:            make(chan struct{}),
				lock:               sync.RWMutex{},
				currentBaseJWTSVID: mockJWTSVID,
			},
		}
		close(s.spiffe.readyCh) // Mark as ready

		svid, err := s.FetchJWTSVID(t.Context(), jwtsvid.Params{
			Audience: "requested-audience",
		})

		require.Nil(t, svid)
		require.Error(t, err)

		// Verify the specific error type and contents
		audienceErr, ok := err.(*audienceMismatchError)
		require.True(t, ok, "Expected audienceMismatchError")
		require.Equal(t, "JWT SVID has different audiences than requested: expected requested-audience, got actual-audience", audienceErr.Error())
	})

	t.Run("PER: should return error when audience doesn't match", func(t *testing.T) {
		// Create a mock SVID with a specific audience
		mockJWTSVID, err := createMockJWTSVID([]string{"actual-audience"})
		require.NoError(t, err)

		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh:            make(chan struct{}),
				lock:               sync.RWMutex{},
				currentBaseJWTSVID: mockJWTSVID,
				currentPerAudJWTSVID: map[string]*jwtsvid.SVID{
					"actual-audience": mockJWTSVID,
				},
			},
		}
		close(s.spiffe.readyCh) // Mark as ready

		svid, err := s.FetchJWTSVID(t.Context(), jwtsvid.Params{
			Audience: "requested-audience",
		})

		require.Nil(t, svid)
		require.Error(t, err)

		// Verify the specific error type and contents
		audienceErr, ok := err.(*audienceMismatchError)
		require.True(t, ok, "Expected audienceMismatchError")
		require.Equal(t, "JWT SVID has different audiences than requested: expected requested-audience, got actual-audience", audienceErr.Error())
	})

	t.Run("should return JWT SVID when audience matches", func(t *testing.T) {
		mockJWTSVID, err := createMockJWTSVID([]string{"test-audience", "extra-audience"})
		require.NoError(t, err)

		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh:            make(chan struct{}),
				lock:               sync.RWMutex{},
				currentBaseJWTSVID: mockJWTSVID,
			},
		}
		close(s.spiffe.readyCh) // Mark as ready

		svid, err := s.FetchJWTSVID(t.Context(), jwtsvid.Params{
			Audience: "test-audience",
		})

		require.NoError(t, err)
		require.Equal(t, mockJWTSVID, svid)
	})

	t.Run("PER: should return JWT SVID when audience matches", func(t *testing.T) {
		mockJWTSVID1, err := createMockJWTSVID([]string{"test-audience", "extra-audience"})
		require.NoError(t, err)
		mockJWTSVID2, err := createMockJWTSVID([]string{"test-audience"})
		require.NoError(t, err)
		mockJWTSVID3, err := createMockJWTSVID([]string{"extra-audience"})
		require.NoError(t, err)

		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh:            make(chan struct{}),
				lock:               sync.RWMutex{},
				currentBaseJWTSVID: mockJWTSVID1,
				currentPerAudJWTSVID: map[string]*jwtsvid.SVID{
					"test-audience":  mockJWTSVID2,
					"extra-audience": mockJWTSVID3,
				},
			},
		}
		close(s.spiffe.readyCh) // Mark as ready

		svid, err := s.FetchJWTSVID(t.Context(), jwtsvid.Params{
			Audience: "test-audience",
		})

		require.NoError(t, err)
		require.Equal(t, mockJWTSVID2, svid)
	})

	t.Run("does not hold RLock while waiting for readyCh", func(t *testing.T) {
		readyCh := make(chan struct{})
		s := &svidSource{spiffe: &SPIFFE{readyCh: readyCh}}

		go func() {
			s.FetchJWTSVID(t.Context(), jwtsvid.Params{Audience: "test-audience"}) //nolint:errcheck
		}()

		time.Sleep(10 * time.Millisecond)

		writeLockFree := s.spiffe.lock.TryLock()
		if writeLockFree {
			s.spiffe.lock.Unlock()
		}

		close(readyCh)

		require.True(t, writeLockFree, "FetchJWTSVID held RLock while waiting for readyCh")
	})

	t.Run("should wait for readyCh before checking SVID", func(t *testing.T) {
		mockJWTSVID, err := createMockJWTSVID([]string{"test-audience"})
		require.NoError(t, err)

		readyCh := make(chan struct{})
		s := &svidSource{
			spiffe: &SPIFFE{
				readyCh:            readyCh,
				lock:               sync.RWMutex{},
				currentBaseJWTSVID: mockJWTSVID,
			},
		}

		// Start goroutine to fetch SVID
		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
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

		// require that fetch is blocked
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
			require.NotNil(t, result.svid)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("FetchJWTSVID should have completed after readyCh was closed")
		}
	})
}

func TestMintJWTSVID(t *testing.T) {
	const testAudience = "https://api.example.com"

	workloadID := spiffeid.RequireFromString("spiffe://example.org/ns/default/app")
	otherID := spiffeid.RequireFromString("spiffe://example.org/ns/default/other")

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	var minted atomic.Int32

	mint := func(sub spiffeid.ID, audiences []string) string {
		tkn, berr := jwt.NewBuilder().
			JwtID(fmt.Sprintf("jti-%d", minted.Add(1))).
			Subject(sub.String()).
			Audience(audiences).
			IssuedAt(time.Now()).
			Expiration(time.Now().Add(5 * time.Minute)).
			Build()
		require.NoError(t, berr)

		signed, serr := jwt.Sign(tkn, jwt.WithKey(jwa.ES256, key))
		require.NoError(t, serr)

		return string(signed)
	}

	newSource := func(fn RequestJWTSVIDFn) *svidSource {
		s := &svidSource{spiffe: &SPIFFE{
			readyCh:          make(chan struct{}),
			currentX509SVID:  &x509svid.SVID{ID: workloadID},
			requestJWTSVIDFn: fn,
		}}
		close(s.spiffe.readyCh)

		return s
	}

	t.Run("mints a new JWT SVID on every call", func(t *testing.T) {
		var gotAudiences [][]string

		s := newSource(func(_ context.Context, audiences []string) (string, error) {
			gotAudiences = append(gotAudiences, audiences)

			return mint(workloadID, audiences), nil
		})

		svid1, err := s.MintJWTSVID(t.Context(), jwtsvid.Params{Audience: testAudience})
		require.NoError(t, err)
		svid2, err := s.MintJWTSVID(t.Context(), jwtsvid.Params{Audience: testAudience, ExtraAudiences: []string{"extra"}, Subject: workloadID})
		require.NoError(t, err)

		require.Equal(t, [][]string{{testAudience}, {testAudience, "extra"}}, gotAudiences)
		require.Equal(t, []string{testAudience}, svid1.Audience)
		require.Equal(t, workloadID, svid1.ID)
		require.NotEqual(t, svid1.Claims["jti"], svid2.Claims["jti"])
	})

	t.Run("rejects requests and tokens that do not match the workload", func(t *testing.T) {
		for name, test := range map[string]struct {
			token           func(audiences []string) string
			params          jwtsvid.Params
			noX509          bool
			subjectMismatch bool
		}{
			"token without the requested audience": {
				token:  func([]string) string { return mint(workloadID, []string{"wrong"}) },
				params: jwtsvid.Params{Audience: testAudience},
			},
			"token without an extra audience": {
				token:  func([]string) string { return mint(workloadID, []string{testAudience}) },
				params: jwtsvid.Params{Audience: testAudience, ExtraAudiences: []string{"extra"}},
			},
			"token for another SPIFFE ID": {
				token:           func(audiences []string) string { return mint(otherID, audiences) },
				params:          jwtsvid.Params{Audience: testAudience},
				subjectMismatch: true,
			},
			"requested subject is not the workload": {
				params:          jwtsvid.Params{Audience: testAudience, Subject: otherID},
				subjectMismatch: true,
			},
			"no X.509 SVID": {
				params: jwtsvid.Params{Audience: testAudience},
				noX509: true,
			},
		} {
			t.Run(name, func(t *testing.T) {
				s := newSource(func(_ context.Context, audiences []string) (string, error) {
					require.NotNil(t, test.token, "the request must be rejected before minting")
					return test.token(audiences), nil
				})
				if test.noX509 {
					s.spiffe.currentX509SVID = nil
				}

				svid, err := s.MintJWTSVID(t.Context(), test.params)
				require.Error(t, err)
				require.Nil(t, svid)

				if test.subjectMismatch {
					var subErr *subjectMismatchError
					require.ErrorAs(t, err, &subErr)
				}
			})
		}
	})

	t.Run("waits for the identity before minting", func(t *testing.T) {
		s := newSource(nil)
		s.spiffe.readyCh = make(chan struct{})

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		_, err := s.MintJWTSVID(ctx, jwtsvid.Params{Audience: testAudience})
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
