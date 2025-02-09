package minimatch

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/castaneai/minimatch/pkg/statestore"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"open-match.dev/open-match/pkg/pb"
)

type TestServer struct {
	mm   *MiniMatch
	addr string
}

// RunTestServer helps with integration tests using Open Match.
// It provides an Open Match Frontend equivalent API in the Go process using a random port.
func RunTestServer(t *testing.T, profile *pb.MatchProfile, mmf MatchFunction, assigner Assigner) *TestServer {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := statestore.NewRedisStore(rc)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen test server: %+v", err)
	}
	t.Cleanup(func() { _ = lis.Close() })
	sv := grpc.NewServer()
	mm := NewMiniMatch(store)
	mm.AddBackend(profile, mmf, assigner)
	go func() {
		if err := mm.StartBackend(context.Background(), 1*time.Second); err != nil {
			t.Logf("error occured in minimatch backend: %+v", err)
		}
	}()
	pb.RegisterFrontendServiceServer(sv, mm.FrontendService())
	t.Cleanup(func() { sv.Stop() })
	go func() { _ = sv.Serve(lis) }()
	return &TestServer{
		mm:   mm,
		addr: lis.Addr().String(),
	}
}

func (ts *TestServer) DialFrontend(t *testing.T) pb.FrontendServiceClient {
	cc, err := grpc.Dial(ts.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial to minimatch test server: %+v", err)
	}
	return pb.NewFrontendServiceClient(cc)
}

// TickBackend triggers a Director's Tick, which immediately calls Match Function and Assigner.
// This is useful for sleep-independent testing.
func (ts *TestServer) TickBackend() error {
	return ts.mm.TickBackend(context.Background())
}
