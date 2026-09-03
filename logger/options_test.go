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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
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
		logTimestampFormatAsserted := false
		testStringVarFn := func(p *string, name string, value string, usage string) {
			if name == "log-level" && value == defaultOutputLevel {
				logLevelAsserted = true
			}

			if name == "log-file" && value == "" {
				logFileAsserted = true
			}

			if name == "log-timestamp-format" && value == "" {
				logTimestampFormatAsserted = true
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
		assert.True(t, logTimestampFormatAsserted)
		assert.True(t, logAsJSONAsserted)
	})

	// Flag names are load-bearing: they become D3E chart annotations, so a
	// rename is a breaking change for users who have already set them. Assert
	// the exact registered set rather than spot-checking individual names, so
	// that adding, removing or renaming a flag fails here deliberately.
	t.Run("registers the exact set of log flags", func(t *testing.T) {
		o := DefaultOptions()

		stringFlags := map[string]string{}
		boolFlags := map[string]bool{}

		o.AttachCmdFlags(
			func(p *string, name string, value string, usage string) {
				stringFlags[name] = value
				assert.NotEmpty(t, usage, "flag --%s must have usage text", name)
			},
			func(p *bool, name string, value bool, usage string) {
				boolFlags[name] = value
				assert.NotEmpty(t, usage, "flag --%s must have usage text", name)
			},
		)

		assert.Equal(t, map[string]string{
			"log-level":            defaultOutputLevel,
			"log-file":             "",
			"log-timestamp-format": "",
			"log-file-max-size":    "",
			"log-file-max-backups": "",
			"log-file-max-age":     "",
			"log-file-compression": "",
			"log-outputs":          "",
		}, stringFlags)

		assert.Equal(t, map[string]bool{
			"log-as-json": defaultJSONOutput,
		}, boolFlags)
	})
}

func TestApplyOptionsToLoggers(t *testing.T) {
	testOptions := Options{
		JSONFormatEnabled: true,
		appID:             "dapr-app",
		OutputLevel:       "debug",
		TimestampFormat:   "2006/01/02 15:04:05.000",
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
		assert.Equal(
			t,
			"2006/01/02 15:04:05.000",
			(l.(*daprLogger)).timestampFormat)
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
	t.Cleanup(func() {
		// Revert to stdout, which also closes the log file.
		require.NoError(t, ApplyOptionsToLoggers(&Options{
			OutputLevel: "info",
		}))
	})

	dl, ok := l.(*daprLogger)
	require.True(t, ok)
	fileOut, ok := dl.logger.Logger.Out.(*os.File)
	require.True(t, ok)
	assert.Equal(t, logPath, fileOut.Name())

	msg := "log-file-test-message"
	l.Info(msg)

	b, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(b), msg)
}

func TestLogFileTee(t *testing.T) {
	// t.TempDir() must be called before the cleanup below is registered.
	// Cleanups run LIFO, so registering TempDir's RemoveAll first makes it run
	// last — after the cleanup that closes the log file. The reverse order
	// fails on Windows, which cannot delete a file that is still open.
	logPath := filepath.Join(t.TempDir(), "dapr.log")

	var console bytes.Buffer

	consoleWriter = &console

	t.Cleanup(func() {
		consoleWriter = os.Stdout
		// Re-point all registered loggers back at the real stdout, which also
		// closes the log file. Doing this before restoring consoleWriter would
		// leave them aimed at the dead test buffer.
		o := DefaultOptions()
		require.NoError(t, ApplyOptionsToLoggers(&o))
	})

	o := DefaultOptions()
	o.outputsStr = "stdout," + logPath

	l := NewLogger("testLoggerTee")

	require.NoError(t, ApplyOptionsToLoggers(&o))

	msg := "hello-tee"
	l.Info(msg)

	b, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(b), msg, "message should reach the file")
	assert.Contains(t, console.String(), msg, "message should also reach the console")
}

func TestLogFileTeeDisabledKeepsFileOnly(t *testing.T) {
	// TempDir before Cleanup — see the ordering note in TestLogFileTee.
	logPath := filepath.Join(t.TempDir(), "dapr.log")

	var console bytes.Buffer

	consoleWriter = &console

	t.Cleanup(func() {
		consoleWriter = os.Stdout
		o := DefaultOptions()
		require.NoError(t, ApplyOptionsToLoggers(&o))
	})

	o := DefaultOptions()
	o.outputsStr = logPath
	// No console entry in --log-outputs: the file is the complete
	// destination list, so the console must stay silent.

	l := NewLogger("testLoggerTeeDisabled")

	require.NoError(t, ApplyOptionsToLoggers(&o))

	msg := "file-only"
	l.Info(msg)

	b, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(b), msg)
	assert.NotContains(t, console.String(), msg, "console must stay silent when tee is off")
}

