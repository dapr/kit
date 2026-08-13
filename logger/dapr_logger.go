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
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
)

// DaprVersion is the version reported in the ver field. It is set at link time
// via -X github.com/dapr/kit/logger.DaprVersion=<version>.
var DaprVersion = "unknown"

// daprLogger implements the deprecated printf-style [Logger] interface on top
// of slog.
//
// It exists so that the ~1700 call sites across dapr and components-contrib,
// and the third-party interfaces Dapr satisfies structurally (durabletask-go's
// backend.Logger, dubbo-go's logger.Logger), keep working unchanged while call
// sites migrate to [Log].
type daprLogger struct {
	name    string
	state   *state
	handler *handler
	log     *slog.Logger
}

func newDaprLogger(name string) *daprLogger {
	return newDaprLoggerState(name, sharedState(name))
}

func newDaprLoggerState(name string, s *state) *daprLogger {
	h := newHandler(s, name)

	return &daprLogger{
		name:    name,
		state:   s,
		handler: h,
		log:     slog.New(h),
	}
}

// EnableJSONOutput enables JSON formatted output log.
func (l *daprLogger) EnableJSONOutput(enabled bool) {
	l.state.setJSON(enabled)
}

// SetAppID sets app_id field in the log. Default value is empty string.
func (l *daprLogger) SetAppID(id string) {
	l.state.setAppID(id)
}

// SetOutputLevel sets log output level.
func (l *daprLogger) SetOutputLevel(outputLevel LogLevel) {
	l.state.setLevel(toSlogLevel(outputLevel))
}

// IsOutputLevelEnabled returns true if the logger will output this LogLevel.
func (l *daprLogger) IsOutputLevelEnabled(level LogLevel) bool {
	return l.state.enabled(toSlogLevel(level))
}

// SetOutput sets the destination for the logs.
func (l *daprLogger) SetOutput(dst io.Writer) {
	l.state.setOutput(dst)
}

// WithLogType specify the log_type field in log. Default value is LogTypeLog.
func (l *daprLogger) WithLogType(logType string) Logger {
	return l.derive(l.handler.withLogType(logType))
}

// WithFields returns a logger with the added structured fields.
func (l *daprLogger) WithFields(fields map[string]any) Logger {
	if len(fields) == 0 {
		return l
	}

	// Sort so that repeated calls with the same map produce a stable handler;
	// the encoder sorts by key anyway, but this keeps derived handlers
	// deterministic and comparable in tests.
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	attrs := make([]slog.Attr, 0, len(keys))
	for _, k := range keys {
		attrs = append(attrs, slog.Any(k, fields[k]))
	}

	h, _ := l.handler.WithAttrs(attrs).(*handler)

	return l.derive(h)
}

// Structured returns this logger as a [Log], for call sites that have migrated
// to structured attributes but receive a [Logger].
func (l *daprLogger) Structured() *Log {
	return &Log{
		Logger:  l.log,
		name:    l.name,
		state:   l.state,
		handler: l.handler,
	}
}

// Info logs a message at level Info.
func (l *daprLogger) Info(args ...any) { l.emitSprint(LevelInfo, args...) }

// Infof logs a message at level Info.
func (l *daprLogger) Infof(format string, args ...any) { l.emitf(LevelInfo, format, args...) }

// Debug logs a message at level Debug.
func (l *daprLogger) Debug(args ...any) { l.emitSprint(LevelDebug, args...) }

// Debugf logs a message at level Debug.
func (l *daprLogger) Debugf(format string, args ...any) { l.emitf(LevelDebug, format, args...) }

// Warn logs a message at level Warn.
func (l *daprLogger) Warn(args ...any) { l.emitSprint(LevelWarn, args...) }

// Warnf logs a message at level Warn.
func (l *daprLogger) Warnf(format string, args ...any) { l.emitf(LevelWarn, format, args...) }

// Error logs a message at level Error.
func (l *daprLogger) Error(args ...any) { l.emitSprint(LevelError, args...) }

// Errorf logs a message at level Error.
func (l *daprLogger) Errorf(format string, args ...any) { l.emitf(LevelError, format, args...) }

// Fatal logs a message at level Fatal then the process will exit with status set to 1.
func (l *daprLogger) Fatal(args ...any) {
	l.emitSprint(LevelFatal, args...)
	exit(1)
}

// Fatalf logs a message at level Fatal then the process will exit with status set to 1.
func (l *daprLogger) Fatalf(format string, args ...any) {
	l.emitf(LevelFatal, format, args...)
	exit(1)
}

func (l *daprLogger) derive(h *handler) *daprLogger {
	return &daprLogger{
		name:    l.name,
		state:   l.state,
		handler: h,
		log:     slog.New(h),
	}
}

func (l *daprLogger) emitSprint(level slog.Level, args ...any) {
	if !l.state.enabled(level) {
		return
	}

	l.log.Log(context.Background(), level, fmtSprint(args...))
}

func (l *daprLogger) emitf(level slog.Level, format string, args ...any) {
	if !l.state.enabled(level) {
		return
	}

	l.log.Log(context.Background(), level, fmt.Sprintf(format, args...))
}
