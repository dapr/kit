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
	"io"
	"strings"
	"sync"
)

const (
	// LogTypeLog is normal log type.
	LogTypeLog = "log"
	// LogTypeRequest is Request log type.
	LogTypeRequest = "request"

	// Field names that defines Dapr log schema.
	logFieldTimeStamp = "time"
	logFieldLevel     = "level"
	logFieldType      = "type"
	logFieldScope     = "scope"
	logFieldMessage   = "msg"
	logFieldInstance  = "instance"
	logFieldDaprVer   = "ver"
	logFieldAppID     = "app_id"
)

type logContextKeyType struct{}

// logContextKey is how we find Loggers in a context.Context
var logContextKey = logContextKeyType{}

// LogLevel is Dapr Logger Level type.
type LogLevel string

const (
	// DebugLevel has verbose message.
	DebugLevel LogLevel = "debug"
	// InfoLevel is default log level.
	InfoLevel LogLevel = "info"
	// WarnLevel is for logging messages about possible issues.
	WarnLevel LogLevel = "warn"
	// ErrorLevel is for logging errors.
	ErrorLevel LogLevel = "error"
	// FatalLevel is for logging fatal messages. The system shuts down after logging the message.
	FatalLevel LogLevel = "fatal"

	// UndefinedLevel is for undefined log level.
	UndefinedLevel LogLevel = "undefined"
)

// globalLoggers is the collection of Dapr Logger that is shared globally.
// TODO: User will disable or enable logger on demand.
var (
	globalLoggers     = map[string]Logger{}
	globalLoggersLock = sync.RWMutex{}
	defaultOpLogger   = &nopLogger{}
)

// globalStates holds one configuration per logger name, shared between the
// [Logger] and the [Log] of that name so that configuring either configures
// both. Each name gets its own state, matching the previous implementation
// where every named logger owned a separate logrus.Logger.
var (
	globalStates     = map[string]*state{}
	globalStatesLock = sync.Mutex{}
)

// sharedState returns the configuration for name, creating it if necessary.
func sharedState(name string) *state {
	globalStatesLock.Lock()
	defer globalStatesLock.Unlock()

	s, ok := globalStates[name]
	if !ok {
		s = newState()
		globalStates[name] = s
	}

	return s
}

// getStates returns a snapshot of every registered logger's configuration.
func getStates() []*state {
	globalStatesLock.Lock()
	defer globalStatesLock.Unlock()

	out := make([]*state, 0, len(globalStates))
	for _, s := range globalStates {
		out = append(out, s)
	}

	return out
}

// Logger includes the logging api sets.
//
// Prefer [Log] and [New] for new code, which take structured attributes rather
// than printf format strings:
//
//	log := logger.New("dapr.runtime")
//	log.Info("component loaded", "component", name)
//	log.Error("failed to load component", logger.Err(err))
//
// This interface is not itself deprecated: it remains the parameter type of
// public constructors across the Dapr ecosystem, notably every
// components-contrib component, and third-party code satisfies it
// structurally. Use [FromLogger] to get a [Log] from one.
//
// Its printf-style logging methods are deprecated. They build the message
// eagerly, box their arguments before the level is checked, and produce output
// that cannot be filtered on by field. They will be removed in a future major
// release.
type Logger interface { //nolint: interfacebloat
	// EnableJSONOutput enables JSON formatted output log
	EnableJSONOutput(enabled bool)

	// SetAppID sets dapr_id field in the log. Default value is empty string
	SetAppID(id string)

	// SetOutputLevel sets the log output level
	SetOutputLevel(outputLevel LogLevel)
	// SetOutput sets the destination for the logs
	SetOutput(dst io.Writer)

	// IsOutputLevelEnabled returns true if the logger will output this LogLevel.
	IsOutputLevelEnabled(level LogLevel) bool

	// WithLogType specifies the log_type field in log. Default value is LogTypeLog
	WithLogType(logType string) Logger

	// WithFields returns a logger with the added structured fields.
	WithFields(fields map[string]any) Logger

	// Info logs a message at level Info.
	//
	// Deprecated: use [Log.Info] with structured attributes.
	Info(args ...any)
	// Infof logs a message at level Info.
	//
	// Deprecated: use [Log.Info] with structured attributes.
	Infof(format string, args ...any)
	// Debug logs a message at level Debug.
	//
	// Deprecated: use [Log.Debug] with structured attributes.
	Debug(args ...any)
	// Debugf logs a message at level Debug.
	//
	// Deprecated: use [Log.Debug] with structured attributes.
	Debugf(format string, args ...any)
	// Warn logs a message at level Warn.
	//
	// Deprecated: use [Log.Warn] with structured attributes.
	Warn(args ...any)
	// Warnf logs a message at level Warn.
	//
	// Deprecated: use [Log.Warn] with structured attributes.
	Warnf(format string, args ...any)
	// Error logs a message at level Error.
	//
	// Deprecated: use [Log.Error] with structured attributes.
	Error(args ...any)
	// Errorf logs a message at level Error.
	//
	// Deprecated: use [Log.Error] with structured attributes.
	Errorf(format string, args ...any)
	// Fatal logs a message at level Fatal then the process will exit with status set to 1.
	//
	// Deprecated: use [Log.Fatal] with structured attributes.
	Fatal(args ...any)
	// Fatalf logs a message at level Fatal then the process will exit with status set to 1.
	//
	// Deprecated: use [Log.Fatal] with structured attributes.
	Fatalf(format string, args ...any)
}

// toLogLevel converts to LogLevel.
func toLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn":
		return WarnLevel
	case "error":
		return ErrorLevel
	case "fatal":
		return FatalLevel
	}

	// unsupported log level by Dapr
	return UndefinedLevel
}

// NewLogger creates new Logger instance.
//
// Prefer [New], which returns a [Log] taking structured attributes. This
// constructor is retained, and not deprecated, because a [Logger] is still
// required wherever it is a parameter type of a public API. It shares its
// configuration with the [Log] of the same name.
func NewLogger(name string) Logger {
	globalLoggersLock.Lock()
	defer globalLoggersLock.Unlock()

	logger, ok := globalLoggers[name]
	if !ok {
		logger = newDaprLogger(name)
		globalLoggers[name] = logger
	}

	return logger
}

// NewContext returns a new Context, derived from ctx, which carries the
// provided Logger.
func NewContext(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, logContextKey, logger)
}

// FromContextOrDefault returns a Logger from ctx.  If no Logger is found, this
// returns a Logger that discards all log messages.
func FromContextOrDefault(ctx context.Context) Logger {
	if v, ok := ctx.Value(logContextKey).(Logger); ok {
		return v
	}

	return defaultOpLogger
}
