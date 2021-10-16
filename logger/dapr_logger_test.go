package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.opencensus.io/trace/propagation"
	"google.golang.org/grpc/metadata"

	"github.com/dapr/kit/trace"
)

const fakeLoggerName = "fakeLogger"

func getTestLogger(buf io.Writer) *daprLogger {
	l := newDaprLogger(fakeLoggerName)
	l.logger.Logger.SetOutput(buf)

	return l
}

func TestEnableJSON(t *testing.T) {
	var buf bytes.Buffer
	testLogger := getTestLogger(&buf)

	expectedHost, _ := os.Hostname()
	testLogger.EnableJSONOutput(true)
	_, okJSON := testLogger.logger.Logger.Formatter.(*logrus.JSONFormatter)
	assert.True(t, okJSON)
	assert.Equal(t, "fakeLogger", testLogger.logger.Data[logFieldScope])
	assert.Equal(t, LogTypeLog, testLogger.logger.Data[logFieldType])
	assert.Equal(t, expectedHost, testLogger.logger.Data[logFieldInstance])

	testLogger.EnableJSONOutput(false)
	_, okText := testLogger.logger.Logger.Formatter.(*logrus.TextFormatter)
	assert.True(t, okText)
	assert.Equal(t, "fakeLogger", testLogger.logger.Data[logFieldScope])
	assert.Equal(t, LogTypeLog, testLogger.logger.Data[logFieldType])
	assert.Equal(t, expectedHost, testLogger.logger.Data[logFieldInstance])
}

func TestJSONLoggerFields(t *testing.T) {
	tests := []struct {
		name        string
		outputLevel LogLevel
		level       string
		appID       string
		message     string
		instance    string
		fn          func(*daprLogger, string)
	}{
		{
			"info()",
			InfoLevel,
			"info",
			"dapr_app",
			"King Dapr",
			"dapr-pod",
			func(l *daprLogger, msg string) {
				l.Info(msg)
			},
		},
		{
			"infof()",
			InfoLevel,
			"info",
			"dapr_app",
			"King Dapr",
			"dapr-pod",
			func(l *daprLogger, msg string) {
				l.Infof("%s", msg)
			},
		},
		{
			"debug()",
			DebugLevel,
			"debug",
			"dapr_app",
			"King Dapr",
			"dapr-pod",
			func(l *daprLogger, msg string) {
				l.Debug(msg)
			},
		},
		{
			"debugf()",
			DebugLevel,
			"debug",
			"dapr_app",
			"King Dapr",
			"dapr-pod",
			func(l *daprLogger, msg string) {
				l.Debugf("%s", msg)
			},
		},
		{
			"error()",
			InfoLevel,
			"error",
			"dapr_app",
			"King Dapr",
			"dapr-pod",
			func(l *daprLogger, msg string) {
				l.Error(msg)
			},
		},
		{
			"errorf()",
			InfoLevel,
			"error",
			"dapr_app",
			"King Dapr",
			"dapr-pod",
			func(l *daprLogger, msg string) {
				l.Errorf("%s", msg)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			testLogger := getTestLogger(&buf)
			testLogger.EnableJSONOutput(true)
			testLogger.SetAppID(tt.appID)
			DaprVersion = tt.appID
			testLogger.SetOutputLevel(tt.outputLevel)
			testLogger.logger.Data[logFieldInstance] = tt.instance

			tt.fn(testLogger, tt.message)

			b, _ := buf.ReadBytes('\n')
			var o map[string]interface{}
			assert.NoError(t, json.Unmarshal(b, &o))

			// assert
			assert.Equal(t, tt.appID, o[logFieldAppID])
			assert.Equal(t, tt.instance, o[logFieldInstance])
			assert.Equal(t, tt.level, o[logFieldLevel])
			assert.Equal(t, LogTypeLog, o[logFieldType])
			assert.Equal(t, fakeLoggerName, o[logFieldScope])
			assert.Equal(t, tt.message, o[logFieldMessage])
			_, err := time.Parse(time.RFC3339, o[logFieldTimeStamp].(string))
			assert.NoError(t, err)
		})
	}
}

