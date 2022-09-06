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
	"os"
	"time"

	"github.com/dapr/kit/trace"

	"github.com/sirupsen/logrus"
)

// daprLogger is the implemention for logrus.
type daprLogger struct {
	// name is the name of logger that is published to log as a scope
	name string
	// traceEnabled is the flag to enable trace.
	traceEnabled bool
	// loger is the instance of logrus logger
	logger *logrus.Entry
}

var DaprVersion = "unknown"

func newDaprLogger(name string) *daprLogger {
	newLogger := logrus.New()
	newLogger.SetOutput(os.Stdout)

	dl := &daprLogger{
		name:         name,
		traceEnabled: defaultTraceEnabled,
		logger: newLogger.WithFields(logrus.Fields{
			logFieldScope: name,
			logFieldType:  LogTypeLog,
		}),
	}

	dl.EnableJSONOutput(defaultJSONOutput)

	return dl
}

// EnableJSONOutput enables JSON formatted output log.
func (l *daprLogger) EnableJSONOutput(enabled bool) {
	var formatter logrus.Formatter

	fieldMap := logrus.FieldMap{
		// If time field name is conflicted, logrus adds "fields." prefix.
		// So rename to unused field @time to avoid the confliction.
		logrus.FieldKeyTime:  logFieldTimeStamp,
		logrus.FieldKeyLevel: logFieldLevel,
		logrus.FieldKeyMsg:   logFieldMessage,
	}

	hostname, _ := os.Hostname()
	l.logger.Data = logrus.Fields{
		logFieldScope:    l.logger.Data[logFieldScope],
		logFieldType:     LogTypeLog,
		logFieldInstance: hostname,
		logFieldDaprVer:  DaprVersion,
	}

	if enabled {
		formatter = &logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
			FieldMap:        fieldMap,
		}
	} else {
		formatter = &logrus.TextFormatter{
			TimestampFormat: time.RFC3339Nano,
			FieldMap:        fieldMap,
		}
	}

	l.logger.Logger.SetFormatter(formatter)
}

// SetAppID sets app_id field in the log. Default value is empty string.
func (l *daprLogger) SetAppID(id string) {
	l.logger = l.logger.WithField(logFieldAppID, id)
}

// SetTraceEnabled sets trace enabled for dapr logger.
func (l *daprLogger) SetTraceEnabled(enabled bool) {
	l.traceEnabled = enabled
}

func toLogrusLevel(lvl LogLevel) logrus.Level {
	// ignore error because it will never happens
	l, _ := logrus.ParseLevel(string(lvl))
	return l
}

// SetOutputLevel sets log output level.
func (l *daprLogger) SetOutputLevel(outputLevel LogLevel) {
	l.logger.Logger.SetLevel(toLogrusLevel(outputLevel))
}

// WithLogType specify the log_type field in log. Default value is LogTypeLog.
func (l *daprLogger) WithLogType(logType string) Logger {
	return &daprLogger{
		name:         l.name,
		traceEnabled: defaultTraceEnabled,
		logger:       l.logger.WithField(logFieldType, logType),
	}
}

// Info logs a message at level Info.
func (l *daprLogger) Info(args ...interface{}) {
	l.print(nil, logrus.InfoLevel, args...)
}

// Infof logs a message at level Info.
func (l *daprLogger) Infof(format string, args ...interface{}) {
	l.printf(nil, logrus.InfoLevel, format, args...)
}

// InfoWithContext logs a message and context (traceid...)at level Info.
func (l *daprLogger) InfoWithContext(ctx context.Context, args ...interface{}) {
	l.print(ctx, logrus.InfoLevel, args...)
}

// InfoWithContextf logs a message and context (traceid...)at level Info.
func (l *daprLogger) InfoWithContextf(ctx context.Context, format string, args ...interface{}) {
	l.printf(ctx, logrus.InfoLevel, format, args...)
}

// Debug logs a message at level Debug.
func (l *daprLogger) Debug(args ...interface{}) {
	l.print(nil, logrus.DebugLevel, args...)
}

