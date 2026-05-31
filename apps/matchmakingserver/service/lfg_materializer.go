package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/rs/zerolog/log"

	matchmaking "github.com/walkline/ToCloud9/apps/matchmakingserver"
	pbGroup "github.com/walkline/ToCloud9/gen/group/pb"
	pbServRegistry "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	pbWorld "github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/gameserver/conn"
	wowguid "github.com/walkline/ToCloud9/shared/wow/guid"
)

var (
	ErrLFGMaterializerMissingPayload      = errors.New("lfg materializer payload is nil")
	ErrLFGMaterializerMissingWorldserver  = errors.New("lfg materializer leader worldserver is empty")
	ErrLFGMaterializerMissingMembers      = errors.New("lfg materializer members are empty")
	ErrLFGMaterializerMissingRegistry     = errors.New("lfg materializer servers registry client is nil")
	ErrLFGMaterializerMissingGRPCConnMgr  = errors.New("lfg materializer game server grpc connection manager is nil")
	ErrLFGMaterializerMissingWorldService = errors.New("lfg materializer worldserver grpc client is nil")
	ErrLFGMaterializerMissingRegistryData = errors.New("lfg materializer servers registry response is nil")
	ErrLFGMaterializerMissingMemberWorld  = errors.New("lfg materializer member worldserver is empty")
	ErrLFGMaterializerNoDungeonOwner      = errors.New("lfg materializer no worldserver available for dungeon map")
	ErrLFGMaterializerWorldserverNotFound = errors.New("lfg materializer worldserver not found")
)

const lfgMaterializerNoLocalPlayerRetryDelay = 250 * time.Millisecond

type LFGMaterializer struct {
	serversRegistryClient pbServRegistry.ServersRegistryServiceClient
	gameserverGRPCConnMgr conn.GameServerGRPCConnMgr
}

func NewLFGMaterializer(serversRegistryClient pbServRegistry.ServersRegistryServiceClient, gameserverGRPCConnMgr conn.GameServerGRPCConnMgr, groupServiceClient pbGroup.GroupServiceClient) *LFGMaterializer {
	return &LFGMaterializer{
		serversRegistryClient: serversRegistryClient,
		gameserverGRPCConnMgr: gameserverGRPCConnMgr,
	}
}

func (m *LFGMaterializer) MaterializeAcceptedProposal(ctx context.Context, payload *events.MatchmakingEventLfgProposalAcceptedPayload) error {
	if payload == nil {
		return ErrLFGMaterializerMissingPayload
	}
	if payload.LeaderWorldserverID == "" {
		return ErrLFGMaterializerMissingWorldserver
	}
	if len(payload.Members) == 0 {
		return ErrLFGMaterializerMissingMembers
	}
	if err := ensureLFGMaterializerMemberWorldservers(payload); err != nil {
		return err
	}
	if m.serversRegistryClient == nil {
		return ErrLFGMaterializerMissingRegistry
	}
	if m.gameserverGRPCConnMgr == nil {
		return ErrLFGMaterializerMissingGRPCConnMgr
	}

	leaderGameServerGRPCClient, err := m.gameServerClientForLeaderWorldserverID(ctx, payload)
	if err != nil {
		return err
	}
	if leaderGameServerGRPCClient == nil {
		return ErrLFGMaterializerMissingWorldService
	}

	targetServer, err := m.lfgDungeonOwnerServer(ctx, payload, leaderGameServerGRPCClient)
	if err != nil {
		return err
	}
	targetWorldserverID := targetServer.GetId()
	if targetWorldserverID == "" {
		return ErrLFGMaterializerNoDungeonOwner
	}

	splitWorldservers, err := validateLFGMaterializerMemberWorldservers(payload, targetWorldserverID)
	if err != nil {
		return err
	}

	gameServerGRPCClient, err := m.gameServerClientForRegistryServer(targetServer)
	if err != nil {
		return err
	}
	if gameServerGRPCClient == nil {
		return ErrLFGMaterializerMissingWorldService
	}

	members := make([]*pbWorld.MaterializeLfgProposalRequest_Member, 0, len(payload.Members))
	for _, member := range payload.Members {
		members = append(members, &pbWorld.MaterializeLfgProposalRequest_Member{
			PlayerGUID:      lfgMaterializerPlayerGUID(payload, member.RealmID, uint64(member.PlayerGUID)),
			SelectedRoles:   uint32(member.SelectedRoles),
			AssignedRole:    uint32(member.AssignedRole),
			QueueLeaderGUID: lfgMaterializerPlayerGUID(payload, member.QueueLeaderRealmID, uint64(member.QueueLeaderGUID)),
		})
	}

	req := &pbWorld.MaterializeLfgProposalRequest{
		Api:          matchmaking.SupportedGameServerVer,
		RealmID:      payload.RealmID,
		ProposalID:   payload.ProposalID,
		DungeonEntry: payload.DungeonEntry,
		LeaderGUID:   lfgMaterializerPlayerGUID(payload, payload.LeaderRealmID, uint64(payload.LeaderGUID)),
		Members:      members,
	}

	// The accepted-proposal event is what drives gateway worldport routing.
	// AzerothCore then remains the readiness authority for materialization by
	// returning NoLocalPlayer until the destination worldserver owns a member.
	for attempt := 1; ; attempt++ {
		res, err := gameServerGRPCClient.MaterializeLfgProposal(ctx, req)
		if err != nil {
			return fmt.Errorf("materialize lfg proposal %d on worldserver %q failed: %w", payload.ProposalID, targetWorldserverID, err)
		}
		if res == nil {
			return fmt.Errorf("materialize lfg proposal %d on worldserver %q returned nil response", payload.ProposalID, targetWorldserverID)
		}
		if res.GetStatus() == pbWorld.MaterializeLfgProposalResponse_Success {
			log.Info().
				Uint32("realmID", payload.RealmID).
				Uint32("proposalID", payload.ProposalID).
				Uint32("dungeonEntry", payload.DungeonEntry).
				Str("leaderWorldserverID", payload.LeaderWorldserverID).
				Str("targetWorldserverID", targetWorldserverID).
				Int("members", len(payload.Members)).
				Int("attempt", attempt).
				Bool("splitWorldservers", splitWorldservers).
				Msg("materialized LFG proposal")
			return nil
		}
		if !splitWorldservers || res.GetStatus() != pbWorld.MaterializeLfgProposalResponse_NoLocalPlayer {
			return fmt.Errorf("materialize lfg proposal %d on worldserver %q returned status %s", payload.ProposalID, targetWorldserverID, res.GetStatus())
		}
		if !waitForLFGMaterializerRetry(ctx) {
			return fmt.Errorf("materialize lfg proposal %d on worldserver %q still missing local players after %d attempts: %w", payload.ProposalID, targetWorldserverID, attempt, ctx.Err())
		}
	}
}