func TestNewFileWriter(t *testing.T) {
	t.Run("rotation options build a rotating writer", func(t *testing.T) {
		o := DefaultOptions()
		o.OutputFile = filepath.Join(t.TempDir(), "dapr.log")
		o.outputFileMaxSize = new(uint)
		*o.outputFileMaxSize = 1
		o.outputFileMaxBackups = new(uint)
		*o.outputFileMaxBackups = 3
		o.outputFileMaxAge = new(uint)
		*o.outputFileMaxAge = 7
		o.outputFileCompression = compressionGzip

		w, c, err := newFileWriter(o.OutputFile, &o)
		require.NoError(t, err)

		lj, ok := w.(*lumberjack.Logger)
		require.True(t, ok, "expected a lumberjack writer")
		assert.Equal(t, o.OutputFile, lj.Filename)
		assert.Equal(t, 1, lj.MaxSize)
		assert.Equal(t, 3, lj.MaxBackups)
		assert.Equal(t, 7, lj.MaxAge)
		assert.True(t, lj.Compress)

		require.NoError(t, c.Close())
	})

	t.Run("no rotation options keeps a plain append file", func(t *testing.T) {
		o := DefaultOptions()
		o.OutputFile = filepath.Join(t.TempDir(), "dapr.log")

		w, c, err := newFileWriter(o.OutputFile, &o)
		require.NoError(t, err)

		_, ok := w.(*os.File)
		assert.True(t, ok, "expected a plain *os.File when no rotation option is set")

		require.NoError(t, c.Close())
	})

	t.Run("compress alone is enough to enable rotation", func(t *testing.T) {
		o := DefaultOptions()
		o.OutputFile = filepath.Join(t.TempDir(), "dapr.log")
		o.outputFileCompression = compressionGzip

		w, c, err := newFileWriter(o.OutputFile, &o)
		require.NoError(t, err)

		_, ok := w.(*lumberjack.Logger)
		assert.True(t, ok)

		require.NoError(t, c.Close())
	})

	t.Run("invalid flag values fail validation", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			apply func(o *Options)
		}{
			{"max-size not a number", func(o *Options) { o.outputFileMaxSizeStr = "not-a-number" }},
			{"max-backups negative", func(o *Options) { o.outputFileMaxBackupsStr = "-1" }},
			{"max-age not a number", func(o *Options) { o.outputFileMaxAgeStr = "7d" }},
			{"unknown compression", func(o *Options) { o.outputFileCompressionStr = "zstd" }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				o := DefaultOptions()
				o.OutputFile = filepath.Join(t.TempDir(), "dapr.log")
				tc.apply(&o)

				require.Error(t, o.validate())
			})
		}
	})

	t.Run("compression values parse", func(t *testing.T) {
		for str, want := range map[string]logFileCompression{
			"":     compressionNone,
			"none": compressionNone,
			"gzip": compressionGzip,
		} {
			o := DefaultOptions()
			o.outputFileCompressionStr = str

			require.NoError(t, o.validate())
			assert.Equal(t, want, o.outputFileCompression)
		}
	})
}

func TestParseOptionalUint(t *testing.T) {
	t.Run("empty means not provided", func(t *testing.T) {
		got, err := parseOptionalUint("log-file-max-size", "")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("parses a non-negative integer", func(t *testing.T) {
		got, err := parseOptionalUint("log-file-max-size", "42")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, uint(42), *got)
	})

	t.Run("rejects negatives and non-numbers", func(t *testing.T) {
		for _, v := range []string{"-1", "abc", "1.5", " 1"} {
			_, err := parseOptionalUint("log-file-max-size", v)
			require.Error(t, err, "value %q should be rejected", v)
		}
	})
}

func TestApplyOptionsToLoggersRotation(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "dapr.log")

	o := DefaultOptions()
	o.OutputFile = logPath
	o.outputFileMaxSizeStr = "1"

	l := NewLogger("testLoggerRotation")

	require.NoError(t, ApplyOptionsToLoggers(&o))

	t.Cleanup(func() {
		d := DefaultOptions()
		require.NoError(t, ApplyOptionsToLoggers(&d))
	})

	msg := "rotating-message"
	l.Info(msg)

	b, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(b), msg)
}