// Debugf logs a message at level Debug.
func (l *daprLogger) Debugf(format string, args ...interface{}) {
	l.printf(nil, logrus.DebugLevel, format, args...)
}

// DebugWithContext logs a message and context (traceid...) at level Debug.
func (l *daprLogger) DebugWithContext(ctx context.Context, args ...interface{}) {
	l.print(ctx, logrus.DebugLevel, args...)
}

// DebugWithContext logs a message and context (traceid...) at level Debug.
func (l *daprLogger) DebugWithContextf(ctx context.Context, format string, args ...interface{}) {
	l.printf(ctx, logrus.DebugLevel, format, args...)
}

// Warn logs a message at level Warn.
func (l *daprLogger) Warn(args ...interface{}) {
	l.print(nil, logrus.WarnLevel, args...)
}

// Warnf logs a message at level Warn.
func (l *daprLogger) Warnf(format string, args ...interface{}) {
	l.printf(nil, logrus.WarnLevel, format, args...)
}

// WarnWithContext logs a message and context (tarceid...) at level Warn.
func (l *daprLogger) WarnWithContext(ctx context.Context, args ...interface{}) {
	l.print(ctx, logrus.WarnLevel, args...)
}

// WarnWithContext logs a message and context (tarceid...) at level Warn.
func (l *daprLogger) WarnWithContextf(ctx context.Context, format string, args ...interface{}) {
	l.printf(ctx, logrus.WarnLevel, format, args...)
}

// Error logs a message at level Error.
func (l *daprLogger) Error(args ...interface{}) {
	l.print(nil, logrus.ErrorLevel, args...)
}

// Errorf logs a message at level Error.
func (l *daprLogger) Errorf(format string, args ...interface{}) {
	l.printf(nil, logrus.ErrorLevel, format, args...)
}

// ErrorWithContext logs a message and context (traceid...) at level Error.
func (l *daprLogger) ErrorWithContext(ctx context.Context, args ...interface{}) {
	l.print(ctx, logrus.ErrorLevel, args...)
}

// ErrorWithContext logs a message and context (traceid...) at level Error.
func (l *daprLogger) ErrorWithContextf(ctx context.Context, format string, args ...interface{}) {
	l.printf(ctx, logrus.ErrorLevel, format, args...)
}

// Fatal logs a message at level Fatal then the process will exit with status set to 1.
func (l *daprLogger) Fatal(args ...interface{}) {
	l.print(nil, logrus.FatalLevel, args...)
}

// Fatalf logs a message at level Fatal then the process will exit with status set to 1.
func (l *daprLogger) Fatalf(format string, args ...interface{}) {
	l.printf(nil, logrus.FatalLevel, format, args...)
}

// FatalWithContext logs a message and context (traceid...) at level Fatal then the process will exit with status set to 1.
func (l *daprLogger) FatalWithContext(ctx context.Context, format string, args ...interface{}) {
	l.print(ctx, logrus.FatalLevel, args...)
}

// FatalWithContext logs a message and context (traceid...) at level Fatal then the process will exit with status set to 1.
func (l *daprLogger) FatalWithContextf(ctx context.Context, format string, args ...interface{}) {
	l.printf(ctx, logrus.FatalLevel, format, args...)
}

func (l *daprLogger) print(ctx context.Context, level logrus.Level, args ...interface{}) {
	var id string
	if l.traceEnabled {
		id = trace.TraceID(ctx)
	}

	if id != "" {
		l.logger.WithField("id", id).Log(level, args...)
	} else {
		l.logger.Log(level, args...)
	}
}

func (l *daprLogger) printf(ctx context.Context, level logrus.Level, format string, args ...interface{}) {
	var id string
	if l.traceEnabled {
		id = trace.TraceID(ctx)
	}

	if id != "" {
		l.logger.WithField("id", id).Logf(level, format, args...)
	} else {
		l.logger.Logf(level, format, args...)
	}
}
