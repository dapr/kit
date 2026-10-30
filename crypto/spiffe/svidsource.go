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
	"errors"
	"fmt"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

var (
	errNoX509SVIDAvailable = errors.New("no X509 SVID available")
	errNoJWTSVIDAvailable  = errors.New("no JWT SVID available")
	errAudienceRequired    = errors.New("JWT audience is required")
)

// svidSource implements the go-spiffe x509svid.Source and jwtsvid.Source
// interfaces, and JWTSVIDMinter.
type svidSource struct {
	spiffe *SPIFFE
}

// GetX509SVID returns the current X.509 certificate identity as a SPIFFE SVID.
// Implements the go-spiffe x509svid.Source interface.
func (s *svidSource) GetX509SVID() (*x509svid.SVID, error) {
	<-s.spiffe.readyCh

	s.spiffe.lock.RLock()
	defer s.spiffe.lock.RUnlock()

	svid := s.spiffe.currentX509SVID
	if svid == nil {
		return nil, errNoX509SVIDAvailable
	}

	return svid, nil
}

// audienceMismatchError is an error that contains information about mismatched audiences
type audienceMismatchError struct {
	expected []string
	actual   []string
}

func (e *audienceMismatchError) Error() string {
	return fmt.Sprintf("JWT SVID has different audiences than requested: expected %s, got %s",
		strings.Join(e.expected, ", "), strings.Join(e.actual, ", "))
}

// subjectMismatchError is returned when a JWT SVID would be for a SPIFFE ID
// other than the workload's own.
type subjectMismatchError struct {
	expected spiffeid.ID
	actual   spiffeid.ID
}

func (e *subjectMismatchError) Error() string {
	return fmt.Sprintf("JWT SVID subject %s is not the workload's SPIFFE ID %s", e.actual, e.expected)
}

// FetchJWTSVID returns the current JWT SVID.
// Implements the go-spiffe jwtsvid.Source interface.
func (s *svidSource) FetchJWTSVID(ctx context.Context, params jwtsvid.Params) (*jwtsvid.SVID, error) {
	if params.Audience == "" {
		return nil, errAudienceRequired
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.spiffe.readyCh:
	}

	s.spiffe.lock.RLock()
	defer s.spiffe.lock.RUnlock()

	svid, ok := s.spiffe.currentPerAudJWTSVID[params.Audience]
	if !ok {
		svid = s.spiffe.currentBaseJWTSVID
	}

	if svid == nil {
		return nil, errNoJWTSVIDAvailable
	}

	// verify that the audience being requested is the same as the audience in the SVID
	// WARN: we do not check extra audiences here.
	if !audiencesMatch(svid.Audience, []string{params.Audience}) {
		return nil, &audienceMismatchError{
			expected: []string{params.Audience},
			actual:   svid.Audience,
		}
	}

	return svid, nil
}

// MintJWTSVID mints a new JWT SVID for the requested audiences from the SPIFFE
// server. Implements JWTSVIDMinter.
func (s *svidSource) MintJWTSVID(ctx context.Context, params jwtsvid.Params) (*jwtsvid.SVID, error) {
	if params.Audience == "" {
		return nil, errAudienceRequired
	}

	// The server authenticates the request with the current X.509 SVID.
	err := s.spiffe.Ready(ctx)
	if err != nil {
		return nil, err
	}

	s.spiffe.lock.RLock()
	x509SVID := s.spiffe.currentX509SVID
	s.spiffe.lock.RUnlock()

	if x509SVID == nil {
		return nil, errNoX509SVIDAvailable
	}

	if !params.Subject.IsZero() && params.Subject != x509SVID.ID {
		return nil, &subjectMismatchError{expected: x509SVID.ID, actual: params.Subject}
	}

	audiences := append([]string{params.Audience}, params.ExtraAudiences...)

	token, err := s.spiffe.requestJWTSVIDFn(ctx, audiences)
	if err != nil {
		return nil, err
	}

	// The token's signature is not verified: RequestJWTSVIDFn receives it from
	// the issuer over an authenticated channel.
	svid, err := jwtsvid.ParseInsecure(token, []string{params.Audience})
	if err != nil {
		return nil, err
	}

	if svid.ID != x509SVID.ID {
		return nil, &subjectMismatchError{expected: x509SVID.ID, actual: svid.ID}
	}

	if !audiencesMatch(svid.Audience, audiences) {
		return nil, &audienceMismatchError{expected: audiences, actual: svid.Audience}
	}

	return svid, nil
}
