/*
Copyright 2025 The Dapr Authors
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

package multi

import (
	"context"
	"errors"

	"github.com/spiffe/go-spiffe/v2/bundle/jwtbundle"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"

	"github.com/dapr/kit/concurrency"
	"github.com/dapr/kit/crypto/spiffe/trustanchors"
)

var (
	ErrNotImplemented      = errors.New("not implemented")
	ErrTrustDomainNotFound = errors.New("trust domain not found")
)

type Options struct {
	TrustAnchors map[spiffeid.TrustDomain]trustanchors.Interface
}

// multi is a TrustAnchors implementation which uses multiple trust anchors
// which are indexed by trust domain.
type multi struct {
	trustAnchors map[spiffeid.TrustDomain]trustanchors.Interface
}

func From(opts Options) trustanchors.Interface {
	return &multi{
		trustAnchors: opts.TrustAnchors,
	}
}

func (m *multi) Run(ctx context.Context) error {
	r := concurrency.NewRunnerManager()
	for _, ta := range m.trustAnchors {
		if err := r.Add(ta.Run); err != nil {
			return err
		}
	}

	return r.Run(ctx)
}

func (m *multi) CurrentTrustAnchors(context.Context) ([]byte, error) {
	return nil, ErrNotImplemented
}

func (m *multi) GetX509BundleForTrustDomain(td spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	for tad, ta := range m.trustAnchors {
		if td.Compare(tad) == 0 {
			return ta.GetX509BundleForTrustDomain(td)
		}
	}

	return nil, ErrTrustDomainNotFound
}

func (m *multi) GetJWTBundleForTrustDomain(td spiffeid.TrustDomain) (*jwtbundle.Bundle, error) {
	for tad, ta := range m.trustAnchors {
		if td.Compare(tad) == 0 {
			return ta.GetJWTBundleForTrustDomain(td)
		}
	}

	return nil, ErrTrustDomainNotFound
}

func (m *multi) Watch(context.Context, chan<- []byte) {
	return
}