func ensureLFGMaterializerMemberWorldservers(payload *events.MatchmakingEventLfgProposalAcceptedPayload) error {
	for _, member := range payload.Members {
		if member.WorldserverID == "" {
			return ErrLFGMaterializerMissingMemberWorld
		}
	}
	return nil
}

func validateLFGMaterializerMemberWorldservers(payload *events.MatchmakingEventLfgProposalAcceptedPayload, targetWorldserverID string) (bool, error) {
	splitWorldservers := false
	for _, member := range payload.Members {
		if member.WorldserverID == "" {
			return false, ErrLFGMaterializerMissingMemberWorld
		}
		if member.WorldserverID != targetWorldserverID {
			splitWorldservers = true
		}
	}
	return splitWorldservers, nil
}

func (m *LFGMaterializer) lfgDungeonOwnerServer(ctx context.Context, payload *events.MatchmakingEventLfgProposalAcceptedPayload, metadataClient pbWorld.WorldServerServiceClient) (*pbServRegistry.Server, error) {
	dungeonInfo, err := metadataClient.GetLfgDungeonInfo(ctx, &pbWorld.GetLfgDungeonInfoRequest{
		Api:          matchmaking.SupportedGameServerVer,
		DungeonEntry: payload.DungeonEntry,
	})
	if err != nil {
		return nil, fmt.Errorf("get lfg dungeon info for proposal %d failed: %w", payload.ProposalID, err)
	}
	if dungeonInfo == nil {
		return nil, fmt.Errorf("get lfg dungeon info for proposal %d returned nil response", payload.ProposalID)
	}
	if dungeonInfo.GetStatus() != pbWorld.GetLfgDungeonInfoResponse_Success {
		return nil, fmt.Errorf("get lfg dungeon info for proposal %d returned status %s", payload.ProposalID, dungeonInfo.GetStatus())
	}

	servers, err := m.serversRegistryClient.AvailableGameServersForMapAndRealm(ctx, &pbServRegistry.AvailableGameServersForMapAndRealmRequest{
		Api:          matchmaking.SupportedServerRegistryVer,
		RealmID:      lfgMaterializationRealmID(payload),
		MapID:        dungeonInfo.GetMapID(),
		IsCrossRealm: payload.CrossRealm,
	})
	if err != nil {
		return nil, fmt.Errorf("list game servers for LFG dungeon map %d in realm %d crossrealm=%t failed: %w", dungeonInfo.GetMapID(), lfgMaterializationRealmID(payload), payload.CrossRealm, err)
	}
	if servers == nil {
		return nil, ErrLFGMaterializerMissingRegistryData
	}

	target := chooseLFGMaterializationServer(servers.GetGameServers(), payload.LeaderWorldserverID)
	if target == nil {
		return nil, ErrLFGMaterializerNoDungeonOwner
	}
	return target, nil
}

