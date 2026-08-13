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
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

// Log is a structured logger scoped to a name.
//
// It embeds *slog.Logger, so Debug, Info, Warn and Error take a message
// followed by alternating keys and values, or slog.Attr values:
//
//	log.Info("component loaded", "component", name, "type", kind)
//	log.Error("failed to load component", logger.Err(err))
//
// Arguments are only formatted if the level is enabled, so a disabled call
// costs an atomic load and nothing else. On paths hot enough for that to
// matter, LogAttrs avoids boxing the arguments at the call site entirely:
//
//	log.LogAttrs(ctx, logger.LevelDebug, "policy resolved",
//	    slog.String("app", app), slog.Int("count", n))
//
// Log does not satisfy the deprecated [Logger] interface: the embedded slog
// methods take a leading message string, which is incompatible with Logger's
// variadic fmt.Sprint signatures. Use [Log.Legacy] where a Logger is required,
// such as a components-contrib constructor.
type Log struct {
	*slog.Logger

	name    string
	state   *state
	handler *handler
}

var (
	globalLogs     = map[string]*Log{}
	globalLogsLock sync.RWMutex
)

// New returns the structured logger for name, creating it if necessary.
//
// Names are dot-separated and hierarchical, mirroring the package they belong
// to, for example "dapr.runtime.actors.placement". The name is emitted as the
// scope field.
//
// Repeated calls with the same name return the same logger, and share
// configuration with the [Logger] of the same name returned by [NewLogger].
func New(name string) *Log {
	globalLogsLock.Lock()
	defer globalLogsLock.Unlock()

	if l, ok := globalLogs[name]; ok {
		return l
	}

	l := newLog(name, sharedState(name))
	globalLogs[name] = l

	return l
}

func newLog(name string, s *state) *Log {
	h := newHandler(s, name)

	return &Log{
		Logger:  slog.New(h),
		name:    name,
		state:   s,
		handler: h,
	}
}

// derive returns a Log wrapping an already-built handler, sharing state.

// Name returns the logger's scope.
func (l *Log) Name() string {
	return l.name
}

// With returns a logger that includes the given attributes on every record.
func (l *Log) With(args ...any) *Log {
	if len(args) == 0 {
		return l
	}

	h, _ := l.Logger.With(args...).Handler().(*handler)

	return l.derive(h)
}

// WithGroup returns a logger whose subsequent attribute keys are prefixed with
// name. Output stays flat: keys are dot-joined, because the Dapr log schema is
// a flat set of top-level fields.
func (l *Log) WithGroup(name string) *Log {
	if name == "" {
		return l
	}

	h, _ := l.Logger.WithGroup(name).Handler().(*handler)

	return l.derive(h)
}

// WithLogType returns a logger emitting a different type field. The default is
// LogTypeLog; the API access loggers use LogTypeRequest.
func (l *Log) WithLogType(logType string) *Log {
	return l.derive(l.handler.withLogType(logType))
}

// Legacy adapts this logger to the deprecated [Logger] interface, for the
// printf-style APIs that have not been migrated yet, such as
// components-contrib constructors and the durabletask-go backend.
//
// The returned Logger shares this logger's configuration and scope.
func (l *Log) Legacy() Logger {
	return &daprLogger{
		name:    l.name,
		state:   l.state,
		handler: l.handler,
		log:     l.Logger,
	}
}

// FromLogger returns a structured view over a [Logger], sharing its scope,
// attributes and configuration.
//
// This is the migration entry point for code that receives a Logger through an
// API that cannot change. Every components-contrib component is constructed as
// func NewX(logger.Logger) ..., a signature third-party component authors
// depend on, so without this bridge that code could never reach the structured
// API. A component migrates without touching its signature:
//
//	func NewRedis(l logger.Logger) state.Store {
//	    return &redis{log: logger.FromLogger(l)}
//	}
//
// Loggers produced by this package are converted directly. Any other
// implementation is wrapped in an adapter that renders records through its
// printf methods, so third-party Logger implementations keep working.
func FromLogger(l Logger) *Log {
	if v, ok := l.(*daprLogger); ok {
		return v.Structured()
	}

	h := &bridgeHandler{target: l}

	return &Log{
		Logger: slog.New(h),
		name:   "",
		state:  nil,
	}
}

// Fatal logs at fatal level then exits the process with status 1.
func (l *Log) Fatal(msg string, args ...any) {
	l.log(LevelFatal, msg, args...)
	exit(1)
}

// Enabled reports whether the logger emits records at the given level. Use it
// to guard genuinely expensive argument construction; slog already avoids
// formatting for disabled levels, so most call sites do not need it.
func (l *Log) Enabled(ctx context.Context, level slog.Level) bool {
	return l.Logger.Enabled(ctx, level)
}

// The configuration setters below are no-ops on a logger obtained from
// [FromLogger] over a third-party Logger, whose output this package does not
// own.

// SetOutputLevel sets the minimum level this logger emits. It also affects
// loggers derived from it, and the [Logger] of the same name.
func (l *Log) SetOutputLevel(level LogLevel) {
	if l.state == nil {
		return
	}

	l.state.setLevel(toSlogLevel(level))
}

// SetOutput sets the destination for this logger's records.
func (l *Log) SetOutput(dst io.Writer) {
	if l.state == nil {
		return
	}

	l.state.setOutput(dst)
}

// EnableJSONOutput selects JSON encoding over the text encoding.
func (l *Log) EnableJSONOutput(enabled bool) {
	if l.state == nil {
		return
	}

	l.state.setJSON(enabled)
}

// SetAppID sets the app_id field.
func (l *Log) SetAppID(id string) {
	if l.state == nil {
		return
	}

	l.state.setAppID(id)
}

// log emits at an arbitrary level without the call-site skipping that slog's
// own helpers apply, which is what Fatal needs.
func (l *Log) log(level slog.Level, msg string, args ...any) {
	l.Log(context.Background(), level, msg, args...)
}

// exit is a variable so tests can observe Fatal without terminating.
var exit = os.Exit

func (l *Log) derive(h *handler) *Log {
	return &Log{
		Logger:  slog.New(h),
		name:    l.name,
		state:   l.state,
		handler: h,
	}
}
