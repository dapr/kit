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

import "log/slog"

// Dapr log levels expressed as slog levels.
//
// LevelFatal sits above slog.LevelError so that a fatal record is emitted by
// any handler that would emit an error. LevelUndefined is above everything and
// is never used for a record, so selecting it as the output level silences all
// output, matching the behaviour of the previous logrus implementation.
const (
	LevelDebug = slog.LevelDebug // -4
	LevelInfo  = slog.LevelInfo  // 0
	LevelWarn  = slog.LevelWarn  // 4
	LevelError = slog.LevelError // 8

	LevelFatal     = slog.Level(12)
	LevelUndefined = slog.Level(16)
)

// levelString renders a level using the names the previous logrus-backed
// implementation emitted. Note "warning" rather than slog's "WARN": the label
// is part of the documented log schema and is matched by user log pipelines.
func levelString(l slog.Level) string {
	switch {
	case l < LevelInfo:
		return "debug"
	case l < LevelWarn:
		return "info"
	case l < LevelError:
		return "warning"
	case l < LevelFatal:
		return "error"
	default:
		return "fatal"
	}
}

// toSlogLevel converts a Dapr LogLevel to its slog equivalent.
func toSlogLevel(lvl LogLevel) slog.Level {
	switch lvl {
	case DebugLevel:
		return LevelDebug
	case InfoLevel:
		return LevelInfo
	case WarnLevel:
		return LevelWarn
	case ErrorLevel:
		return LevelError
	case FatalLevel:
		return LevelFatal
	case UndefinedLevel:
		return LevelUndefined
	default:
		return LevelUndefined
	}
}
