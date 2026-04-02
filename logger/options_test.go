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
		assert.Equal(
			t,
			"dapr-app",
			(l.(*daprLogger)).logger.Data[logFieldAppID])
		assert.Equal(
			t,
			toLogrusLevel(DebugLevel),
			(l.(*daprLogger)).logger.Logger.GetLevel())
	}
}

func TestApplyOptionsToLoggersFileOutput(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "dapr.log")

	testOptions := Options{
		OutputLevel: "debug",
		OutputFile:  logPath,
	}

	l := NewLogger("testLoggerFileOutput")

	require.NoError(t, ApplyOptionsToLoggers(&testOptions))

	dl, ok := l.(*daprLogger)
	require.True(t, ok)
	fileOut, ok := dl.logger.Logger.Out.(*os.File)
	require.True(t, ok)
	assert.Equal(t, logPath, fileOut.Name())
	t.Cleanup(func() {
		// Revert to stdout, which also closes the log file.
		require.NoError(t, ApplyOptionsToLoggers(&Options{
			OutputLevel: "info",
		}))
	})

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

	t.Cleanup(func() {
		require.NoError(t, ApplyOptionsToLoggers(&Options{
			OutputLevel: "info",
		}))
	})

	b1, err := os.ReadFile(logPath1)
	require.NoError(t, err)
	assert.Contains(t, string(b1), "message-one")
	assert.NotContains(t, string(b1), "message-two")

	b2, err := os.ReadFile(logPath2)
	require.NoError(t, err)
	assert.Contains(t, string(b2), "message-two")
}
