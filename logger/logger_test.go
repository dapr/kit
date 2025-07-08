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
	"testing"

	"github.com/stretchr/testify/assert"
)

func clearLoggers() {
	globalLoggers = map[string]Logger{}
}

func TestNewLogger(t *testing.T) {
	testLoggerName := "dapr.test"

	t.Run("create new logger instance", func(t *testing.T) {
		clearLoggers()

		// act
		NewLogger(testLoggerName)
		_, ok := globalLoggers[testLoggerName]

		// assert
		assert.True(t, ok)
	})

	t.Run("return the existing logger instance", func(t *testing.T) {
		clearLoggers()

		// act
		oldLogger := NewLogger(testLoggerName)
		newLogger := NewLogger(testLoggerName)

		// assert
		assert.Equal(t, oldLogger, newLogger)
	})
}

func TestToLogLevel(t *testing.T) {
	t.Run("convert debug to DebugLevel", func(t *testing.T) {
		assert.Equal(t, DebugLevel, toLogLevel("debug"))
	})

	t.Run("convert info to InfoLevel", func(t *testing.T) {
		assert.Equal(t, InfoLevel, toLogLevel("info"))
	})

	t.Run("convert warn to WarnLevel", func(t *testing.T) {
		assert.Equal(t, WarnLevel, toLogLevel("warn"))
	})

	t.Run("convert error to ErrorLevel", func(t *testing.T) {
		assert.Equal(t, ErrorLevel, toLogLevel("error"))
	})

	t.Run("convert fatal to FatalLevel", func(t *testing.T) {
		assert.Equal(t, FatalLevel, toLogLevel("fatal"))
	})

	t.Run("undefined loglevel", func(t *testing.T) {
		assert.Equal(t, UndefinedLevel, toLogLevel("undefined"))
	})
}

func TestNewContext(t *testing.T) {
	t.Run("input nil logger", func(t *testing.T) {
		ctx := NewContext(t.Context(), nil)
		assert.NotNil(t, ctx, "ctx is not nil")

		logger := FromContextOrDefault(ctx)
		assert.NotNil(t, logger, "logger is not nil")
		assert.Equal(t, logger, defaultOpLogger)
	})

	t.Run("input non-nil logger", func(t *testing.T) {
		testLoggerName := "dapr.test"
		logger := NewLogger(testLoggerName)
		assert.NotNil(t, logger)

		ctx := NewContext(t.Context(), logger)
		assert.NotNil(t, ctx, "ctx is not nil")
		logger2 := FromContextOrDefault(ctx)
		assert.NotNil(t, logger2)
		assert.Equal(t, logger2, logger)
		assert.NotEqual(t, logger2, defaultOpLogger)
	})
}
