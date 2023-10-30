/*
Copyright 2022 The Dapr Authors
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
)

type nopLogger struct{}

// EnableJSONOutput enables JSON formatted output log.
func (n *nopLogger) EnableJSONOutput(_ bool) {}

// SetAppID sets dapr_id field in the log. nopLogger value is empty string.
func (n *nopLogger) SetAppID(_ string) {}

// SetOutputLevel sets log output level.
func (n *nopLogger) SetOutputLevel(_ LogLevel) {}

// SetOutput sets the destination for the logs
func (n *nopLogger) SetOutput(_ io.Writer) {}

// IsOutputLevelEnabled returns true if the logger will output this LogLevel.
func (n *nopLogger) IsOutputLevelEnabled(_ LogLevel) bool { return true }

// WithLogType specify the log_type field in log. nopLogger value is LogTypeLog.
func (n *nopLogger) WithLogType(_ string) Logger {
	return n
}

// WithFields returns a logger with the added structured fields.
func (n *nopLogger) WithFields(_ map[string]any) Logger {
	return n
}

// Info logs a message at level Info.
func (n *nopLogger) Info(_ ...interface{}) {}

// Infof logs a message at level Info.
func (n *nopLogger) Infof(_ string, _ ...interface{}) {}

// Debug logs a message at level Debug.
func (n *nopLogger) Debug(_ ...interface{}) {}

// Debugf logs a message at level Debug.
func (n *nopLogger) Debugf(_ string, _ ...interface{}) {}

// Warn logs a message at level Warn.
func (n *nopLogger) Warn(_ ...interface{}) {}

// Warnf logs a message at level Warn.
func (n *nopLogger) Warnf(_ string, _ ...interface{}) {}

// Error logs a message at level Error.
func (n *nopLogger) Error(_ ...interface{}) {}

// Errorf logs a message at level Error.
func (n *nopLogger) Errorf(_ string, _ ...interface{}) {}

// Fatal logs a message at level Fatal then the process will exit with status set to 1.
func (n *nopLogger) Fatal(_ ...interface{}) {}

// Fatalf logs a message at level Fatal then the process will exit with status set to 1.
func (n *nopLogger) Fatalf(_ string, _ ...interface{}) {}
