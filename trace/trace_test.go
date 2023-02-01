package trace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
)

func TestID(t *testing.T) {
	ctx := context.Background()
	assert.Emptyf(t, ID(ctx), "traceid is empty")

	sc := GenerateSpanContext()
	ctx = trace.ContextWithSpanContext(ctx, sc)
	assert.NotEmptyf(t, ID(ctx), "traceid is not empty")
}

func TestNewSpanContextFromTrace(t *testing.T) {
	sc1 := GenerateSpanContext()

	sc2 := NewSpanContextFromTrace(Traceparent(sc1), sc1.TraceState().String())
	assert.Equal(t, sc1, sc2)
}

func TestSpanContextFromW3CString(t *testing.T) {
	uts := []struct {
		traceparent string
		expected    bool
		desc        string
	}{
		{
			"0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
			false,
			"traceparent error with only 3 fields(-)",
		},
		{
			"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
			true,
			"traceparent is valid",
		},
		{
			"00-0af7651916cd43dd8448eb211c80319c-b7ad6b716920333-01",
			false,
			"spanid error with len=15",
		},
		{
			"00-0af7651916cd43dd8448eb211c80319-b7ad6b7169203331-01",
			false,
			"traceparent error with len=35",
		},
		{
			"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-0",
			false,
			"traceflag error with len=1",
		},
	}
	for _, ut := range uts {
		sc, ok := SpanContextFromW3CString(ut.traceparent)
		assert.Equalf(t, ut.expected, sc.IsValid(), ut.desc)
		assert.Equalf(t, ut.expected, ok, ut.desc)
	}
}

func TestStateFromW3CString(t *testing.T) {
	uts := []struct {
		tracestate  string
		expectedLen int
		desc        string
	}{
		{
			"congo=ucfJifl5GOE,rojo=00f067aa0ba902b7",
			2,
			"tracestate is valid",
		},
		{
			"invalid$@#=invalid",
			0,
			"tracestate is empty",
		},
	}
	for _, ut := range uts {
		ts := StateFromW3CString(ut.tracestate)
		assert.Equalf(t, ut.expectedLen, ts.Len(), ut.desc)
	}
}
