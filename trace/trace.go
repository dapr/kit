package trace

import (
	"context"
	"encoding/hex"
	"regexp"
	"strings"

	"go.opencensus.io/trace"
	"go.opencensus.io/trace/propagation"
	"go.opencensus.io/trace/tracestate"
	"google.golang.org/grpc/metadata"
)

// We have leveraged the code from opencensus-go plugin to adhere the w3c trace context.
// Reference : https://github.com/census-instrumentation/opencensus-go/blob/master/plugin/ochttp/propagation/tracecontext/propagation.go.
const (
	maxVersion       = 254
	maxTracestateLen = 512
	// tracebinMetadata trace key for grpc protocol.
	tracebinMetadata = "grpc-trace-bin"
	trimOWSRegexFmt  = `^[\x09\x20]*(.*[^\x20\x09])[\x09\x20]*$`
)

var trimOWSRegExp = regexp.MustCompile(trimOWSRegexFmt)

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
	tid, err := hex.DecodeString(sections[1])
	if err != nil {
		return trace.SpanContext{}, false
	}
	copy(sc.TraceID[:], tid)

	if len(sections[2]) != 16 {
		return trace.SpanContext{}, false
	}
	sid, err := hex.DecodeString(sections[2])
	if err != nil {
		return trace.SpanContext{}, false
	}
	copy(sc.SpanID[:], sid)

	opts, err := hex.DecodeString(sections[3])
	if err != nil || len(opts) < 1 {
		return trace.SpanContext{}, false
	}
	sc.TraceOptions = trace.TraceOptions(opts[0])

	// Don't allow all zero trace or span ID.
	if sc.TraceID == [16]byte{} || sc.SpanID == [8]byte{} {
		return trace.SpanContext{}, false
	}

	return sc, true
}

// StateFromW3CString extracts a span tracestate from given string which got earlier from StateFromW3CString format.
func StateFromW3CString(h string) *tracestate.Tracestate {
	if h == "" {
		return nil
	}

	entries := make([]tracestate.Entry, 0, len(h))
	pairs := strings.Split(h, ",")
	hdrLenWithoutOWS := len(pairs) - 1 // Number of commas.
	for _, pair := range pairs {
		matches := trimOWSRegExp.FindStringSubmatch(pair)
		if matches == nil {
			return nil
		}
		pair = matches[1]
		hdrLenWithoutOWS += len(pair)
		if hdrLenWithoutOWS > maxTracestateLen {
			return nil
		}
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			return nil
		}
		entries = append(entries, tracestate.Entry{Key: kv[0], Value: kv[1]})
	}
	ts, err := tracestate.New(nil, entries...)
	if err != nil {
		return nil
	}

	return ts
}

// GetSpanContext get span context.
func GetSpanContext(ctx context.Context) trace.SpanContext {
	var (
		md metadata.MD
		sc trace.SpanContext
		ok bool
	)
	if md, ok = metadata.FromIncomingContext(ctx); !ok {
		if md, ok = metadata.FromOutgoingContext(ctx); !ok {
			return sc
		}
	}
	if md != nil {
		if len(md[tracebinMetadata]) > 0 {
			binV := md[tracebinMetadata][0]
			sc, _ = propagation.FromBinary([]byte(binV))
		}

	}
	return sc
}
