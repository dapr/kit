package trace

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// GenerateSpanContext generates span context.
func GenerateSpanContext() trace.SpanContext {
	tid, sid := DefaultIDGenerator().NewIDs(context.Background())
	scc := trace.SpanContextConfig{
		TraceID: tid,
		SpanID:  sid,
	}

	return trace.NewSpanContext(scc)
}

// SpanContextWithContext writes span context with context.
func SpanContextWithContext(ctx context.Context) context.Context {
	sc := GenerateSpanContext()

	return trace.ContextWithSpanContext(ctx, sc)
}

// SpanContext writes span context to target.
func SpanContextTo(ctx context.Context, to func(context.Context, trace.SpanContext) context.Context) context.Context {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ctx
	}

	return to(ctx, sc)
}

// SpanContextFrom gets span context from protocol.
func SpanContextFrom(ctx context.Context, from func(context.Context) trace.SpanContext) context.Context {
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		return ctx
	}

	sc = from(ctx)

	if sc.IsValid() {
		return trace.ContextWithSpanContext(ctx, sc)
	}

	return SpanContextWithContext(ctx)
}
