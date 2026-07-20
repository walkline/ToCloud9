package session

import (
	"context"
	"fmt"

	root "github.com/walkline/ToCloud9/apps/gateway"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

func (s *GameSession) selectLayerGameServer(ctx context.Context, groupID uint32, preferredAlias string) (*pbServ.Server, error) {
	if s.character == nil {
		return nil, nil
	}
	response, err := s.serversRegistryClient.AvailableGameServersForMapAndRealm(ctx, &pbServ.AvailableGameServersForMapAndRealmRequest{
		Api: root.SupportedServerRegistryVer, RealmID: root.RealmID, MapID: s.character.Map,
		GroupID: groupID, PreferredGameServerAlias: preferredAlias,
	})
	if err != nil || len(response.GameServers) == 0 {
		return nil, err
	}
	return response.GameServers[0], nil
}

func (s *GameSession) applyGroupLayer(ctx context.Context, groupID uint32) error {
	server, err := s.selectLayerGameServer(ctx, groupID, "")
	if err != nil || server == nil {
		return err
	}
	if server.ID == s.currentGameServerID {
		s.currentGameServerAlias = server.Alias
		return nil
	}
	s.SendSysMessage(fmt.Sprintf("Switching to %s.", server.Alias))
	if err := s.redirectToSelectedLayer(ctx, server); err != nil {
		return err
	}
	return nil
}

func (s *GameSession) redirectToSelectedLayer(ctx context.Context, server *pbServ.Server) error {
	if server == nil || s.character == nil {
		return nil
	}
	s.gameServerGRPCConnMgr.AddAddressMapping(server.Address, server.GrpcAddress)
	client, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(server.Address)
	if err != nil {
		return fmt.Errorf("connect to layer gameserver gRPC: %w", err)
	}
	if err := s.redirectPlayerToGameServer(ctx, s.character.GUID, server.Address); err != nil {
		return err
	}
	s.gameServerGRPCClient = client
	s.currentGameServerID = server.ID
	s.currentGameServerAlias = server.Alias
	return nil
}

// redirectPlayerToGameServer reuses the existing authenticated worldserver
// handoff without changing battleground behavior.
func (s *GameSession) redirectPlayerToGameServer(ctx context.Context, characterGUID uint64, address string) error {
	return s.battlegroundPlayerRedirect(ctx, characterGUID, address)
}
