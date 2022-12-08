package logger

import "io"

type defaultLogger struct{}

// EnableJSONOutput enables JSON formatted output log.
func (d *defaultLogger) EnableJSONOutput(dnabled bool) {}

// SetAppID sets dapr_id field in the log. defaultLogger value is empty string.
func (d *defaultLogger) SetAppID(id string) {}

// SetOutputLevel sets log output level.
func (d *defaultLogger) SetOutputLevel(outputLevel LogLevel) {}

// SetOutput sets the destination for the logs
func (d *defaultLogger) SetOutput(dst io.Writer) {}

// IsOutputLevelEnabled returns true if the logger will output this LogLevel.
func (d *defaultLogger) IsOutputLevelEnabled(level LogLevel) bool { return true }

// WithLogType specify the log_type field in log. defaultLogger value is LogTypeLog.
func (d *defaultLogger) WithLogType(logType string) Logger {
	return nil
}

// WithFields returns a logger with the added structured fields.
func (d *defaultLogger) WithFields(fields map[string]any) Logger {
	return nil
}

// Info logs a message at level Info.
func (d *defaultLogger) Info(args ...interface{}) {}

// Infof logs a message at level Info.
func (d *defaultLogger) Infof(format string, args ...interface{}) {}

// Debug logs a message at level Debug.
func (d *defaultLogger) Debug(args ...interface{}) {}

// Debugf logs a message at level Debug.
func (d *defaultLogger) Debugf(format string, args ...interface{}) {}

// Warn logs a message at level Warn.
func (d *defaultLogger) Warn(args ...interface{}) {}

// Warnf logs a message at level Warn.
func (d *defaultLogger) Warnf(format string, args ...interface{}) {}

// Error logs a message at level Error.
func (d *defaultLogger) Error(args ...interface{}) {}

// Errorf logs a message at level Error.
func (d *defaultLogger) Errorf(format string, args ...interface{}) {}

// Fatal logs a message at level Fatal then the process will exit with status set to 1.
func (d *defaultLogger) Fatal(args ...interface{}) {}

// Fatalf logs a message at level Fatal then the process will exit with status set to 1.
func (d *defaultLogger) Fatalf(format string, args ...interface{}) {}
