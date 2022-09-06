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
	gpb "google.golang.org/grpc/examples/features/proto/echo"
)

const message = "hello world!"

func TestGRPCStream(t *testing.T) {
	const (
		port = ":8086"
		addr = "localhost:8086"
	)

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	fmt.Printf("server listening at %v\n", lis.Addr())

	s := grpc.NewServer(grpc.UnaryInterceptor(GRPCServerUnaryInterceptor),
		grpc.StreamInterceptor(GRPCServerStreamInterceptor))

	gpb.RegisterEchoServer(s, &gStreamServer{})
	go func() {
		if err = s.Serve(lis); err != nil {
			t.Error("serve", err.Error())
		}
	}()

	// ############# client #############
	time.Sleep(2 * time.Second)

	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(GRPCClientUnaryInterceptor),
		grpc.WithChainStreamInterceptor(GRPCClientStreamInterceptor),
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	c := gpb.NewEchoClient(conn)

	unaryCallWithMetadata(t, c, message)
	time.Sleep(1 * time.Second)

	serverStreamingWithMetadata(t, c, message)
	time.Sleep(1 * time.Second)

	clientStreamWithMetadata(t, c, message)
	time.Sleep(1 * time.Second)

	bidirectionalWithMetadata(t, c, message)
}

func unaryCallWithMetadata(t *testing.T, c gpb.EchoClient, message string) {
	ctx, sc := spanContextWithContext(context.Background())

	in := gpb.EchoRequest{Message: message}
	out, err := c.UnaryEcho(ctx, &in)
	if err != nil {
		t.Error("unaryEcho", err.Error())
		return
	}

	traceID := sc.TraceID().String()
	if out.Message != traceID {
		t.Errorf("trace id not match: '%s' '%s'", out.Message, traceID)
		return
	}

	t.Log(out.Message, traceID)
}

func serverStreamingWithMetadata(t *testing.T, c gpb.EchoClient, message string) {
	ctx, sc := spanContextWithContext(context.Background())

	in := gpb.EchoRequest{Message: message}
	strm, err := c.ServerStreamingEcho(ctx, &in)
	if err != nil {
		t.Error("unaryEcho", err.Error())
		return
	}

	out, err := strm.Recv()
	if err != nil {
		t.Error("unaryEcho", err.Error())
		return
	}
	traceID := sc.TraceID().String()
	if out.Message != traceID {
		t.Errorf("trace id not match: '%s' '%s'", out.Message, traceID)
		return
	}

	t.Log(out.Message, traceID)
}

func clientStreamWithMetadata(t *testing.T, c gpb.EchoClient, message string) {
	ctx, sc := spanContextWithContext(context.Background())

	strm, err := c.ClientStreamingEcho(ctx)
	if err != nil {
		t.Error("clientStream", err.Error())
		return
	}

	in := gpb.EchoRequest{Message: message}
	if err = strm.Send(&in); err != nil {
		t.Error("clientSend", err.Error())
		return
	}

	out, err := strm.CloseAndRecv()
	if err != nil {
		t.Error("unaryEcho", err.Error())
		return
	}

	traceID := sc.TraceID().String()
	if out.Message != traceID {
		t.Errorf("trace id not match: '%s' '%s'", out.Message, traceID)
		return
	}

	t.Log(out.Message, traceID)
}

func bidirectionalWithMetadata(t *testing.T, c gpb.EchoClient, message string) {
	ctx, sc := spanContextWithContext(context.Background())

	strm, err := c.ClientStreamingEcho(ctx)
	if err != nil {
		t.Error("clientStream", err.Error())
		return
	}

	in := gpb.EchoRequest{Message: message}
	if err = strm.Send(&in); err != nil {
		t.Error("clientSend", err.Error())
		return
	}

	out, err := strm.CloseAndRecv()
	if err != nil {
		t.Error("unaryEcho", err.Error())
		return
	}
	traceID := sc.TraceID().String()
	if out.Message != traceID {
		t.Errorf("trace id not match: '%s' '%s'", out.Message, traceID)
		return
	}

	t.Log(out.Message, traceID)
}

type gStreamServer struct {
	gpb.UnimplementedEchoServer
}

func (s *gStreamServer) UnaryEcho(ctx context.Context, in *gpb.EchoRequest) (*gpb.EchoResponse, error) {
	traceID := TraceID(ctx)
	log.Printf("received: %s %s \n", in.Message, traceID)
	return &gpb.EchoResponse{Message: traceID}, nil
}

func (s *gStreamServer) ServerStreamingEcho(in *gpb.EchoRequest, stream gpb.Echo_ServerStreamingEchoServer) error {
	traceID := TraceID(stream.Context())
	log.Printf("received: %s %s \n", in.Message, traceID)

	if err := stream.Send(&gpb.EchoResponse{Message: traceID}); err != nil {
		log.Println("send error", err.Error())
		return nil
	}

	return nil
}

func (s *gStreamServer) ClientStreamingEcho(stream gpb.Echo_ClientStreamingEchoServer) error {
	in, err := stream.Recv()
	if err != nil {
		log.Println("recv error", err.Error())
		return nil
	}

	traceID := TraceID(stream.Context())
	log.Printf("received: %s %s \n", in.Message, traceID)

	if err := stream.SendAndClose(&gpb.EchoResponse{Message: traceID}); err != nil {
		log.Println("send error", err.Error())
		return nil
	}

	return nil
}

func (s *gStreamServer) BidirectionalStreamingEcho(stream gpb.Echo_BidirectionalStreamingEchoServer) error {
	in, err := stream.Recv()
	if err != nil {
		log.Println("recv error", err.Error())
		return nil
	}

	traceID := TraceID(stream.Context())
	log.Printf("received: %s %s \n", in.Message, traceID)

	if err := stream.Send(&gpb.EchoResponse{Message: traceID}); err != nil {
		log.Println("send error", err.Error())
		return nil
	}
	return nil
}