func TestInertFileOptionsWarn(t *testing.T) {
	// TempDir before Cleanup — see the ordering note in TestLogFileTee.
	logPath := filepath.Join(t.TempDir(), "dapr.log")

	var console bytes.Buffer

	consoleWriter = &console

	t.Cleanup(func() {
		consoleWriter = os.Stdout
		o := DefaultOptions()
		require.NoError(t, ApplyOptionsToLoggers(&o))
	})

	const warning = "have no effect because no file destination is configured"

	// A rotation option without any file destination warns on the console.
	o := DefaultOptions()
	o.outputFileMaxSizeStr = "1"
	require.NoError(t, ApplyOptionsToLoggers(&o))
	assert.Contains(t, console.String(), warning)

	// A default configuration does not warn.
	console.Reset()

	o = DefaultOptions()
	require.NoError(t, ApplyOptionsToLoggers(&o))
	assert.NotContains(t, console.String(), warning)

	// The same options with a file destination are effective, so no warning.
	console.Reset()

	o = DefaultOptions()
	o.OutputFile = logPath
	o.outputFileMaxSizeStr = "1"
	require.NoError(t, ApplyOptionsToLoggers(&o))
	assert.NotContains(t, console.String(), warning)
}

func TestRotationKeepsFilePermissionParity(t *testing.T) {
	dir := t.TempDir()

	plainPath := filepath.Join(dir, "plain.log")
	o := DefaultOptions()
	o.OutputFile = plainPath

	w, c, err := newFileWriter(o.OutputFile, &o)
	require.NoError(t, err)
	_, err = w.Write([]byte("x\n"))
	require.NoError(t, err)
	require.NoError(t, c.Close())

	rotPath := filepath.Join(dir, "rotating.log")
	o = DefaultOptions()
	o.OutputFile = rotPath
	o.outputFileMaxSize = new(uint)
	*o.outputFileMaxSize = 1

	w, c, err = newFileWriter(o.OutputFile, &o)
	require.NoError(t, err)
	_, err = w.Write([]byte("x\n"))
	require.NoError(t, err)
	require.NoError(t, c.Close())

	plainInfo, err := os.Stat(plainPath)
	require.NoError(t, err)
	rotInfo, err := os.Stat(rotPath)
	require.NoError(t, err)

	// Compare the two paths rather than asserting an absolute mode, so the
	// test is immune to the process umask and to Windows permission quirks.
	assert.Equal(t, plainInfo.Mode(), rotInfo.Mode(),
		"enabling rotation must not change log file permissions (lumberjack alone would create 0600 where the plain path creates 0644)")
}

// TestFileRotationActuallyRotates is the behavioural counterpart to
// TestNewFileWriter: that test only asserts the lumberjack struct is populated
// correctly, which would still pass if the rotating writer were never actually
// installed as the log output, or if MaxSize were interpreted in the wrong
// unit. This one drives real log volume through the configured logger and
// observes the rotation on disk.
//
// Only size-based rotation is asserted, because lumberjack performs it
// synchronously on the write that would exceed MaxSize. Compression and
// MaxBackups pruning run on a background goroutine ("milling"), so asserting
// on a .gz file appearing would be timing-dependent and flaky in CI.
func TestFileRotationActuallyRotates(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "dapr.log")

	o := DefaultOptions()
	o.OutputFile = logPath
	o.outputFileMaxSizeStr = "1" // 1 MB

	l := NewLogger("testLoggerRotationBehaviour")

	require.NoError(t, ApplyOptionsToLoggers(&o))

	t.Cleanup(func() {
		d := DefaultOptions()
		require.NoError(t, ApplyOptionsToLoggers(&d))
	})

	// Write comfortably more than 1 MB. Each line carries a ~2 KB payload, so
	// ~1500 lines is roughly 3 MB and forces at least one rotation.
	payload := strings.Repeat("x", 2048)
	for range 1500 {
		l.Info(payload)
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var active, archives int

	for _, e := range entries {
		if e.Name() == "dapr.log" {
			active++
			continue
		}
		// lumberjack names archives dapr-<timestamp>.log[.gz]
		if strings.HasPrefix(e.Name(), "dapr-") {
			archives++
		}
	}

	assert.Equal(t, 1, active, "the active log file should still exist")
	assert.Positive(t, archives,
		"expected at least one rotated archive after writing ~3MB with max-size=1MB, found none in %v", entries)

	// The active file must have been truncated by the rotation, i.e. it should
	// be well under the total volume written.
	fi, err := os.Stat(logPath)
	require.NoError(t, err)
	assert.Less(t, fi.Size(), int64(2*1024*1024),
		"active file should have been rolled, not grown past MaxSize unchecked")
}

// TestTeeWithRotation covers the combination LNRS actually configures: file
// output that both rotates and keeps writing to the console. setLogOutput
// wraps the rotating writer in an io.MultiWriter, and nothing else exercises
// that composition.
func TestTeeWithRotation(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "dapr.log")

	var console bytes.Buffer

	consoleWriter = &console

	t.Cleanup(func() {
		consoleWriter = os.Stdout
		d := DefaultOptions()
		require.NoError(t, ApplyOptionsToLoggers(&d))
	})

	o := DefaultOptions()
	o.outputsStr = "stdout," + logPath
	o.outputFileMaxSizeStr = "1"
	o.outputFileCompressionStr = "gzip"

	l := NewLogger("testLoggerTeeRotation")

	require.NoError(t, ApplyOptionsToLoggers(&o))

	msg := "tee-and-rotate"
	l.Info(msg)

	b, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(b), msg, "rotating writer should still receive the message")
	assert.Contains(t, console.String(), msg, "console should still receive the message when rotation is on")
}

