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

package logger

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGoldenOutput pins the exact bytes this package emits.
//
// Every expectation below was captured from the previous logrus-backed
// implementation before it was replaced, so this test is what guarantees the
// swap is invisible to users. The log schema is documented public API, and
// user log pipelines, the CLI end-to-end tests and the dapr integration suite
// all match on this text.
//
// If a change here is genuinely intended, it needs a release note and a docs
// update, not just a new expectation.
func TestGoldenOutput(t *testing.T) {
	tests := []struct {
		name string
		json bool
		fn   func(l Logger)
		want string
	}{
		{
			name: "text/plain",
			fn:   func(l Logger) { l.Info("hello world") },
			want: `time="<TIME>" level=info msg="hello world" instance=<HOST> scope=<SCOPE> type=log ver=unknown`,
		},
		{
			name: "json/plain",
			json: true,
			fn:   func(l Logger) { l.Info("hello world") },
			want: `{"instance":"<HOST>","level":"info","msg":"hello world","scope":"<SCOPE>","time":"<TIME>","type":"log","ver":"unknown"}`,
		},
		{
			// logrus quotes with %q, so inner quotes are backslash escaped.
			name: "text/quoting",
			fn:   func(l Logger) { l.Info(`msg with "quotes" and spaces`) },
			want: `time="<TIME>" level=info msg="msg with \"quotes\" and spaces" instance=<HOST> scope=<SCOPE> type=log ver=unknown`,
		},
		{
			// An empty message is omitted entirely in text mode. This is
			// logrus behaviour and several callers log at a level purely as a
			// signal, with no message.
			name: "text/empty message omitted",
			fn:   func(l Logger) { l.Info("") },
			want: `time="<TIME>" level=info instance=<HOST> scope=<SCOPE> type=log ver=unknown`,
		},
		{
			// ...but is always present in JSON.
			name: "json/empty message present",
			json: true,
			fn:   func(l Logger) { l.Info("") },
			want: `{"instance":"<HOST>","level":"info","msg":"","scope":"<SCOPE>","time":"<TIME>","type":"log","ver":"unknown"}`,
		},
		{
			// Values are bare unless they contain something outside
			// [a-zA-Z0-9] and -._/@^+ . An empty value stays bare.
			name: "text/fields sorted and quoted",
			fn: func(l Logger) {
				l.WithFields(map[string]any{"zebra": 1, "alpha": "a b", "empty": ""}).Info("with fields")
			},
			want: `time="<TIME>" level=info msg="with fields" alpha="a b" empty= instance=<HOST> scope=<SCOPE> type=log ver=unknown zebra=1`,
		},
		{
			name: "json/fields sorted",
			json: true,
			fn: func(l Logger) {
				l.WithFields(map[string]any{"zebra": 1, "alpha": "a b", "empty": ""}).Info("with fields")
			},
			want: `{"alpha":"a b","empty":"","instance":"<HOST>","level":"info","msg":"with fields","scope":"<SCOPE>","time":"<TIME>","type":"log","ver":"unknown","zebra":1}`,
		},
		{
			name: "text/log type",
			fn:   func(l Logger) { l.WithLogType(LogTypeRequest).Info("api called") },
			want: `time="<TIME>" level=info msg="api called" instance=<HOST> scope=<SCOPE> type=request ver=unknown`,
		},
		{
			// Warn renders as "warning", not slog's "WARN".
			name: "text/warn label",
			fn:   func(l Logger) { l.Warn("w") },
			want: `time="<TIME>" level=warning msg=w instance=<HOST> scope=<SCOPE> type=log ver=unknown`,
		},
		{
			name: "text/debug label",
			fn:   func(l Logger) { l.Debug("d") },
			want: `time="<TIME>" level=debug msg=d instance=<HOST> scope=<SCOPE> type=log ver=unknown`,
		},
		{
			name: "text/error label",
			fn:   func(l Logger) { l.Error("e") },
			want: `time="<TIME>" level=error msg=e instance=<HOST> scope=<SCOPE> type=log ver=unknown`,
		},
		{
			// The deprecated variadic methods concatenate with fmt.Sprint.
			name: "text/sprint semantics",
			fn:   func(l Logger) { l.Info("a", "b", 3) },
			want: `time="<TIME>" level=info msg=ab3 instance=<HOST> scope=<SCOPE> type=log ver=unknown`,
		},
		{
			// app_id sorts ahead of every other schema field, which the CLI
			// end-to-end tests rely on.
			name: "text/app id first",
			fn:   func(l Logger) { l.SetAppID("myapp"); l.Info("with app id") },
			want: `time="<TIME>" level=info msg="with app id" app_id=myapp instance=<HOST> scope=<SCOPE> type=log ver=unknown`,
		},
		{
			name: "json/app id first",
			json: true,
			fn:   func(l Logger) { l.SetAppID("myapp"); l.Info("with app id") },
			want: `{"app_id":"myapp","instance":"<HOST>","level":"info","msg":"with app id","scope":"<SCOPE>","time":"<TIME>","type":"log","ver":"unknown"}`,
		},
		{
			// encoding/json escapes HTML by default and logrus inherited that
			// through json.Encoder, so angle brackets and ampersands are
			// emitted as <, > and & rather than literally.
			name: "json/html escaped",
			json: true,
			fn:   func(l Logger) { l.Info("a <b> & c") },
			want: `{"instance":"<HOST>","level":"info","msg":"a \u003cb\u003e \u0026 c","scope":"<SCOPE>","time":"<TIME>","type":"log","ver":"unknown"}`,
		},
		{
			// Errors render via Error(), not Go's struct formatting.
			name: "json/error value",
			json: true,
			fn: func(l Logger) {
				l.WithFields(map[string]any{"error": errors.New("boom")}).Info("failed")
			},
			want: `{"error":"boom","instance":"<HOST>","level":"info","msg":"failed","scope":"<SCOPE>","time":"<TIME>","type":"log","ver":"unknown"}`,
		},
	}

	timeRE := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T[0-9:.+\-Z]+`)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			setHostname(t, "test-host")

			DaprVersion = "unknown"

			l := newDaprLoggerState("dapr.golden", newState())
			l.EnableJSONOutput(tt.json)
			l.SetOutputLevel(DebugLevel)
			l.SetOutput(&buf)

			tt.fn(l)

			got := strings.TrimSuffix(buf.String(), "\n")
			got = timeRE.ReplaceAllString(got, "<TIME>")
			got = strings.ReplaceAll(got, "test-host", "<HOST>")
			got = strings.ReplaceAll(got, "dapr.golden", "<SCOPE>")

			assert.Equal(t, tt.want, got)
		})
	}
}
