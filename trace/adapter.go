package trace

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// GRPCServerUnaryInterceptor grpc unary server interceptor.
func GRPCServerUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	newCtx := SpanContextFrom(ctx, SpanContextFromGrpc)
	return handler(newCtx, req)
}

type wrappedStream struct {
	ctx context.Context
	grpc.ServerStream
}

func (ws wrappedStream) Context() context.Context {
	if ws.ctx != nil {
		return ws.ctx
	}
	return ws.ServerStream.Context()
}

// ServerStreamInterceptor server stream.
func GRPCServerStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	newCtx := SpanContextFrom(ss.Context(), SpanContextFromGrpc)
	return handler(srv, &wrappedStream{ServerStream: ss, ctx: newCtx})
}

// SpanContextGrpc gets span context from grpc request.
func SpanContextFromGrpc(ctx context.Context) trace.SpanContext {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return trace.SpanContext{}
	}

	m1 := md.Get(traceparentHeader)
	m2 := md.Get(tracestateHeader)

	var traceparent, tracestate string
	if len(m1) > 0 {
		traceparent = m1[0]
	}
	if len(m2) > 0 {
		tracestate = m2[0]
	}
	if len(traceparent) == 0 {
		return trace.SpanContext{}
	}

	return NewSpanContextFromTrace(traceparent, tracestate)
}

// GRPCClientUnaryInterceptor gets span context for unary client request.
func GRPCClientUnaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	newCtx := SpanContextTo(ctx, SpanContextToGrpc)
	return invoker(newCtx, method, req, reply, cc, opts...)
}

// GRPCClientStreamInterceptor gets span context from stream client request.
func GRPCClientStreamInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	newCtx := SpanContextTo(ctx, SpanContextToGrpc)
	return streamer(newCtx, desc, cc, method, opts...)
}

// SpanContextToGrpc writes span context to grpc outgoing metadata.
func SpanContextToGrpc(ctx context.Context, sc trace.SpanContext) context.Context {
	return metadata.AppendToOutgoingContext(ctx,
		traceparentHeader, Traceparent(sc),
		tracestateHeader, sc.TraceState().String(),
	)
}

// SpanContextHTTP writes span context to http header.
func SpanContextToHTTP(ctx context.Context, h http.Header) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return
	}

	h.Set(traceparentHeader, Traceparent(sc))
	h.Set(tracestateHeader, sc.TraceState().String())
}

// SpanContextFromHTTP get span context from http request.
func SpanContextFromHTTP(h http.Header) trace.SpanContext {
	traceparent := h.Get(traceparentHeader)
	tracestate := h.Get(tracestateHeader)
	if len(traceparent) > 0 {
		return NewSpanContextFromTrace(traceparent, tracestate)
	}

	return trace.SpanContext{}
}
