package server

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/gen/guilds/pb"
)

type getGuildNamesRealServiceStub struct {
	pb.UnimplementedGuildServiceServer
	resp *pb.GetGuildNamesByIDsResponse
}

func (s *getGuildNamesRealServiceStub) GetGuildNamesByIDs(_ context.Context, _ *pb.GetGuildNamesByIDsParams) (*pb.GetGuildNamesByIDsResponse, error) {
	return s.resp, nil
}

// The middleware embeds pb.UnimplementedGuildServiceServer, so a method it does
// not forward explicitly silently answers codes.Unimplemented in production:
// assert the call reaches the real service.
func TestDebugLoggerMiddlewareForwardsGetGuildNamesByIDs(t *testing.T) {
	want := &pb.GetGuildNamesByIDsResponse{GuildNames: map[uint64]string{7: "Moebius"}}
	m := NewGuildsDebugLoggerMiddleware(&getGuildNamesRealServiceStub{resp: want}, zerolog.Nop())

	got, err := m.GetGuildNamesByIDs(context.Background(), &pb.GetGuildNamesByIDsParams{RealmID: 1, GuildIDs: []uint64{7}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("expected the real service response, got %v", got)
	}
}
