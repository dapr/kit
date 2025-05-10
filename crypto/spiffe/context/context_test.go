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
	ctx := WithX509(context.Background(), source)

	retrieved, ok := X509From(ctx)
	if !ok {
		t.Error("Failed to retrieve X509 source from context")
	}
	if retrieved != source {
		t.Error("Retrieved source does not match the original source")
	}
}

func TestWithJWTFromJWT(t *testing.T) {
	source := &mockJWTSource{}
	ctx := WithJWT(context.Background(), source)

	retrieved, ok := JWTFrom(ctx)
	if !ok {
		t.Error("Failed to retrieve JWT source from context")
	}
	if retrieved != source {
		t.Error("Retrieved source does not match the original source")
	}
}

func TestWithFrom(t *testing.T) {
	x509Source := &mockX509Source{}
	ctx := WithX509(context.Background(), x509Source)

	// Should be able to retrieve using the legacy From function
	retrieved, ok := From(ctx)
	if !ok {
		t.Error("Failed to retrieve X509 source from context using legacy From")
	}
	if retrieved != x509Source {
		t.Error("Retrieved source does not match the original source using legacy From")
	}
}
