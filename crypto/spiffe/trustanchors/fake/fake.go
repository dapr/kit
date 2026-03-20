/*
Copyright 2026 The Dapr Authors
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

package fake

import (
	"context"
	"crypto/x509"

	"github.com/spiffe/go-spiffe/v2/bundle/jwtbundle"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"

	"github.com/dapr/kit/crypto/spiffe/trustanchors"
)

type Fake struct {
	trustanchors.Interface
	bundle *x509bundle.Bundle
}

func New(authorities ...*x509.Certificate) *Fake {
	td := spiffeid.TrustDomain{}
	bundle := x509bundle.New(td)
	for _, a := range authorities {
		bundle.AddX509Authority(a)
	}
	return &Fake{bundle: bundle}
}

func (f *Fake) GetX509BundleForTrustDomain(spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return f.bundle, nil
}

func (f *Fake) GetJWTBundleForTrustDomain(spiffeid.TrustDomain) (*jwtbundle.Bundle, error) {
	return nil, nil
}

func (f *Fake) CurrentTrustAnchors(context.Context) ([]byte, error) { return nil, nil }
func (f *Fake) Watch(context.Context, chan<- []byte)                {}
func (f *Fake) Run(context.Context) error                           { return nil }
