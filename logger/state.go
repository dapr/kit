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
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
)

// state holds the logging configuration that callers may change at runtime,
// after loggers have already been created.
//
// slog handlers are expected to be immutable, but Dapr mutates the level,
// output destination and encoding long after loggers exist (see
// ApplyOptionsToLoggers, and the components that honour a logLevel metadata
// field). Keeping that configuration behind a pointer which handlers consult
// on every record reconciles the two: handler values stay immutable while the
// configuration they read does not.
//
// There is one state per named logger, mirroring the previous implementation
// where each named logger owned its own logrus.Logger. Loggers derived through
// WithFields/With share their parent's state, so adjusting the level on a
// derived logger affects the parent, as it always has.
type state struct {
	// level is the minimum enabled slog.Level, stored as an int64 so the hot
	// Enabled path is a single atomic load.
	level atomic.Int64

	// json selects JSON encoding over the logfmt-style text encoding.
	json atomic.Bool

	// appID is the app_id field value. Empty until SetAppID is called.
	appID atomic.Pointer[string]

	// outMu guards out. It is held for reading while a record is written so a
	// concurrent SetOutput cannot swap the writer mid-line.
	outMu sync.RWMutex
	out   io.Writer
}

// defaults holds the configuration applied to loggers created from now on.
//
// The logrus implementation only ever pushed options into the loggers that
// existed when ApplyOptionsToLoggers ran, so any logger constructed later
// silently kept the built-in defaults. Recording the options here instead means
// late-constructed loggers inherit them.
var defaults = struct {
	mu    sync.RWMutex
	level slog.Level
	json  bool
	appID string
	out   io.Writer
}{
	level: LevelInfo,
	json:  defaultJSONOutput,
	out:   os.Stdout,
}

func newState() *state {
	defaults.mu.RLock()
	defer defaults.mu.RUnlock()

	s := &state{out: defaults.out}
	s.level.Store(int64(defaults.level))
	s.json.Store(defaults.json)

	if defaults.appID != undefinedAppID {
		id := defaults.appID
		s.appID.Store(&id)
	}

	return s
}

func (s *state) enabled(l slog.Level) bool {
	return l >= slog.Level(s.level.Load())
}

func (s *state) setLevel(l slog.Level) {
	s.level.Store(int64(l))
}

func (s *state) setJSON(enabled bool) {
	s.json.Store(enabled)
}

func (s *state) setAppID(id string) {
	s.appID.Store(&id)
}

// getAppID returns the configured app_id, or "" if SetAppID was never called.
func (s *state) getAppID() string {
	if p := s.appID.Load(); p != nil {
		return *p
	}

	return ""
}

func (s *state) setOutput(w io.Writer) {
	s.outMu.Lock()
	s.out = w
	s.outMu.Unlock()
}

// write emits one complete, already-encoded record. The lock is held for the
// duration of the Write so records are never interleaved or torn by a
// concurrent setOutput.
func (s *state) write(b []byte) error {
	s.outMu.RLock()
	defer s.outMu.RUnlock()

	_, err := s.out.Write(b)

	return err
}
