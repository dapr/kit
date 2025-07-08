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
	"testing"

	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"

	"github.com/stretchr/testify/assert"
)

type mockX509Source struct{}

func (m *mockX509Source) GetX509SVID() (*x509svid.SVID, error) {
	return nil, nil
}

type mockJWTSource struct{}

func (m *mockJWTSource) FetchJWTSVID(context.Context, jwtsvid.Params) (*jwtsvid.SVID, error) {
	return nil, nil
}

func TestWithX509FromX509(t *testing.T) {
	source := &mockX509Source{}
	ctx := WithX509(t.Context(), source)

	retrieved, ok := X509From(ctx)
	assert.True(t, ok, "Failed to retrieve X509 source from context")
	assert.Equal(t, x509svid.Source(source), retrieved, "Retrieved source does not match the original source")
}

func TestWithJWTFromJWT(t *testing.T) {
	source := &mockJWTSource{}
	ctx := WithJWT(t.Context(), source)

	retrieved, ok := JWTFrom(ctx)
	assert.True(t, ok, "Failed to retrieve JWT source from context")
	assert.Equal(t, jwtsvid.Source(source), retrieved, "Retrieved source does not match the original source")
}

func TestWithFrom(t *testing.T) {
	x509Source := &mockX509Source{}
	ctx := WithX509(t.Context(), x509Source)

	// Should be able to retrieve using the legacy From function
	retrieved, ok := From(ctx)
	assert.True(t, ok, "Failed to retrieve X509 source from context using legacy From")
	assert.Equal(t, x509svid.Source(x509Source), retrieved, "Retrieved source does not match the original source using legacy From")
}
