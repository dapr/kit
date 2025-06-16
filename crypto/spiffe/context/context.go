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

package context

import (
	"context"

	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"

	"github.com/dapr/kit/crypto/spiffe"
)

type ctxkey int

const (
	x509SvidKey ctxkey = iota
	jwtSvidKey
)

// Deprecated: use WithX509 instead.
// With adds the x509 SVID source from the SPIFFE object to the context.
func With(ctx context.Context, spiffe *spiffe.SPIFFE) context.Context {
	return context.WithValue(ctx, x509SvidKey, spiffe.X509SVIDSource())
}

// Deprecated: use X509From instead.
// From retrieves the x509 SVID source from the context.
func From(ctx context.Context) (x509svid.Source, bool) {
	svid, ok := ctx.Value(x509SvidKey).(x509svid.Source)
	return svid, ok
}

// WithX509 adds an x509 SVID source to the context.
func WithX509(ctx context.Context, source x509svid.Source) context.Context {
	return context.WithValue(ctx, x509SvidKey, source)
}

// WithJWT adds a JWT SVID source to the context.
func WithJWT(ctx context.Context, source jwtsvid.Source) context.Context {
	return context.WithValue(ctx, jwtSvidKey, source)
}

// X509From retrieves the x509 SVID source from the context.
func X509From(ctx context.Context) (x509svid.Source, bool) {
	svid, ok := ctx.Value(x509SvidKey).(x509svid.Source)
	return svid, ok
}

// JWTFrom retrieves the JWT SVID source from the context.
func JWTFrom(ctx context.Context) (jwtsvid.Source, bool) {
	svid, ok := ctx.Value(jwtSvidKey).(jwtsvid.Source)
	return svid, ok
}

// WithSpiffe adds both X509 and JWT SVID sources to the context.
func WithSpiffe(ctx context.Context, spiffe *spiffe.SPIFFE) context.Context {
	ctx = WithX509(ctx, spiffe.X509SVIDSource())
	return WithJWT(ctx, spiffe.JWTSVIDSource())
}
