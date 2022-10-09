package trace

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"

	"google.golang.org/grpc/credentials/insecure"
	gpb "google.golang.org/grpc/examples/helloworld/helloworld"

	"go.opentelemetry.io/otel/trace"
)

func spanContextWithContext(ctx context.Context) (context.Context, trace.SpanContext) {
	sc := GenerateSpanContext()
	return trace.ContextWithSpanContext(ctx, sc), sc
}

func TestGrpc(t *testing.T) {
	var (
		port = 8081
		addr = "localhost:8081"
	)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
		return
	}

	s := grpc.NewServer(grpc.UnaryInterceptor(GRPCServerUnaryInterceptor))
	gpb.RegisterGreeterServer(s, new(testServer))

	go func() {
		if err = s.Serve(lis); err != nil {
			t.Errorf("failed to serve: %v", err)
		}
	}()
	defer s.Stop()

	time.Sleep(2 * time.Second)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(GRPCClientUnaryInterceptor))
	if err != nil {
		t.Errorf("did not connect: %v", err)
		return
	}
	defer conn.Close()

	ctx, sc := spanContextWithContext(context.Background())

	c := gpb.NewGreeterClient(conn)
	r, err := c.SayHello(ctx, &gpb.HelloRequest{Name: "Tom"})
	if err != nil {
		t.Errorf("could not greet: %v", err)
		return
	}

	traceID := sc.TraceID().String()
	if traceID != r.GetMessage() {
		t.Errorf("trace id not match: '%s' '%s'", r.GetMessage(), traceID)
		return
	}

	t.Log(r.GetMessage(), traceID)
}

// server is used to implement helloworld.GreeterServer.
type testServer struct {
	gpb.UnimplementedGreeterServer
}

// SayHello implements helloworld.GreeterServer
func (s *testServer) SayHello(ctx context.Context, in *gpb.HelloRequest) (*gpb.HelloReply, error) {
	traceID := ID(ctx)
	log.Printf("Received: %s, %s", in.GetName(), traceID)
	return &gpb.HelloReply{Message: traceID}, nil
}
