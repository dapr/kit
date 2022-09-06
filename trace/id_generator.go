package trace

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync"

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
	idGenerator *randomIDGenerator
	_           trace.IDGenerator = &randomIDGenerator{}
	idOnce      sync.Once
)

type randomIDGenerator struct {
	sync.Mutex
	randSource *rand.Rand
}

// NewSpanID returns a non-zero span ID from a randomly-chosen sequence.
func (gen *randomIDGenerator) NewSpanID(ctx context.Context, traceID apitrace.TraceID) apitrace.SpanID {
	gen.Lock()
	defer gen.Unlock()
	sid := apitrace.SpanID{}
	gen.randSource.Read(sid[:])
	return sid
}

// NewIDs returns a non-zero trace ID and a non-zero span ID from a
// randomly-chosen sequence.
func (gen *randomIDGenerator) NewIDs(ctx context.Context) (apitrace.TraceID, apitrace.SpanID) {
	gen.Lock()
	defer gen.Unlock()
	tid := apitrace.TraceID{}
	gen.randSource.Read(tid[:])
	sid := apitrace.SpanID{}
	gen.randSource.Read(sid[:])
	return tid, sid
}

// DefaultIDGenerator default idgenerator.
func DefaultIDGenerator() trace.IDGenerator {
	idOnce.Do(
		func() {
			idGenerator = &randomIDGenerator{}
			var rngSeed int64
			_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
			idGenerator.randSource = rand.New(rand.NewSource(rngSeed))
		},
	)
	return idGenerator
}
