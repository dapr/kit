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

	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

var (
	errNoX509SVIDAvailable = errors.New("no X509 SVID available")
	errNoJWTSVIDAvailable  = errors.New("no JWT SVID available")
	errAudienceRequired    = errors.New("audience is required")
)

// svidSource is an implementation of both go-spiffe x509svid.Source and jwtsvid.Source interfaces.
type svidSource struct {
	spiffe *SPIFFE
}

// GetX509SVID returns the current X.509 certificate identity as a SPIFFE SVID.
// Implements the go-spiffe x509svid.Source interface.
func (s *svidSource) GetX509SVID() (*x509svid.SVID, error) {
	s.spiffe.lock.RLock()
	defer s.spiffe.lock.RUnlock()

	<-s.spiffe.readyCh

	svid := s.spiffe.currentX509SVID
	if svid == nil {
		return nil, errNoX509SVIDAvailable
	}

	return svid, nil
}

// audienceMismatchError is an error that contains information about mismatched audiences
type audienceMismatchError struct {
	Expected []string
	Actual   []string
}

func (e *audienceMismatchError) Error() string {
	return fmt.Sprintf("JWT SVID has different audiences than requested: expected %s, got %s",
		strings.Join(e.Expected, ", "), strings.Join(e.Actual, ", "))
}

// FetchJWTSVID returns the current JWT SVID.
// Implements the go-spiffe jwtsvid.Source interface.
func (s *svidSource) FetchJWTSVID(_ context.Context, params jwtsvid.Params) (*jwtsvid.SVID, error) {
	s.spiffe.lock.RLock()
	defer s.spiffe.lock.RUnlock()

	if params.Audience == "" {
		return nil, errAudienceRequired
	}

	<-s.spiffe.readyCh

	svid := s.spiffe.currentJWTSVID
	if svid == nil {
		return nil, errNoJWTSVIDAvailable
	}

	// verify that the audience being requested is the same as the audience in the SVID
	// WARN: we do not check extra audiences here.
	if !audiencesMatch(svid.Audience, []string{params.Audience}) {
		return nil, &audienceMismatchError{
			Expected: []string{params.Audience},
			Actual:   svid.Audience,
		}
	}

	return svid, nil
}