// TestLogOutputsUnionWithLogFile covers both flags together: --log-file merges
// into the destination list rather than being overridden or duplicated.
func TestLogOutputsUnionWithLogFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "dapr.log")

	var console bytes.Buffer

	consoleWriter = &console

	t.Cleanup(func() {
		consoleWriter = os.Stdout
		o := DefaultOptions()
		require.NoError(t, ApplyOptionsToLoggers(&o))
	})

	o := DefaultOptions()
	o.OutputFile = logPath
	o.outputsStr = "stdout"

	l := NewLogger("testLoggerUnion")

	require.NoError(t, ApplyOptionsToLoggers(&o))

	msg := "union-msg"
	l.Info(msg)

	b, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(b), msg, "the --log-file destination should receive the message")
	assert.Contains(t, console.String(), msg, "the stdout destination should receive the message")
}

// TestLogOutputsDeduplicates proves a destination listed twice writes once —
// two writers on one file would double every line, and two lumberjack
// instances on one path would corrupt rotation.
func TestLogOutputsDeduplicates(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "dapr.log")

	o := DefaultOptions()
	o.OutputFile = logPath
	o.outputsStr = logPath + "," + logPath

	l := NewLogger("testLoggerDedupe")

	require.NoError(t, ApplyOptionsToLoggers(&o))

	t.Cleanup(func() {
		d := DefaultOptions()
		require.NoError(t, ApplyOptionsToLoggers(&d))
	})

	msg := "dedupe-msg"
	l.Info(msg)

	b, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(b), msg), "a deduplicated destination must receive the message exactly once")
}

func TestLogOutputsStderr(t *testing.T) {
	var stderrBuf bytes.Buffer

	stderrWriter = &stderrBuf

	t.Cleanup(func() {
		stderrWriter = os.Stderr
		o := DefaultOptions()
		require.NoError(t, ApplyOptionsToLoggers(&o))
	})

	o := DefaultOptions()
	o.outputsStr = "stderr"

	l := NewLogger("testLoggerStderr")

	require.NoError(t, ApplyOptionsToLoggers(&o))

	msg := "stderr-msg"
	l.Info(msg)

	assert.Contains(t, stderrBuf.String(), msg)
}

// TestApplyOptionsDestinationOpenFailure pins the error-unwind path: when a
// destination fails to open mid-list, the apply must fail without redirecting
// any logger away from its previous output.
func TestApplyOptionsDestinationOpenFailure(t *testing.T) {
	// A directory cannot be opened O_WRONLY, so it fails as a file destination.
	dir := t.TempDir()

	var console bytes.Buffer

	consoleWriter = &console

	t.Cleanup(func() {
		consoleWriter = os.Stdout
		o := DefaultOptions()
		require.NoError(t, ApplyOptionsToLoggers(&o))
	})

	l := NewLogger("testLoggerOpenFailure")

	// Point loggers at the captured console first, so "unchanged" is
	// observable after the failed apply.
	o := DefaultOptions()
	require.NoError(t, ApplyOptionsToLoggers(&o))

	bad := DefaultOptions()
	bad.outputsStr = "stdout," + dir
	require.Error(t, ApplyOptionsToLoggers(&bad))

	msg := "still-on-previous-output"
	l.Info(msg)
	assert.Contains(t, console.String(), msg,
		"loggers must keep their previous output when a destination fails to open")
}

func TestValidateDestinations(t *testing.T) {
	t.Run("consoles sort before files, entries trimmed and deduplicated", func(t *testing.T) {
		o := DefaultOptions()
		o.OutputFile = "/var/log/a.log"
		o.outputsStr = " /var/log/a.log , stdout,, stderr "

		require.NoError(t, o.validate())
		// File paths pass through filepath.Clean, whose separator differs by
		// OS — compare against the cleaned form so the assertion is
		// platform-correct (this is what broke Windows CI).
		assert.Equal(t, []string{"stdout", "stderr", filepath.Clean("/var/log/a.log")}, o.outputDestinations)
	})

	t.Run("path spellings normalize to one destination", func(t *testing.T) {
		o := DefaultOptions()
		o.OutputFile = "a.log"
		o.outputsStr = "./a.log"

		require.NoError(t, o.validate())
		assert.Equal(t, []string{"a.log"}, o.outputDestinations)
	})

	t.Run("empty configuration means no destinations", func(t *testing.T) {
		o := DefaultOptions()

		require.NoError(t, o.validate())
		assert.Empty(t, o.outputDestinations)
	})
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
