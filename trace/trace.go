package trace

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

const (
	maxVersion       = 254
	supportedVersion = 0

	TraceparentHeader = "traceparent"
	TracestateHeader  = "tracestate"
)

// Traceparent gets traceparent from spancontext.
func Traceparent(sc trace.SpanContext) string {
	flags := sc.TraceFlags() & trace.FlagsSampled

	return fmt.Sprintf("%.2x-%s-%s-%s",
		supportedVersion,
		sc.TraceID(),
		sc.SpanID(),
		flags)
}

// ID gets traceid from context.
func ID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}

// NewSpanContextFromTrace generates span context.
func NewSpanContextFromTrace(traceparent, tracestate string) trace.SpanContext {
	sc, ok := SpanContextFromW3CString(traceparent)
	if !ok {
		return trace.SpanContext{}
	}
	ts := StateFromW3CString(tracestate)

	return sc.WithTraceState(ts)
}

// SpanContextFromW3CString extracts a span context from given string which got earlier from SpanContextToW3CString format.
func SpanContextFromW3CString(h string) (sc trace.SpanContext, ok bool) {
	if h == "" {
		return trace.SpanContext{}, false
	}
	sections := strings.Split(h, "-")
	if len(sections) < 4 {
		return trace.SpanContext{}, false
	}

	if len(sections[0]) != 2 {
		return trace.SpanContext{}, false
	}
	ver, err := hex.DecodeString(sections[0])
	if err != nil {
		return trace.SpanContext{}, false
	}
	version := int(ver[0])
	if version > maxVersion {
		return trace.SpanContext{}, false
	}

	if version == 0 && len(sections) != 4 {
		return trace.SpanContext{}, false
	}

	if len(sections[1]) != 32 {
		return trace.SpanContext{}, false
	}
	tid, err := trace.TraceIDFromHex(sections[1])
	if err != nil {
		return trace.SpanContext{}, false
	}
	sc = sc.WithTraceID(tid)

	if len(sections[2]) != 16 {
		return trace.SpanContext{}, false
	}
	sid, err := trace.SpanIDFromHex(sections[2])
	if err != nil {
		return trace.SpanContext{}, false
	}
	sc = sc.WithSpanID(sid)

	opts, err := hex.DecodeString(sections[3])
	if err != nil || len(opts) < 1 {
		return trace.SpanContext{}, false
	}
	sc = sc.WithTraceFlags(trace.TraceFlags(opts[0]))

	// Don't allow all zero trace or span ID.
	if sc.TraceID() == [16]byte{} || sc.SpanID() == [8]byte{} {
		return trace.SpanContext{}, false
	}

	return sc, true
}

// StateFromW3CString generates tracestate.
func StateFromW3CString(tracestate string) trace.TraceState {
	if tracestate == "" {
		return trace.TraceState{}
	}

	ts, err := trace.ParseTraceState(tracestate)
	if err != nil {
		return trace.TraceState{}
	}

	return ts
}

// TraceparentToW3CString gets traceparent from spancontext.
func TraceparentToW3CString(sc trace.SpanContext) string {
	flags := sc.TraceFlags() & trace.FlagsSampled

	return fmt.Sprintf("%.2x-%s-%s-%s",
		supportedVersion,
		sc.TraceID(),
		sc.SpanID(),
		flags)
}

// StateToW3CString extracts the TraceState from given SpanContext and returns its string representation.
func StateToW3CString(sc trace.SpanContext) string {
	return sc.TraceState().String()
}
