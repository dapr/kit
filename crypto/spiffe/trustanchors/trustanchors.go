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

	"github.com/spiffe/go-spiffe/v2/bundle/jwtbundle"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
)

// Interface exposes a SPIFFE trust anchor from a source.
// Allows consumers to get the current trust anchor bundle, and subscribe to
// bundle updates.
type Interface interface {
	// Source implements the SPIFFE trust anchor x509 bundle source.
	x509bundle.Source
	// Source implements the SPIFFE trust anchor jwt bundle source.
	jwtbundle.Source

	// CurrentTrustAnchors returns the current trust anchor PEM bundle.
	CurrentTrustAnchors(ctx context.Context) ([]byte, error)

	// Watch watches for changes to the trust domains and returns the PEM encoded
	// trust domain roots.
	// Returns when the given context is canceled.
	Watch(ctx context.Context, ch chan<- []byte)

	// Run starts the trust anchor source.
	Run(ctx context.Context) error
}
