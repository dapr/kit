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
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLog(t *testing.T, buf *bytes.Buffer) *Log {
	t.Helper()

	setHostname(t, "test-host")

	l := newLog("dapr.structured", newState())
	l.EnableJSONOutput(true)
	l.SetOutputLevel(DebugLevel)
	l.SetOutput(buf)

	return l
}

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	b, err := buf.ReadBytes('\n')
	require.NoError(t, err)

	var o map[string]any
	require.NoError(t, json.Unmarshal(b, &o))

	return o
}

func TestLogStructuredAttrs(t *testing.T) {
	var buf bytes.Buffer

	l := testLog(t, &buf)
	l.Info("component loaded", "component", "statestore.redis", "count", 3)

	o := decode(t, &buf)
	assert.Equal(t, "component loaded", o[logFieldMessage])
	assert.Equal(t, "statestore.redis", o["component"])
	assert.InDelta(t, float64(3), o["count"], 0.001)
	assert.Equal(t, "dapr.structured", o[logFieldScope])
}

// TestErrAttr covers the replacement for the trailing ": %v" that most error
// logging in Dapr used to end with.
func TestErrAttr(t *testing.T) {
	t.Run("error is rendered via Error()", func(t *testing.T) {
		var buf bytes.Buffer

		l := testLog(t, &buf)
		l.Error("failed to load component", Err(errors.New("connection refused")))

		o := decode(t, &buf)
		assert.Equal(t, "connection refused", o[FieldError])
		assert.Equal(t, "failed to load component", o[logFieldMessage])
	})

	t.Run("nil error is omitted", func(t *testing.T) {
		var buf bytes.Buffer

		l := testLog(t, &buf)
		l.Info("all good", Err(nil))

		o := decode(t, &buf)
		_, present := o[FieldError]
		assert.False(t, present, "a nil error should not emit an error field")
	})
}

func TestLogWith(t *testing.T) {
	var buf bytes.Buffer

	l := testLog(t, &buf)
	child := l.With("component", "redis")

	child.Info("saved")
	assert.Equal(t, "redis", decode(t, &buf)["component"])

	// The parent must be unaffected.
	l.Info("parent")

	o := decode(t, &buf)
	_, present := o["component"]
	assert.False(t, present, "attributes must not leak back to the parent")
}

// TestLogWithGroup pins that groups are flattened into dotted keys rather than
// nested objects. The Dapr log schema is flat and user queries index it that
// way, so this is a deliberate deviation from slog.JSONHandler.
func TestLogWithGroup(t *testing.T) {
	var buf bytes.Buffer

	l := testLog(t, &buf)

	// "type" is a reserved schema key, and forbidden as a bare attribute for
	// that reason. Inside a group it is emitted as "actor.type", so there is no
	// collision, and proving that is the point of this test.
	//nolint:sloglint
	l.WithGroup("actor").Info("activated", "type", "myactor")

	o := decode(t, &buf)
	assert.Equal(t, "myactor", o["actor.type"])
	assert.Equal(t, LogTypeLog, o[logFieldType], "the schema type field must not be shadowed by a group key")
}

// TestReservedKeysNotShadowed pins that a caller cannot displace a schema field
// by passing an attribute of the same name.
func TestReservedKeysNotShadowed(t *testing.T) {
	var buf bytes.Buffer

	l := testLog(t, &buf)
	l.Info("hello", logFieldScope, "hijacked", logFieldInstance, "hijacked")

	o := decode(t, &buf)
	assert.Equal(t, "dapr.structured", o[logFieldScope])
	assert.Equal(t, "test-host", o[logFieldInstance])
}

func TestLogFatal(t *testing.T) {
	var buf bytes.Buffer

	called := noExit(t)
	l := testLog(t, &buf)
	l.Fatal("boom", "reason", "timeout")

	assert.True(t, *called)

	o := decode(t, &buf)
	assert.Equal(t, "fatal", o[logFieldLevel])
	assert.Equal(t, "timeout", o["reason"])
}

// TestLegacyAndStructuredShareState is the property that lets a package migrate
// one call site at a time: the two faces of a logger are configured together.
func TestLegacyAndStructuredShareState(t *testing.T) {
	var buf bytes.Buffer

	l := testLog(t, &buf)
	legacy := l.Legacy()

	l.SetOutputLevel(ErrorLevel)
	assert.False(t, legacy.IsOutputLevelEnabled(InfoLevel))

	legacy.SetOutputLevel(DebugLevel)
	assert.True(t, l.Enabled(t.Context(), LevelDebug))
}

func TestNewSharesStateWithNewLogger(t *testing.T) {
	const name = "dapr.shared.state"

	structured := New(name)
	legacy := NewLogger(name)

	t.Cleanup(func() { structured.SetOutputLevel(InfoLevel) })

	structured.SetOutputLevel(ErrorLevel)
	assert.False(t, legacy.IsOutputLevelEnabled(InfoLevel),
		"New and NewLogger of the same name must share configuration")
}

// TestFromLogger covers the bridge that lets code receiving a Logger through a
// frozen API, such as every components-contrib constructor, reach the
// structured API.
func TestFromLogger(t *testing.T) {
	t.Run("converts a logger from this package in place", func(t *testing.T) {
		var buf bytes.Buffer

		legacy := newDaprLoggerState("dapr.bridge", newState())
		legacy.EnableJSONOutput(true)
		legacy.SetOutputLevel(DebugLevel)
		legacy.SetOutput(&buf)

		setHostname(t, "test-host")

		FromLogger(legacy).Info("via bridge", "key", "value")

		o := decode(t, &buf)
		assert.Equal(t, "via bridge", o[logFieldMessage])
		assert.Equal(t, "value", o["key"])
		assert.Equal(t, "dapr.bridge", o[logFieldScope], "scope must be preserved")
	})

	t.Run("renders through a third-party Logger rather than discarding", func(t *testing.T) {
		fake := &recordingLogger{}

		FromLogger(fake).Warn("careful", "attempt", 2)

		require.Len(t, fake.warns, 1)
		assert.Equal(t, "careful attempt=2", fake.warns[0])
	})
}

// recordingLogger is a third-party style Logger implementation: it is not one
// of this package's types, so FromLogger must route through the bridge.
type recordingLogger struct {
	nopLogger

	warns []string
}

func (r *recordingLogger) Warn(args ...any) {
	r.warns = append(r.warns, fmtSprint(args...))
}
