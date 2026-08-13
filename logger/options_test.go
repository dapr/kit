/*
Copyright 2021 The Dapr Authors
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
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptions(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		o := DefaultOptions()
		assert.Equal(t, defaultJSONOutput, o.JSONFormatEnabled)
		assert.Equal(t, undefinedAppID, o.appID)
		assert.Equal(t, defaultOutputLevel, o.OutputLevel)
		assert.Empty(t, o.OutputFile)
	})

	t.Run("set dapr ID", func(t *testing.T) {
		o := DefaultOptions()
		assert.Equal(t, undefinedAppID, o.appID)

		o.SetAppID("dapr-app")
		assert.Equal(t, "dapr-app", o.appID)
	})

	t.Run("attaching log related cmd flags", func(t *testing.T) {
		o := DefaultOptions()

		logLevelAsserted := false
		logFileAsserted := false
		testStringVarFn := func(p *string, name string, value string, usage string) {
			if name == "log-level" && value == defaultOutputLevel {
				logLevelAsserted = true
			}

			if name == "log-file" && value == "" {
				logFileAsserted = true
			}
		}

		logAsJSONAsserted := false
		testBoolVarFn := func(p *bool, name string, value bool, usage string) {
			if name == "log-as-json" && value == defaultJSONOutput {
				logAsJSONAsserted = true
			}
		}

		o.AttachCmdFlags(testStringVarFn, testBoolVarFn)

		// assert
		assert.True(t, logLevelAsserted)
		assert.True(t, logFileAsserted)
		assert.True(t, logAsJSONAsserted)
	})
}

func TestApplyOptionsToLoggers(t *testing.T) {
	testOptions := Options{
		JSONFormatEnabled: true,
		appID:             "dapr-app",
		OutputLevel:       "debug",
	}

	// Create two loggers
	testLoggers := []Logger{
		NewLogger("testLogger0"),
		NewLogger("testLogger1"),
	}

	for _, l := range testLoggers {
		l.EnableJSONOutput(false)
		l.SetOutputLevel(InfoLevel)
	}

	require.NoError(t, ApplyOptionsToLoggers(&testOptions))

	for _, l := range testLoggers {
		dl, ok := l.(*daprLogger)
		require.True(t, ok)

		assert.Equal(t, "dapr-app", dl.state.getAppID())
		assert.Equal(t, LevelDebug, slog.Level(dl.state.level.Load()))
		assert.True(t, dl.state.json.Load())
		assert.True(t, l.IsOutputLevelEnabled(DebugLevel))
	}
}

// TestApplyOptionsToLoggersLateCreated pins that a logger created after the
// options were applied inherits them. The logrus implementation only pushed
// options into the loggers that already existed, so anything constructed later
// silently kept text output at info level.
func TestApplyOptionsToLoggersLateCreated(t *testing.T) {
	require.NoError(t, ApplyOptionsToLoggers(&Options{
		JSONFormatEnabled: true,
		appID:             "late-app",
		OutputLevel:       "debug",
	}))

	t.Cleanup(func() {
		require.NoError(t, ApplyOptionsToLoggers(&Options{OutputLevel: "info"}))
	})

	var buf bytes.Buffer

	l := NewLogger("testLoggerCreatedAfterApply")
	l.SetOutput(&buf)
	l.Debug("late")

	var o map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &o))

	assert.Equal(t, "debug", o[logFieldLevel])
	assert.Equal(t, "late-app", o[logFieldAppID])
	assert.Equal(t, "late", o[logFieldMessage])
}

func TestApplyOptionsToLoggersFileOutput(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "dapr.log")

	testOptions := Options{
		OutputLevel: "debug",
		OutputFile:  logPath,
	}

	l := NewLogger("testLoggerFileOutput")

	require.NoError(t, ApplyOptionsToLoggers(&testOptions))
	t.Cleanup(func() {
		// Revert to stdout, which also closes the log file.
		require.NoError(t, ApplyOptionsToLoggers(&Options{
			OutputLevel: "info",
		}))
	})

	dl, ok := l.(*daprLogger)
	require.True(t, ok)

	dl.state.outMu.RLock()
	fileOut, ok := dl.state.out.(*os.File)
	dl.state.outMu.RUnlock()

	require.True(t, ok)
	assert.Equal(t, logPath, fileOut.Name())

	msg := "log-file-test-message"
	l.Info(msg)

	b, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(b), msg)
}

func TestApplyOptionsToLoggersFileOutputReapply(t *testing.T) {
	dir := t.TempDir()
	logPath1 := filepath.Join(dir, "dapr1.log")
	logPath2 := filepath.Join(dir, "dapr2.log")

	l := NewLogger("testLoggerReapply")

	t.Cleanup(func() {
		require.NoError(t, ApplyOptionsToLoggers(&Options{
			OutputLevel: "info",
		}))
	})

	// Apply first file output.
	require.NoError(t, ApplyOptionsToLoggers(&Options{
		OutputLevel: "debug",
		OutputFile:  logPath1,
	}))
	l.Info("message-one")

	// Re-apply with a different file — should close the first.
	require.NoError(t, ApplyOptionsToLoggers(&Options{
		OutputLevel: "debug",
		OutputFile:  logPath2,
	}))
	l.Info("message-two")

	b1, err := os.ReadFile(logPath1)
	require.NoError(t, err)
	assert.Contains(t, string(b1), "message-one")
	assert.NotContains(t, string(b1), "message-two")

	b2, err := os.ReadFile(logPath2)
	require.NoError(t, err)
	assert.Contains(t, string(b2), "message-two")
}
