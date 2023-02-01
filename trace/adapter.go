package trace

import (
	"context"
	"net/http"

	"github.com/valyala/fasthttp"
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

	m1 := md.Get(TraceparentHeader)
	m2 := md.Get(TracestateHeader)

	var traceparent, tracestate string
	if len(m1) > 0 {
		traceparent = m1[0]
	}
	if len(traceparent) == 0 {
		return trace.SpanContext{}
	}
	if len(m2) > 0 {
		tracestate = m2[0]
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
		TraceparentHeader, Traceparent(sc),
		TracestateHeader, sc.TraceState().String(),
	)
}

// SpanContextHTTP writes span context to http header.
func SpanContextToHTTP(ctx context.Context, h http.Header) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return
	}

	h.Set(TraceparentHeader, Traceparent(sc))
	h.Set(TracestateHeader, sc.TraceState().String())
}

// SpanContextFromHTTP get span context from http request.
func SpanContextFromHTTP(h http.Header) trace.SpanContext {
	traceparent := h.Get(TraceparentHeader)
	tracestate := h.Get(TracestateHeader)
	if len(traceparent) > 0 {
		return NewSpanContextFromTrace(traceparent, tracestate)
	}

	return trace.SpanContext{}
}

// SpanContextToFastHTTP writes span context to fasthttp header.
func SpanContextToFastHTTP(ctx context.Context, reqCtx *fasthttp.RequestCtx) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return
	}

	reqCtx.Request.Header.Set(TraceparentHeader, Traceparent(sc))
	reqCtx.Request.Header.Set(TracestateHeader, sc.TraceState().String())
}

// SpanContextFromFastHTTP get span context from fasthttp request.
func SpanContextFromFastHTTP(ctx context.Context) trace.SpanContext {
	reqCtx, ok := ctx.(*fasthttp.RequestCtx)
	if !ok {
		return trace.SpanContext{}
	}

	traceparent := reqCtx.Request.Header.Peek(TraceparentHeader)
	tracestate := reqCtx.Request.Header.Peek(TracestateHeader)
	if len(traceparent) > 0 {
		return NewSpanContextFromTrace(string(traceparent), string(tracestate))
	}

	return trace.SpanContext{}
}
