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
	"errors"

	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

// svidSource is an implementation of the Go spiffe x509svid Source interface.
type svidSource struct {
	spiffe *SPIFFE
}

// GetX509SVID returns the current X.509 certificate identity as a SPIFFE SVID.
// Implements the go-spiffe x509 source interface.
func (s *svidSource) GetX509SVID() (*x509svid.SVID, error) {
	s.spiffe.lock.RLock()
	defer s.spiffe.lock.RUnlock()

	<-s.spiffe.readyCh

	svid := s.spiffe.currentSVID
	if svid == nil {
		return nil, errors.New("no SVID available")
	}

	return svid, nil
}
