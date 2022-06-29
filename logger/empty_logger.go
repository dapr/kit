package logger

type emptyLogger struct{}

// EnableJSONOutput enables JSON formatted output log
func (e *emptyLogger) EnableJSONOutput(enabled bool) {}

// SetAppID sets dapr_id field in the log. Default value is empty string
func (e *emptyLogger) SetAppID(id string) {}

// SetOutputLevel sets log output level
func (e *emptyLogger) SetOutputLevel(outputLevel LogLevel) {}

// WithLogType specify the log_type field in log. Default value is LogTypeLog
func (e *emptyLogger) WithLogType(logType string) Logger {
	return nil
}

// Info logs a message at level Info.
func (e *emptyLogger) Info(args ...interface{}) {}

// Infof logs a message at level Info.
func (e *emptyLogger) Infof(format string, args ...interface{}) {}

// Debug logs a message at level Debug.
func (e *emptyLogger) Debug(args ...interface{}) {}

// Debugf logs a message at level Debug.
func (e *emptyLogger) Debugf(format string, args ...interface{}) {}

// Warn logs a message at level Warn.
func (e *emptyLogger) Warn(args ...interface{}) {}

// Warnf logs a message at level Warn.
func (e *emptyLogger) Warnf(format string, args ...interface{}) {}

// Error logs a message at level Error.
func (e *emptyLogger) Error(args ...interface{}) {}

// Errorf logs a message at level Error.
func (e *emptyLogger) Errorf(format string, args ...interface{}) {}

// Fatal logs a message at level Fatal then the process will exit with status set to 1.
func (e *emptyLogger) Fatal(args ...interface{}) {}

// Fatalf logs a message at level Fatal then the process will exit with status set to 1.
func (e *emptyLogger) Fatalf(format string, args ...interface{}) {}
