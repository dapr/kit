package trace

import (
	"context"
	"crypto/rand"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
	apitrace "go.opentelemetry.io/otel/trace"
)

// init trace.
func init() {
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{}))
}

var (
	idGenerator                   = new(randomIDGenerator)
	_           trace.IDGenerator = &randomIDGenerator{}
)

type randomIDGenerator struct{}

// NewSpanID returns a non-zero span ID from a randomly-chosen sequence.
func (gen *randomIDGenerator) NewSpanID(ctx context.Context, traceID apitrace.TraceID) apitrace.SpanID {
	sid := apitrace.SpanID{}
	_, _ = rand.Read(sid[:])

	return sid
}

// NewIDs returns a non-zero trace ID and a non-zero span ID from a
// randomly-chosen sequence.
func (gen *randomIDGenerator) NewIDs(ctx context.Context) (apitrace.TraceID, apitrace.SpanID) {
	tid := apitrace.TraceID{}
	_, _ = rand.Read(tid[:])
	sid := apitrace.SpanID{}
	_, _ = rand.Read(sid[:])

	return tid, sid
}

// DefaultIDGenerator default idgenerator.
func DefaultIDGenerator() trace.IDGenerator {
	return idGenerator
}
