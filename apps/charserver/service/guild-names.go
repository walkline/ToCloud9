package service

import (
	"context"

	root "github.com/walkline/ToCloud9/apps/charserver"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
)

// GuildNameResolver resolves guild names in batches.
type GuildNameResolver interface {
	// GuildNamesByIDs returns guild names keyed by guild id. Unknown ids are absent.
	GuildNamesByIDs(ctx context.Context, realmID uint32, guildIDs []uint32) (map[uint32]string, error)
}

// guildNamesService is a GuildNameResolver backed by the guild service, which
// serves names from its warmed-up guilds cache.
type guildNamesService struct {
	client pbGuild.GuildServiceClient
}

// NewGuildNamesService returns GuildNameResolver backed by the guild service.
func NewGuildNamesService(client pbGuild.GuildServiceClient) GuildNameResolver {
	return &guildNamesService{client: client}
}

func (g *guildNamesService) GuildNamesByIDs(ctx context.Context, realmID uint32, guildIDs []uint32) (map[uint32]string, error) {
	if len(guildIDs) == 0 {
		return map[uint32]string{}, nil
	}

	ids := make([]uint64, 0, len(guildIDs))
	for _, id := range guildIDs {
		ids = append(ids, uint64(id))
	}

	resp, err := g.client.GetGuildNamesByIDs(ctx, &pbGuild.GetGuildNamesByIDsParams{
		Api:      root.Ver,
		RealmID:  realmID,
		GuildIDs: ids,
	})
	if err != nil {
		return nil, err
	}

	names := make(map[uint32]string, len(resp.GuildNames))
	for id, name := range resp.GuildNames {
		names[uint32(id)] = name
	}
	return names, nil
}
