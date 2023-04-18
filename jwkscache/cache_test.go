/*
Copyright 2023 The Dapr Authors
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

package jwkscache

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapr/kit/logger"
)

const (
	testJWKS1 = `{"keys":[{"kid":"mykey","alg":"RS256","kty":"RSA","use":"sig","e":"AQAB","n":"3I2mdIK4mRRu-ywMrYjUZzBxt0NlAVLrMhGlaJsby7PWTMiLpZVip4SBD9GwnCU0TGFD7k2-7tfs0y9U6WV7MwgCjc9m_DUUGbE-kKjEU7JYkLzYlndys-6xuhD4Jf1hu9AZVdfXftpWSy_NNg6fVwTH4nckOAbOSL1hXToOYWQcDDW95Rhw3U4z04PqssEpRKn5KGBuTahNNNiZcWns99pChpLTxgdm93LjMBI1KCGBpOaz7fcQJ9V3c6rSwMKyY3IPm1LwS6PIs7xb2ZJ0Eb8A6MtCkGhgNsodpkxhqKbqtxI-KqTuZy9g4jb8WKjJq9lB9q-HPHoQqIEDom6P8w"}]}`
	testJWKS2 = `{"keys":[{"kid":"mykey","alg":"RS256","kty":"RSA","use":"sig","e":"AQAB","n":"3I2mdIK4mRRu-ywMrYjUZzBxt0NlAVLrMhGlaJsby7PWTMiLpZVip4SBD9GwnCU0TGFD7k2-7tfs0y9U6WV7MwgCjc9m_DUUGbE-kKjEU7JYkLzYlndys-6xuhD4Jf1hu9AZVdfXftpWSy_NNg6fVwTH4nckOAbOSL1hXToOYWQcDDW95Rhw3U4z04PqssEpRKn5KGBuTahNNNiZcWns99pChpLTxgdm93LjMBI1KCGBpOaz7fcQJ9V3c6rSwMKyY3IPm1LwS6PIs7xb2ZJ0Eb8A6MtCkGhgNsodpkxhqKbqtxI-KqTuZy9g4jb8WKjJq9lB9q-HPHoQqIEDom6P8w"},{"alg":"RS256","kty":"RSA","use":"sig","n":"yeNlzlub94YgerT030codqEztjfU_S6X4DbDA_iVKkjAWtYfPHDzz_sPCT1Axz6isZdf3lHpq_gYX4Sz-cbe4rjmigxUxr-FgKHQy3HeCdK6hNq9ASQvMK9LBOpXDNn7mei6RZWom4wo3CMvvsY1w8tjtfLb-yQwJPltHxShZq5-ihC9irpLI9xEBTgG12q5lGIFPhTl_7inA1PFK97LuSLnTJzW0bj096v_TMDg7pOWm_zHtF53qbVsI0e3v5nmdKXdFf9BjIARRfVrbxVxiZHjU6zL6jY5QJdh1QCmENoejj_ytspMmGW7yMRxzUqgxcAqOBpVm0b-_mW3HoBdjQ","e":"AQAB","kid":"testkey"}]}`
)

func TestJWKSCache(t *testing.T) {
	log := logger.NewLogger("test")

	t.Run("init with value", func(t *testing.T) {
		cache := NewJWKSCache(testJWKS1, log)
		err := cache.initCache(context.Background())
		require.NoError(t, err)

		set := cache.KeySet()
		require.Equal(t, 1, set.Len())

		key, ok := set.LookupKeyID("mykey")
		require.True(t, ok)
		require.NotNil(t, key)
	})

	t.Run("init with base64-encoded value", func(t *testing.T) {
		cache := NewJWKSCache(base64.StdEncoding.EncodeToString([]byte(testJWKS1)), log)
		err := cache.initCache(context.Background())
		require.NoError(t, err)

		set := cache.KeySet()
		require.Equal(t, 1, set.Len())

		key, ok := set.LookupKeyID("mykey")
		require.True(t, ok)
		require.NotNil(t, key)
	})

	t.Run("init with local file", func(t *testing.T) {
		// Create a temporary directory and put the JWKS in there
		dir := t.TempDir()
		path := filepath.Join(dir, "jwks.json")
		err := os.WriteFile(path, []byte(testJWKS1), 0o666)
		require.NoError(t, err)

		// Should wait for first file to be loaded before initialization is reported as completed
		cache := NewJWKSCache(path, log)
		err = cache.initCache(context.Background())
		require.NoError(t, err)

		set := cache.KeySet()
		require.Equal(t, 1, set.Len())

		key, ok := set.LookupKeyID("mykey")
		require.True(t, ok)
		require.NotNil(t, key)

		// Sleep 1s before writing the file
		time.Sleep(time.Second)

		// Update the file and verify it's picked up
		err = os.WriteFile(path, []byte(testJWKS2), 0o666)
		require.NoError(t, err)

		assert.Eventually(t, func() bool {
			return cache.KeySet().Len() == 2
		}, 5*time.Second, 50*time.Millisecond)

		set = cache.KeySet()
		key, ok = set.LookupKeyID("mykey")
		require.True(t, ok)
		require.NotNil(t, key)
		key, ok = set.LookupKeyID("testkey")
		require.True(t, ok)
		require.NotNil(t, key)
	})

	t.Run("init with HTTP client", func(t *testing.T) {
		// Create a custom HTTP client with a RoundTripper that doesn't require starting a TCP listener
		client := &http.Client{
			Transport: roundTripFn(func(r *http.Request) *http.Response {
				if r.Method != http.MethodGet || r.URL.Path != "/jwks.json" {
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Header:     make(http.Header),
					}
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"content-type": []string{"application/json"},
					},
					Body: io.NopCloser(strings.NewReader(testJWKS1)),
				}
			}),
		}

		cache := NewJWKSCache("http://localhost/jwks.json", log)
		cache.SetHTTPClient(client)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := cache.initCache(ctx)
		require.NoError(t, err)

		set := cache.KeySet()
		require.Equal(t, 1, set.Len())

		key, ok := set.LookupKeyID("mykey")
		require.True(t, ok)
		require.NotNil(t, key)
	})
}

type roundTripFn func(req *http.Request) *http.Response

func (f roundTripFn) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