func TestWithTypeFields(t *testing.T) {
	var buf bytes.Buffer
	testLogger := getTestLogger(&buf)
	testLogger.EnableJSONOutput(true)
	testLogger.SetAppID("dapr_app")
	testLogger.SetOutputLevel(InfoLevel)

	// WithLogType will return new Logger with request log type
	// Meanwhile, testLogger uses the default logtype
	loggerWithRequestType := testLogger.WithLogType(LogTypeRequest)
	loggerWithRequestType.Info("call user app")

	b, _ := buf.ReadBytes('\n')
	var o map[string]interface{}
	assert.NoError(t, json.Unmarshal(b, &o))

	assert.Equalf(t, LogTypeRequest, o[logFieldType], "new logger must be %s type", LogTypeRequest)

	// Log our via testLogger to ensure that testLogger still uses the default logtype
	testLogger.Info("testLogger with log LogType")

	b, _ = buf.ReadBytes('\n')
	assert.NoError(t, json.Unmarshal(b, &o))

	assert.Equalf(t, LogTypeLog, o[logFieldType], "testLogger must be %s type", LogTypeLog)
}

func TestWithTrace(t *testing.T) {
	t.Run("dapr log traceid output", func(t *testing.T) {
		var buf bytes.Buffer
		id := "dd6a7c5a06b14f8aa02fdecb4f4ed480"
		testLogger := getTestLogger(&buf)
		testLogger.EnableJSONOutput(true)
		log := testLogger.WithTrace(id)
		log.Info("log output")
		b, _ := buf.ReadBytes('\n')
		assert.Contains(t, string(b), id, "output log contains trace id")
	})
}

func TestWithTraceFromContext(t *testing.T) {
	testTraceParent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	testSpanContext, _ := trace.SpanContextFromW3CString(testTraceParent)
	testTraceBinary := propagation.Binary(testSpanContext)
	ctx := context.Background()
	t.Run("dapr log traceid output from incoming context", func(t *testing.T) {
		var buf bytes.Buffer
		traceid := "4bf92f3577b34da6a3ce929d0e0e4736"
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("grpc-trace-bin", string(testTraceBinary)))
		testLogger := getTestLogger(&buf)
		testLogger.EnableJSONOutput(true)
		log := testLogger.WithContext(ctx)
		log.Info("log output from incoming")
		b, _ := buf.ReadBytes('\n')
		assert.Contains(t, string(b), traceid, "output log contains trace id")
	})
	t.Run("dapr log traceid output from outcoming context", func(t *testing.T) {
		var buf bytes.Buffer
		traceid := "4bf92f3577b34da6a3ce929d0e0e4736"
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("grpc-trace-bin", string(testTraceBinary)))
		testLogger := getTestLogger(&buf)
		testLogger.EnableJSONOutput(true)
		log := testLogger.WithContext(ctx)
		log.Info("log output from outcoming")
		b, _ := buf.ReadBytes('\n')
		assert.Contains(t, string(b), traceid, "output log contains trace id")
	})
}

func TestToLogrusLevel(t *testing.T) {
	t.Run("Dapr DebugLevel to Logrus.DebugLevel", func(t *testing.T) {
		assert.Equal(t, logrus.DebugLevel, toLogrusLevel(DebugLevel))
	})

	t.Run("Dapr InfoLevel to Logrus.InfoLevel", func(t *testing.T) {
		assert.Equal(t, logrus.InfoLevel, toLogrusLevel(InfoLevel))
	})

	t.Run("Dapr WarnLevel to Logrus.WarnLevel", func(t *testing.T) {
		assert.Equal(t, logrus.WarnLevel, toLogrusLevel(WarnLevel))
	})

	t.Run("Dapr ErrorLevel to Logrus.ErrorLevel", func(t *testing.T) {
		assert.Equal(t, logrus.ErrorLevel, toLogrusLevel(ErrorLevel))
	})

	t.Run("Dapr FatalLevel to Logrus.FatalLevel", func(t *testing.T) {
		assert.Equal(t, logrus.FatalLevel, toLogrusLevel(FatalLevel))
	})
}
