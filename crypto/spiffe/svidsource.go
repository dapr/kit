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

	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
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
		return nil, errors.New("no X509 SVID available")
	}

	return svid, nil
}

// FetchJWTSVID returns the current JWT SVID.
// Implements the go-spiffe jwtsvid.Source interface.
func (s *svidSource) FetchJWTSVID(ctx context.Context, params jwtsvid.Params) (*jwtsvid.SVID, error) {
	s.spiffe.lock.RLock()
	defer s.spiffe.lock.RUnlock()

	<-s.spiffe.readyCh

	svid := s.spiffe.currentJWTSVID
	if svid == nil {
		return nil, errors.New("no JWT SVID available")
	}

	if !audiencesMatch(svid.Audience, []string{params.Audience}) {
		return nil, errors.New("JWT SVID has different audiences than requested")
	}

	return svid, nil
}
