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
	"io"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// daprLogger is the implemention for logrus.
type daprLogger struct {
	// name is the name of logger that is published to log as a scope
	name string
	// loger is the instance of logrus logger
	logger *logrus.Entry
}

var DaprVersion = "unknown"

func newDaprLogger(name string) *daprLogger {
	newLogger := logrus.New()
	newLogger.SetOutput(os.Stdout)

	dl := &daprLogger{
		name: name,
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
		formatter = &logrus.JSONFormatter{ //nolint: exhaustruct
			TimestampFormat: time.RFC3339Nano,
			FieldMap:        fieldMap,
		}
	} else {
		formatter = &logrus.TextFormatter{ //nolint: exhaustruct
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

func toLogrusLevel(lvl LogLevel) logrus.Level {
	// ignore error because it will never happen
	l, _ := logrus.ParseLevel(string(lvl))
	return l
}

// SetOutputLevel sets log output level.
func (l *daprLogger) SetOutputLevel(outputLevel LogLevel) {
	l.logger.Logger.SetLevel(toLogrusLevel(outputLevel))
}

// IsOutputLevelEnabled returns true if the logger will output this LogLevel.
func (l *daprLogger) IsOutputLevelEnabled(level LogLevel) bool {
	return l.logger.Logger.IsLevelEnabled(toLogrusLevel(level))
}

// SetOutput sets the destination for the logs.
func (l *daprLogger) SetOutput(dst io.Writer) {
	l.logger.Logger.SetOutput(dst)
}

// WithLogType specify the log_type field in log. Default value is LogTypeLog.
func (l *daprLogger) WithLogType(logType string) Logger {
	return &daprLogger{
		name:   l.name,
		logger: l.logger.WithField(logFieldType, logType),
	}
}

// WithFields returns a logger with the added structured fields.
func (l *daprLogger) WithFields(fields map[string]any) Logger {
	return &daprLogger{
		name:   l.name,
		logger: l.logger.WithFields(fields),
	}
}

// Info logs a message at level Info.
func (l *daprLogger) Info(args ...any) {
	l.logger.Log(logrus.InfoLevel, args...)
}

// Infof logs a message at level Info.
func (l *daprLogger) Infof(format string, args ...any) {
	l.logger.Logf(logrus.InfoLevel, format, args...)
}

// Debug logs a message at level Debug.
func (l *daprLogger) Debug(args ...any) {
	l.logger.Log(logrus.DebugLevel, args...)
}

// Debugf logs a message at level Debug.
func (l *daprLogger) Debugf(format string, args ...any) {
	l.logger.Logf(logrus.DebugLevel, format, args...)
}

// Warn logs a message at level Warn.
func (l *daprLogger) Warn(args ...any) {
	l.logger.Log(logrus.WarnLevel, args...)
}

// Warnf logs a message at level Warn.
func (l *daprLogger) Warnf(format string, args ...any) {
	l.logger.Logf(logrus.WarnLevel, format, args...)
}

// Error logs a message at level Error.
func (l *daprLogger) Error(args ...any) {
	l.logger.Log(logrus.ErrorLevel, args...)
}

// Errorf logs a message at level Error.
func (l *daprLogger) Errorf(format string, args ...any) {
	l.logger.Logf(logrus.ErrorLevel, format, args...)
}

// Fatal logs a message at level Fatal then the process will exit with status set to 1.
func (l *daprLogger) Fatal(args ...any) {
	l.logger.Fatal(args...)
}

// Fatalf logs a message at level Fatal then the process will exit with status set to 1.
func (l *daprLogger) Fatalf(format string, args ...any) {
	l.logger.Fatalf(format, args...)
}
