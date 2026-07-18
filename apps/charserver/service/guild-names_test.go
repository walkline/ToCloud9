package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
)

// fakeGuildClient implements only GetGuildNamesByIDs; other methods panic via the embedded nil interface.
type fakeGuildClient struct {
	pbGuild.GuildServiceClient

	lastParams *pbGuild.GetGuildNamesByIDsParams
	resp       *pbGuild.GetGuildNamesByIDsResponse
	calls      int
}

func (f *fakeGuildClient) GetGuildNamesByIDs(_ context.Context, params *pbGuild.GetGuildNamesByIDsParams, _ ...grpc.CallOption) (*pbGuild.GetGuildNamesByIDsResponse, error) {
	f.calls++
	f.lastParams = params
	return f.resp, nil
}

func TestGuildNamesByIDs_Batches(t *testing.T) {
	client := &fakeGuildClient{resp: &pbGuild.GetGuildNamesByIDsResponse{
		GuildNames: map[uint64]string{1: "First", 2: "Second"},
	}}

	names, err := NewGuildNamesService(client).GuildNamesByIDs(context.Background(), 1, []uint32{1, 2, 3})
	assert.NoError(t, err)
	assert.Equal(t, map[uint32]string{1: "First", 2: "Second"}, names)
	assert.Equal(t, 1, client.calls)
	assert.Equal(t, []uint64{1, 2, 3}, client.lastParams.GuildIDs)
}

func TestGuildNamesByIDs_EmptySkipsCall(t *testing.T) {
	client := &fakeGuildClient{}

	names, err := NewGuildNamesService(client).GuildNamesByIDs(context.Background(), 1, nil)
	assert.NoError(t, err)
	assert.Empty(t, names)
	assert.Equal(t, 0, client.calls)
}
