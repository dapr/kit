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
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"

	"github.com/dapr/kit/crypto/pem"
)

// static is a TrustAcnhors implementation that uses a static list of trust
// anchors.
type static struct {
	bundle  *x509bundle.Bundle
	anchors []byte
	running atomic.Bool
	closeCh chan struct{}
}

func FromStatic(anchors []byte) (Interface, error) {
	trustAnchorCerts, err := pem.DecodePEMCertificates(anchors)
	if err != nil {
		return nil, fmt.Errorf("failed to decode trust anchors: %w", err)
	}

	return &static{
		anchors: anchors,
		bundle:  x509bundle.FromX509Authorities(spiffeid.TrustDomain{}, trustAnchorCerts),
		closeCh: make(chan struct{}),
	}, nil
}

func (s *static) CurrentTrustAnchors(context.Context) ([]byte, error) {
	bundle := make([]byte, len(s.anchors))
	copy(bundle, s.anchors)
	return bundle, nil
}

func (s *static) Run(ctx context.Context) error {
	if !s.running.CompareAndSwap(false, true) {
		return errors.New("trust anchors source is already running")
	}
	<-ctx.Done()
	close(s.closeCh)
	return nil
}

func (s *static) GetX509BundleForTrustDomain(spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return s.bundle, nil
}

func (s *static) Watch(ctx context.Context, _ chan<- []byte) {
	select {
	case <-ctx.Done():
	case <-s.closeCh:
	}
}