func chooseLFGMaterializationServer(servers []*pbServRegistry.Server, preferredWorldserverID string) *pbServRegistry.Server {
	for _, server := range servers {
		if server.GetId() == preferredWorldserverID {
			return server
		}
	}

	candidates := make([]*pbServRegistry.Server, 0, len(servers))
	for _, server := range servers {
		if server != nil && server.GetId() != "" {
			candidates = append(candidates, server)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].GetId() < candidates[j].GetId()
	})
	if len(candidates) == 0 {
		return nil
	}
	return candidates[0]
}

func lfgLeaderRealmID(payload *events.MatchmakingEventLfgProposalAcceptedPayload) uint32 {
	if payload == nil {
		return 0
	}
	if payload.LeaderRealmID != 0 {
		return payload.LeaderRealmID
	}
	return payload.RealmID
}

func lfgMaterializationRealmID(payload *events.MatchmakingEventLfgProposalAcceptedPayload) uint32 {
	if payload != nil && payload.CrossRealm {
		return 0
	}
	if payload == nil {
		return 0
	}
	return payload.RealmID
}

func lfgMaterializerPlayerGUID(payload *events.MatchmakingEventLfgProposalAcceptedPayload, playerRealmID uint32, playerGUID uint64) uint64 {
	if playerRealmID == 0 {
		playerRealmID = lfgMaterializationRealmID(payload)
	}

	// Materialization sends AzerothCore-facing player ObjectGuid values to the
	// target dungeon owner. For crossrealm dungeon owners realmID=0, every
	// member with a concrete source realm must stay realm-scoped.
	return wowguid.PlayerGUIDForRealm(lfgMaterializationRealmID(payload), playerRealmID, playerGUID)
}

func waitForLFGMaterializerRetry(ctx context.Context) bool {
	timer := time.NewTimer(lfgMaterializerNoLocalPlayerRetryDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}

func (m *LFGMaterializer) gameServerClientForWorldserverID(ctx context.Context, realmID uint32, worldserverID string) (pbWorld.WorldServerServiceClient, error) {
	servers, err := m.serversRegistryClient.ListGameServersForRealm(ctx, &pbServRegistry.ListGameServersForRealmRequest{
		Api:     matchmaking.SupportedServerRegistryVer,
		RealmID: realmID,
	})
	if err != nil {
		return nil, fmt.Errorf("list game servers for realm %d failed: %w", realmID, err)
	}
	if servers == nil {
		return nil, ErrLFGMaterializerMissingRegistryData
	}

	for _, server := range servers.GetGameServers() {
		if server.GetID() != worldserverID {
			continue
		}
		m.gameserverGRPCConnMgr.AddAddressMapping(server.GetAddress(), server.GetGrpcAddress())
		client, err := m.gameserverGRPCConnMgr.GRPCConnByGameServerAddress(server.GetAddress())
		if err != nil {
			return nil, fmt.Errorf("get game server grpc client for worldserver %q failed: %w", worldserverID, err)
		}
		return client, nil
	}

	return nil, fmt.Errorf("%w: leader worldserver %q not found in realm %d", ErrLFGMaterializerWorldserverNotFound, worldserverID, realmID)
}

func (m *LFGMaterializer) gameServerClientForLeaderWorldserverID(ctx context.Context, payload *events.MatchmakingEventLfgProposalAcceptedPayload) (pbWorld.WorldServerServiceClient, error) {
	leaderRealmID := lfgLeaderRealmID(payload)
	client, err := m.gameServerClientForWorldserverID(ctx, leaderRealmID, payload.LeaderWorldserverID)
	if err == nil || !payload.CrossRealm || !errors.Is(err, ErrLFGMaterializerWorldserverNotFound) {
		return client, err
	}

	materializationRealmID := lfgMaterializationRealmID(payload)
	if materializationRealmID == leaderRealmID {
		return nil, err
	}

	return m.gameServerClientForWorldserverID(ctx, materializationRealmID, payload.LeaderWorldserverID)
}

func (m *LFGMaterializer) gameServerClientForRegistryServer(server *pbServRegistry.Server) (pbWorld.WorldServerServiceClient, error) {
	if server == nil {
		return nil, ErrLFGMaterializerNoDungeonOwner
	}
	m.gameserverGRPCConnMgr.AddAddressMapping(server.GetAddress(), server.GetGrpcAddress())
	client, err := m.gameserverGRPCConnMgr.GRPCConnByGameServerAddress(server.GetAddress())
	if err != nil {
		return nil, fmt.Errorf("get game server grpc client for worldserver %q failed: %w", server.GetId(), err)
	}
	return client, nil
}
