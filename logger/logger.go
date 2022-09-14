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

// contextKey is how we find Loggers in a context.Context.
type contextKey struct{}

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
	emptyOpLogger     = &emptyLogger{}
)

// Logger includes the logging api sets.
type Logger interface {
	// EnableJSONOutput enables JSON formatted output log
	EnableJSONOutput(enabled bool)

	// SetAppID sets dapr_id field in the log. Default value is empty string
	SetAppID(id string)
	// SetOutputLevel sets log output level
	SetOutputLevel(outputLevel LogLevel)

	// WithLogType specify the log_type field in log. Default value is LogTypeLog
	WithLogType(logType string) Logger

	// Info logs a message at level Info.
	Info(args ...interface{})
	// Infof logs a message at level Info.
	Infof(format string, args ...interface{})
	// Debug logs a message at level Debug.
	Debug(args ...interface{})
	// Debugf logs a message at level Debug.
	Debugf(format string, args ...interface{})
	// Warn logs a message at level Warn.
	Warn(args ...interface{})
	// Warnf logs a message at level Warn.
	Warnf(format string, args ...interface{})
	// Error logs a message at level Error.
	Error(args ...interface{})
	// Errorf logs a message at level Error.
	Errorf(format string, args ...interface{})
	// Fatal logs a message at level Fatal then the process will exit with status set to 1.
	Fatal(args ...interface{})
	// Fatalf logs a message at level Fatal then the process will exit with status set to 1.
	Fatalf(format string, args ...interface{})
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

func getLoggers() map[string]Logger {
	globalLoggersLock.RLock()
	defer globalLoggersLock.RUnlock()

	l := map[string]Logger{}
	for k, v := range globalLoggers {
		l[k] = v
	}

	return l
}

// NewContext returns a new Context, derived from ctx, which carries the
// provided Logger.
func NewContext(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContextOrDiscard returns a Logger from ctx.  If no Logger is found, this
// returns a Logger that discards all log messages.
func FromContextOrDefault(ctx context.Context) Logger {
	if v, ok := ctx.Value(contextKey{}).(Logger); ok {
		return v
	}

	return emptyOpLogger
}
