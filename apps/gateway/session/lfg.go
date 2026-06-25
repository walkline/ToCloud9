package session

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbGroup "github.com/walkline/ToCloud9/gen/group/pb"
	pbMM "github.com/walkline/ToCloud9/gen/matchmaking/pb"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	pbGameServ "github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

const (
	lfgRoleTank   uint8 = 0x02
	lfgRoleHealer uint8 = 0x04
	lfgRoleDamage uint8 = 0x08

	lfgUpdateTypeLeaderUnk1       uint8 = 1
	lfgUpdateTypeRolecheckAborted uint8 = 4
	lfgUpdateTypeJoinQueue        uint8 = 5
	lfgUpdateTypeRemovedFromQueue uint8 = 7
	lfgUpdateTypeProposalFailed   uint8 = 8
	lfgUpdateTypeProposalDeclined uint8 = 9
	lfgUpdateTypeGroupFound       uint8 = 10
	lfgUpdateTypeAddedToQueue     uint8 = 12
	lfgUpdateTypeProposalBegin    uint8 = 13
	lfgUpdateTypeUpdateStatus     uint8 = 14

	lfgJoinOK            uint32 = 0
	lfgJoinInternalError uint32 = 4

	lfgRolecheckDefault     uint32 = 0
	lfgRolecheckInitialited uint32 = 2

	lfgDungeonIDMask uint32 = 0x00FFFFFF

	// Accepted proposal handling runs after every member has answered and may
	// need to save, redirect, and relogin through a crossrealm dungeon owner.
	lfgProposalAcceptedTimeout = 90 * time.Second

	lfgQueueUnavailableMessage = "Dungeon Finder queue was reset because the matchmaking service restarted. Please queue again."
)

var errLfgNativeWorldportNoHandler = errors.New("source worldserver has no LFG teleport handler")

func (s *GameSession) HandleLfgJoin(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	roles := r.Uint32()
	/*noPartialClear*/ _ = r.Uint8()
	/*achievements*/ _ = r.Uint8()

	slotCount := r.Uint8()
	dungeons := make([]uint32, 0, slotCount)
	for i := uint8(0); i < slotCount; i++ {
		dungeons = append(dungeons, r.Uint32())
	}

	needsCount := r.Uint8()
	for i := uint8(0); i < needsCount; i++ {
		_ = r.Uint8()
	}
	comment := r.String()

	members, memberOnline, ok, err := s.lfgMembersForJoin(ctx, uint8(roles))
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	validDungeons, result, err := s.validateLfgJoinMembers(ctx, dungeons, members, memberOnline)
	if err != nil || result != pbMM.LfgJoinResult_LFG_JOIN_OK {
		if err != nil {
			s.sendLfgJoinResult(lfgJoinInternalError, lfgRolecheckDefault)
			return err
		}
		s.sendLfgJoinResult(uint32(result), lfgRolecheckDefault)
		return nil
	}

	res, err := s.matchmakingServiceClient.JoinLfg(ctx, &pbMM.JoinLfgRequest{
		Api:            root.SupportedMatchmakingServiceVer,
		RealmID:        root.RealmID,
		LeaderGUID:     s.character.GUID,
		Members:        members,
		DungeonEntries: validDungeons,
		Comment:        comment,
	})
	if err != nil {
		s.sendLfgJoinResult(lfgJoinInternalError, lfgRolecheckDefault)
		return err
	}
	if res.GetResult() != pbMM.LfgJoinResult_LFG_JOIN_OK {
		s.sendLfgJoinResult(uint32(res.GetResult()), lfgRolecheckDefault)
		return nil
	}
	s.trackLfgPendingJoin()

	return nil
}

func (s *GameSession) validateLfgJoinMembers(ctx context.Context, dungeons []uint32, members []*pbMM.LfgMember, memberOnline map[uint64]bool) ([]uint32, pbMM.LfgJoinResult, error) {
	validDungeons := append([]uint32(nil), dungeons...)
	for _, member := range members {
		if online, ok := memberOnline[lfgMemberServiceGUID(member.GetRealmID(), member.GetPlayerGUID())]; ok && !online {
			return nil, pbMM.LfgJoinResult_LFG_JOIN_DISCONNECTED, nil
		}
	}

	placements := map[uint64]*pbGroup.MemberPlacement{}
	remoteMembers := make([]uint64, 0, len(members))
	for _, member := range members {
		memberRealmID := lfgPlayerRealmID(member.GetRealmID(), member.GetPlayerGUID())
		memberGUID := lfgMemberServiceGUID(member.GetRealmID(), member.GetPlayerGUID())
		if guid.SamePlayer(root.RealmID, s.character.GUID, memberRealmID, member.GetPlayerGUID()) {
			continue
		}
		remoteMembers = append(remoteMembers, memberGUID)
	}
	if len(remoteMembers) > 0 {
		if s.groupServiceClient == nil {
			return nil, pbMM.LfgJoinResult_LFG_JOIN_PARTY_INFO_FAILED, nil
		}
		res, err := s.groupServiceClient.GetMemberPlacements(ctx, &pbGroup.GetMemberPlacementsRequest{
			Api:         root.SupportedGroupServiceVer,
			RealmID:     root.RealmID,
			MemberGUIDs: remoteMembers,
		})
		if err != nil {
			return nil, pbMM.LfgJoinResult_LFG_JOIN_INTERNAL_ERROR, err
		}
		for _, placement := range res.GetPlacements() {
			placements[placement.GetMemberGUID()] = placement
		}
	}

	localMemberIndex := -1
	for i, member := range members {
		memberRealmID := lfgPlayerRealmID(member.GetRealmID(), member.GetPlayerGUID())
		if guid.SamePlayer(root.RealmID, s.character.GUID, memberRealmID, member.GetPlayerGUID()) {
			localMemberIndex = i
			break
		}
	}
	if localMemberIndex < 0 {
		return nil, pbMM.LfgJoinResult_LFG_JOIN_PARTY_INFO_FAILED, nil
	}

	orderedMembers := make([]*pbMM.LfgMember, 0, len(members))
	orderedMembers = append(orderedMembers, members[localMemberIndex])
	for i, member := range members {
		if i != localMemberIndex {
			orderedMembers = append(orderedMembers, member)
		}
	}

	for _, member := range orderedMembers {
		memberRealmID := lfgPlayerRealmID(member.GetRealmID(), member.GetPlayerGUID())
		memberLowGUID := guid.PlayerLowGUID(member.GetPlayerGUID())
		isLocalPlayer := guid.SamePlayer(root.RealmID, s.character.GUID, memberRealmID, member.GetPlayerGUID())
		client := s.gameServerGRPCClient
		checkPlayerGUID := memberLowGUID
		checkDungeons := validDungeons
		checksCurrentWorldserver := true
		lockResult := pbMM.LfgJoinResult_LFG_JOIN_NOT_MEET_REQS
		if isLocalPlayer {
			checkDungeons = dungeons
		} else {
			memberServiceGUID := lfgMemberServiceGUID(memberRealmID, member.GetPlayerGUID())
			placement := placements[memberServiceGUID]
			lockResult = pbMM.LfgJoinResult_LFG_JOIN_PARTY_NOT_MEET_REQS
			if placement == nil || !placement.GetFresh() || placement.GetWorldserverID() == "" {
				return nil, pbMM.LfgJoinResult_LFG_JOIN_PARTY_INFO_FAILED, nil
			}
			if !placement.GetOnline() {
				return nil, pbMM.LfgJoinResult_LFG_JOIN_DISCONNECTED, nil
			}
			member.WorldserverID = placement.GetWorldserverID()
			if member.WorldserverID != "" && member.WorldserverID != s.worldserverID {
				var err error
				var server *pbServ.GameServerDetailed
				client, server, err = s.lfgGameServerClientAndServerByWorldserverID(ctx, memberRealmID, member.WorldserverID)
				if err != nil {
					return nil, pbMM.LfgJoinResult_LFG_JOIN_INTERNAL_ERROR, err
				}
				if lfgGameServerIsCrossRealm(server) {
					checkPlayerGUID = guid.PlayerGUIDForRealm(0, memberRealmID, memberLowGUID)
				}
				checksCurrentWorldserver = false
			}
		}
		if checksCurrentWorldserver {
			checkPlayerGUID = s.lfgPlayerGUIDForCurrentWorldserver(memberRealmID, memberLowGUID)
		}

		lockInfo, result, err := s.validateLfgPlayerLocks(ctx, checkPlayerGUID, checkDungeons, client, lockResult)
		if err != nil || result != pbMM.LfgJoinResult_LFG_JOIN_OK {
			return nil, result, err
		}
		if isLocalPlayer && len(lockInfo.GetValidDungeonEntries()) > 0 {
			validDungeons = append([]uint32(nil), lockInfo.GetValidDungeonEntries()...)
		}
	}

	return validDungeons, pbMM.LfgJoinResult_LFG_JOIN_OK, nil
}

func lfgMemberServiceGUID(memberRealmID uint32, playerGUID uint64) uint64 {
	return guid.PlayerGUIDForRealm(root.RealmID, lfgPlayerRealmID(memberRealmID, playerGUID), playerGUID)
}

func lfgPlayerRealmID(memberRealmID uint32, playerGUID uint64) uint32 {
	return guid.PlayerRealmIDOrDefault(lfgMemberRealmID(memberRealmID), playerGUID)
}

func (s *GameSession) lfgPlayerGUIDForCurrentWorldserver(memberRealmID uint32, playerGUID uint64) uint64 {
	playerLowGUID := guid.PlayerLowGUID(playerGUID)
	if s.currentMapTransferRouting != nil && s.currentMapTransferRouting.isCrossRealm && s.currentMapTransferRouting.realmID == 0 {
		return guid.PlayerGUIDForRealm(0, lfgMemberRealmID(memberRealmID), playerLowGUID)
	}
	return playerLowGUID
}

func (s *GameSession) validateLfgPlayerLocks(ctx context.Context, playerGUID uint64, dungeons []uint32, client pbGameServ.WorldServerServiceClient, lockedResult pbMM.LfgJoinResult) (*pbGameServ.GetLfgPlayerLockInfoResponse, pbMM.LfgJoinResult, error) {
	if client == nil {
		return nil, pbMM.LfgJoinResult_LFG_JOIN_INTERNAL_ERROR, nil
	}

	res, err := client.GetLfgPlayerLockInfo(ctx, &pbGameServ.GetLfgPlayerLockInfoRequest{
		Api:            root.SupportedGameServerVer,
		PlayerGUID:     playerGUID,
		DungeonEntries: dungeons,
	})
	if err != nil {
		return nil, pbMM.LfgJoinResult_LFG_JOIN_INTERNAL_ERROR, err
	}
	if res == nil {
		return nil, pbMM.LfgJoinResult_LFG_JOIN_INTERNAL_ERROR, nil
	}

	switch res.GetStatus() {
	case pbGameServ.GetLfgPlayerLockInfoResponse_Success:
		if result := lfgJoinRestrictionResult(res.GetJoinResult(), lockedResult); result != pbMM.LfgJoinResult_LFG_JOIN_OK {
			return res, result, nil
		}
		if len(res.GetLocks()) > 0 {
			return res, lockedResult, nil
		}
		return res, pbMM.LfgJoinResult_LFG_JOIN_OK, nil
	case pbGameServ.GetLfgPlayerLockInfoResponse_PlayerNotFound:
		return res, pbMM.LfgJoinResult_LFG_JOIN_PARTY_INFO_FAILED, nil
	default:
		return res, pbMM.LfgJoinResult_LFG_JOIN_INTERNAL_ERROR, nil
	}
}

func lfgJoinRestrictionResult(joinResult uint32, lockedResult pbMM.LfgJoinResult) pbMM.LfgJoinResult {
	result := pbMM.LfgJoinResult(joinResult)
	if result == pbMM.LfgJoinResult_LFG_JOIN_OK {
		return pbMM.LfgJoinResult_LFG_JOIN_OK
	}
	if lockedResult != pbMM.LfgJoinResult_LFG_JOIN_PARTY_NOT_MEET_REQS {
		return result
	}

	switch result {
	case pbMM.LfgJoinResult_LFG_JOIN_DESERTER:
		return pbMM.LfgJoinResult_LFG_JOIN_PARTY_DESERTER
	case pbMM.LfgJoinResult_LFG_JOIN_RANDOM_COOLDOWN:
		return pbMM.LfgJoinResult_LFG_JOIN_PARTY_RANDOM_COOLDOWN
	case pbMM.LfgJoinResult_LFG_JOIN_NOT_MEET_REQS:
		return pbMM.LfgJoinResult_LFG_JOIN_PARTY_NOT_MEET_REQS
	default:
		return result
	}
}

func (s *GameSession) lfgGameServerClientAndServerByWorldserverID(ctx context.Context, realmID uint32, worldserverID string) (pbGameServ.WorldServerServiceClient, *pbServ.GameServerDetailed, error) {
	server, err := s.lfgGameServerByWorldserverID(ctx, realmID, worldserverID)
	if err != nil {
		return nil, nil, err
	}
	if s.gameServerGRPCConnMgr == nil {
		return nil, nil, fmt.Errorf("game server grpc connection manager is nil")
	}
	s.gameServerGRPCConnMgr.AddAddressMapping(server.GetAddress(), server.GetGrpcAddress())
	client, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(server.GetAddress())
	if err != nil {
		return nil, nil, fmt.Errorf("get game server grpc client for worldserver %q failed: %w", worldserverID, err)
	}
	return client, server, nil
}

func (s *GameSession) lfgGameServerByWorldserverID(ctx context.Context, realmID uint32, worldserverID string) (*pbServ.GameServerDetailed, error) {
	if s.serversRegistryClient == nil {
		return nil, fmt.Errorf("servers registry client is nil")
	}
	if realmID == 0 {
		realmID = root.RealmID
	}

	server, err := s.lfgFindGameServerByWorldserverID(ctx, realmID, false, worldserverID)
	if err != nil {
		return nil, err
	}
	if server != nil {
		return server, nil
	}

	server, err = s.lfgFindGameServerByWorldserverID(ctx, 0, true, worldserverID)
	if err != nil {
		return nil, err
	}
	if server != nil {
		return server, nil
	}

	return nil, fmt.Errorf("worldserver %q not found in registry", worldserverID)
}

func (s *GameSession) lfgFindGameServerByWorldserverID(ctx context.Context, realmID uint32, crossRealm bool, worldserverID string) (*pbServ.GameServerDetailed, error) {
	servers, err := s.serversRegistryClient.ListGameServersForRealm(ctx, &pbServ.ListGameServersForRealmRequest{
		Api:          root.SupportedServerRegistryVer,
		RealmID:      realmID,
		IsCrossRealm: crossRealm,
	})
	if err != nil {
		return nil, err
	}
	for _, server := range servers.GetGameServers() {
		if server.GetID() != worldserverID {
			continue
		}
		return server, nil
	}

	return nil, nil
}

func lfgGameServerIsCrossRealm(server *pbServ.GameServerDetailed) bool {
	return server != nil && server.GetIsCrossRealm()
}

func (s *GameSession) lfgMembersForJoin(ctx context.Context, roles uint8) ([]*pbMM.LfgMember, map[uint64]bool, bool, error) {
	members := []*pbMM.LfgMember{{
		RealmID:       root.RealmID,
		PlayerGUID:    s.character.GUID,
		Roles:         uint32(roles),
		Leader:        true,
		WorldserverID: s.worldserverID,
	}}
	memberOnline := map[uint64]bool{
		lfgMemberServiceGUID(root.RealmID, s.character.GUID): true,
	}

	if s.groupServiceClient == nil {
		return members, memberOnline, true, nil
	}

	groupResp, err := s.groupServiceClient.GetGroupByMember(ctx, &pbGroup.GetGroupByMemberRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil || groupResp == nil || groupResp.Group == nil || len(groupResp.Group.Members) == 0 {
		return members, memberOnline, true, nil
	}

	groupRealmID := lfgMemberRealmID(groupHomeRealmIDFromPB(groupResp.Group))
	if !guid.SamePlayer(groupRealmID, groupResp.Group.GetLeader(), root.RealmID, s.character.GUID) {
		return nil, nil, false, nil
	}

	members = make([]*pbMM.LfgMember, 0, len(groupResp.Group.Members))
	for _, member := range groupResp.Group.Members {
		memberRealmID := groupMemberRealmID(groupRealmID, member)
		memberLowGUID := guid.PlayerLowGUID(member.GetGuid())
		isLocalMember := guid.SamePlayer(memberRealmID, member.GetGuid(), root.RealmID, s.character.GUID)

		memberRoles := member.GetRoles()
		if isLocalMember {
			memberRoles = uint32(roles)
		}
		members = append(members, &pbMM.LfgMember{
			RealmID:       memberRealmID,
			PlayerGUID:    memberLowGUID,
			Roles:         memberRoles,
			Leader:        guid.SamePlayer(groupRealmID, groupResp.Group.GetLeader(), memberRealmID, memberLowGUID),
			WorldserverID: lfgJoinMemberWorldserverID(memberRealmID, memberLowGUID, root.RealmID, s.character.GUID, s.worldserverID),
		})
		memberOnline[lfgMemberServiceGUID(memberRealmID, memberLowGUID)] = member.GetIsOnline()
	}

	return members, memberOnline, true, nil
}

func lfgJoinMemberWorldserverID(memberRealmID uint32, memberGUID uint64, localRealmID uint32, localGUID uint64, localWorldserverID string) string {
	if guid.SamePlayer(memberRealmID, memberGUID, localRealmID, localGUID) {
		return localWorldserverID
	}
	return ""
}

func (s *GameSession) HandleLfgLeave(ctx context.Context, p *packet.Packet) error {
	if _, err := s.forwardPacketToWorldserverIfLfgDungeon(ctx, p); err != nil {
		return err
	}

	return s.leaveMatchmakingLfg(ctx)
}

func (s *GameSession) leaveMatchmakingLfg(ctx context.Context) error {
	if s.matchmakingServiceClient == nil || s.character == nil {
		return nil
	}

	_, err := s.matchmakingServiceClient.LeaveLfg(ctx, &pbMM.LeaveLfgRequest{
		Api:        root.SupportedMatchmakingServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
	})
	if err == nil {
		if clearErr := s.clearUnboundLfgDungeonRoute(ctx, 0); clearErr != nil && s.logger != nil {
			s.logger.Error().Err(clearErr).Uint64("character", s.character.GUID).Msg("failed to clear unbound LFG dungeon route")
		}
		s.clearLfgDungeonTransport()
	}
	return err
}

func (s *GameSession) HandleLfgSetRoles(ctx context.Context, p *packet.Packet) error {
	roles := p.Reader().Uint8()
	_, err := s.matchmakingServiceClient.SetLfgRoles(ctx, &pbMM.SetLfgRolesRequest{
		Api:        root.SupportedMatchmakingServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		Roles:      uint32(roles),
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *GameSession) HandleLfgProposalResult(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	proposalID := r.Uint32()
	accept := r.Uint8() != 0

	_, err := s.matchmakingServiceClient.AnswerLfgProposal(ctx, &pbMM.AnswerLfgProposalRequest{
		Api:        root.SupportedMatchmakingServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		ProposalID: proposalID,
		Accept:     accept,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *GameSession) HandleLfgGetStatus(ctx context.Context, p *packet.Packet) error {
	if s.lfgDungeonActive && s.worldSocket != nil {
		s.sendNativeLfgStatusRefresh(ctx)
		s.forwardLfgPacketToWorldserver(p)
		return nil
	}

	res, err := s.matchmakingServiceClient.LfgStatus(ctx, &pbMM.LfgStatusRequest{
		Api:        root.SupportedMatchmakingServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
	})
	if err != nil {
		return err
	}

	if lfgProtoStateUsesNativeDungeon(res.GetStatus().GetState()) && s.worldSocket != nil {
		s.lfgDungeonActive = true
		status := lfgStatusPayloadFromProto(res.Status)
		if err := s.persistLfgDungeonRouteFromStatus(ctx, status); err != nil {
			return err
		}
		s.trackLfgStatus(status)
		s.sendLfgStatusRefresh(status)
		s.forwardLfgPacketToWorldserver(p)
		return nil
	}

	status := lfgStatusPayloadFromProto(res.Status)
	if err := s.persistOrClearLfgDungeonRouteFromStatus(ctx, status); err != nil {
		return err
	}
	s.trackLfgStatus(status)
	s.sendLfgStatusRefresh(status)
	return nil
}

func (s *GameSession) HandleLfgTeleport(ctx context.Context, p *packet.Packet) error {
	if s.gameServerGRPCClient == nil || s.character == nil {
		s.forwardLfgPacketToWorldserver(p)
		return nil
	}

	out := p.Reader().Uint8() != 0
	out = s.normalizeLfgTeleportDirection(out)
	playerGUID := s.lfgControlPlayerGUID()
	dungeonEntry, transferRouting := s.lfgTeleportDungeonEntry(ctx, out)
	var routing *mapTransferRouting
	if !out && dungeonEntry != 0 {
		routing = transferRouting
	}
	err := s.startLfgNativeWorldportTransport(ctx, lfgNativeWorldportRequest{
		operation:    "LFG teleport",
		playerGUID:   playerGUID,
		out:          out,
		dungeonEntry: dungeonEntry,
		routing:      routing,
	})
	if errors.Is(err, errLfgNativeWorldportNoHandler) {
		s.forwardLfgPacketToWorldserver(p)
		return nil
	}
	return err
}

func (s *GameSession) InterceptLfgTeleportDenied(ctx context.Context, p *packet.Packet) error {
	s.clearPendingMapTransferRouting()
	if s.teleportingToNewMap == nil {
		s.clearActiveMapTransferRouting()
	}
	s.gameSocket.SendPacket(p)
	return nil
}

func (s *GameSession) InterceptLfgUpdateParty(ctx context.Context, p *packet.Packet) error {
	s.syncNativeLfgMemberLeaveFromPartyUpdate(ctx, p)
	s.gameSocket.SendPacket(p)
	return nil
}

func (s *GameSession) syncNativeLfgMemberLeaveFromPartyUpdate(ctx context.Context, p *packet.Packet) {
	if s == nil || s.character == nil || s.groupServiceClient == nil || !s.lfgDungeonActive || p == nil || p.Source != packet.SourceWorldServer {
		return
	}

	r := p.Reader()
	updateType := r.Uint8()
	if err := r.Error(); err != nil {
		if s.logger != nil {
			s.logger.Warn().Err(err).Msg("failed to parse native LFG party update")
		}
		return
	}
	if updateType != lfgUpdateTypeLeaderUnk1 {
		return
	}

	_, err := s.groupServiceClient.Leave(ctx, &pbGroup.GroupLeaveParams{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil && s.logger != nil {
		s.logger.Warn().Err(err).Uint64("playerGUID", s.character.GUID).Msg("failed to mirror native LFG member removal to groupservice")
	}
}

type lfgNativeWorldportRequest struct {
	operation    string
	playerGUID   uint64
	out          bool
	dungeonEntry uint32
	routing      *mapTransferRouting
}

func (s *GameSession) startLfgNativeWorldportTransport(ctx context.Context, req lfgNativeWorldportRequest) error {
	if s.gameServerGRPCClient == nil {
		return fmt.Errorf("source worldserver grpc client is nil")
	}

	return s.startClusterNativeWorldportTransport(ctx, clusterNativeWorldportTransport{
		feature:                         clusterTransferFeatureLFG,
		operation:                       req.operation,
		playerGUID:                      req.playerGUID,
		routing:                         req.routing,
		reloadManagedGroupAfterTransfer: req.routing != nil,
		start: func(ctx context.Context) error {
			res, err := s.gameServerGRPCClient.TeleportLfgPlayer(ctx, &pbGameServ.TeleportLfgPlayerRequest{
				Api:          root.SupportedGameServerVer,
				PlayerGUID:   req.playerGUID,
				Out:          req.out,
				DungeonEntry: req.dungeonEntry,
			})
			if err != nil {
				return err
			}
			return lfgNativeWorldportResponseError(req, res)
		},
	})
}

func lfgNativeWorldportResponseError(req lfgNativeWorldportRequest, res *pbGameServ.TeleportLfgPlayerResponse) error {
	if res == nil {
		return fmt.Errorf("%s for player %d returned nil response", req.operation, req.playerGUID)
	}

	switch res.GetStatus() {
	case pbGameServ.TeleportLfgPlayerResponse_Success:
		return nil
	case pbGameServ.TeleportLfgPlayerResponse_NoHandler:
		return fmt.Errorf("%w for player %d", errLfgNativeWorldportNoHandler, req.playerGUID)
	case pbGameServ.TeleportLfgPlayerResponse_PlayerNotFound:
		return fmt.Errorf("source worldserver cannot find LFG player %d", req.playerGUID)
	default:
		return fmt.Errorf("%s for player %d returned status %s", req.operation, req.playerGUID, res.GetStatus().String())
	}
}

func (s *GameSession) HandleLfgSetBootVote(ctx context.Context, p *packet.Packet) error {
	if s.gameServerGRPCClient == nil || s.character == nil {
		s.forwardLfgPacketToWorldserver(p)
		return nil
	}

	agree := p.Reader().Uint8() != 0
	playerGUID := s.lfgControlPlayerGUID()
	res, err := s.gameServerGRPCClient.SetLfgBootVote(ctx, &pbGameServ.SetLfgBootVoteRequest{
		Api:        root.SupportedGameServerVer,
		PlayerGUID: playerGUID,
		Agree:      agree,
	})
	if err != nil {
		return err
	}
	if res == nil {
		return fmt.Errorf("lfg boot vote for player %d returned nil response", playerGUID)
	}

	switch res.GetStatus() {
	case pbGameServ.SetLfgBootVoteResponse_Success:
		return nil
	case pbGameServ.SetLfgBootVoteResponse_NoHandler:
		s.forwardLfgPacketToWorldserver(p)
		return nil
	case pbGameServ.SetLfgBootVoteResponse_PlayerNotFound:
		return fmt.Errorf("lfg boot vote player %d not found", playerGUID)
	default:
		return fmt.Errorf("lfg boot vote for player %d returned status %s", playerGUID, res.GetStatus().String())
	}
}

func (s *GameSession) HandleLfgPlayerLockInfoRequest(ctx context.Context, p *packet.Packet) error {
	if s.gameServerGRPCClient == nil || s.character == nil {
		s.forwardLfgPacketToWorldserver(p)
		return nil
	}

	playerGUID := s.lfgControlPlayerGUID()
	res, err := s.getLfgPlayerInfo(ctx, playerGUID)
	if err != nil {
		return err
	}
	if res.GetStatus() == pbGameServ.GetLfgPlayerInfoResponse_PlayerNotFound {
		localGUID := guid.PlayerLowGUID(playerGUID)
		if localGUID != 0 && localGUID != playerGUID {
			localRes, localErr := s.getLfgPlayerInfo(ctx, localGUID)
			if localErr != nil {
				return localErr
			}
			if localRes.GetStatus() != pbGameServ.GetLfgPlayerInfoResponse_PlayerNotFound {
				playerGUID = localGUID
				res = localRes
			}
		}
	}

	switch res.GetStatus() {
	case pbGameServ.GetLfgPlayerInfoResponse_Success:
		s.sendLfgPlayerInfo(res)
		return nil
	case pbGameServ.GetLfgPlayerInfoResponse_NoHandler:
		s.forwardLfgPacketToWorldserver(p)
		return nil
	case pbGameServ.GetLfgPlayerInfoResponse_PlayerNotFound:
		return fmt.Errorf("lfg player info player %d not found", playerGUID)
	default:
		return fmt.Errorf("lfg player info for player %d returned status %s", playerGUID, res.GetStatus().String())
	}
}

func (s *GameSession) getLfgPlayerInfo(ctx context.Context, playerGUID uint64) (*pbGameServ.GetLfgPlayerInfoResponse, error) {
	res, err := s.gameServerGRPCClient.GetLfgPlayerInfo(ctx, &pbGameServ.GetLfgPlayerInfoRequest{
		Api:        root.SupportedGameServerVer,
		PlayerGUID: playerGUID,
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("lfg player info for player %d returned nil response", playerGUID)
	}
	return res, nil
}

func (s *GameSession) HandleLfgPartyLockInfoRequest(ctx context.Context, p *packet.Packet) error {
	if s.groupServiceClient == nil || s.gameServerGRPCClient == nil {
		s.forwardLfgPacketToWorldserver(p)
		return nil
	}

	groupResp, err := s.groupServiceClient.GetGroupByMember(ctx, &pbGroup.GetGroupByMemberRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return err
	}
	if groupResp == nil || groupResp.Group == nil {
		s.forwardLfgPacketToWorldserver(p)
		return nil
	}

	groupRealmID := lfgMemberRealmID(groupHomeRealmIDFromPB(groupResp.Group))
	memberGUIDs := make([]uint64, 0, len(groupResp.Group.Members))
	memberRealms := make(map[uint64]uint32, len(groupResp.Group.Members))
	memberLows := make(map[uint64]uint64, len(groupResp.Group.Members))
	for _, member := range groupResp.Group.Members {
		memberRealmID := groupMemberRealmID(groupRealmID, member)
		memberLowGUID := guid.PlayerLowGUID(member.GetGuid())
		if guid.SamePlayer(memberRealmID, member.GetGuid(), root.RealmID, s.character.GUID) || !member.GetIsOnline() {
			continue
		}
		memberServiceGUID := lfgMemberServiceGUID(memberRealmID, memberLowGUID)
		memberGUIDs = append(memberGUIDs, memberServiceGUID)
		memberRealms[memberServiceGUID] = memberRealmID
		memberLows[memberServiceGUID] = memberLowGUID
	}
	if len(memberGUIDs) == 0 {
		s.sendLfgPartyInfo(nil)
		return nil
	}

	placements := map[uint64]*pbGroup.MemberPlacement{}
	placementResp, err := s.groupServiceClient.GetMemberPlacements(ctx, &pbGroup.GetMemberPlacementsRequest{
		Api:         root.SupportedGroupServiceVer,
		RealmID:     root.RealmID,
		MemberGUIDs: memberGUIDs,
	})
	if err != nil {
		return err
	}
	for _, placement := range placementResp.GetPlacements() {
		placements[placement.GetMemberGUID()] = placement
	}

	memberLocks := make([]lfgPartyMemberLocks, 0, len(memberGUIDs))
	for _, memberServiceGUID := range memberGUIDs {
		memberRealmID := memberRealms[memberServiceGUID]
		memberLowGUID := memberLows[memberServiceGUID]
		client := s.gameServerGRPCClient
		checkPlayerGUID := s.lfgPlayerGUIDForCurrentWorldserver(memberRealmID, memberLowGUID)
		if placement := placements[memberServiceGUID]; placement != nil {
			if !placement.GetFresh() {
				if memberRealmID != root.RealmID {
					continue
				}
			} else if !placement.GetOnline() {
				continue
			} else if placement.GetWorldserverID() != "" && placement.GetWorldserverID() != s.worldserverID {
				var server *pbServ.GameServerDetailed
				client, server, err = s.lfgGameServerClientAndServerByWorldserverID(ctx, memberRealmID, placement.GetWorldserverID())
				if err != nil {
					return err
				}
				if lfgGameServerIsCrossRealm(server) {
					checkPlayerGUID = guid.PlayerGUIDForRealm(0, memberRealmID, memberLowGUID)
				}
			} else if placement.GetWorldserverID() == "" && memberRealmID != root.RealmID {
				continue
			}
		} else if memberRealmID != root.RealmID {
			continue
		}

		res, err := s.lfgPlayerLockInfo(ctx, checkPlayerGUID, nil, client)
		if err != nil {
			return err
		}
		if res == nil || res.GetStatus() == pbGameServ.GetLfgPlayerLockInfoResponse_PlayerNotFound {
			continue
		}
		if res.GetStatus() != pbGameServ.GetLfgPlayerLockInfoResponse_Success {
			return fmt.Errorf("lfg party lock info for player %d returned status %s", memberLowGUID, res.GetStatus().String())
		}

		memberLocks = append(memberLocks, lfgPartyMemberLocks{
			realmID:    memberRealmID,
			playerGUID: memberLowGUID,
			locks:      res.GetLocks(),
		})
	}

	s.sendLfgPartyInfo(memberLocks)
	return nil
}

type lfgPartyMemberLocks struct {
	realmID    uint32
	playerGUID uint64
	locks      []*pbGameServ.LfgDungeonLock
}

func (s *GameSession) lfgPlayerLockInfo(ctx context.Context, playerGUID uint64, dungeons []uint32, client pbGameServ.WorldServerServiceClient) (*pbGameServ.GetLfgPlayerLockInfoResponse, error) {
	if client == nil {
		return nil, nil
	}

	return client.GetLfgPlayerLockInfo(ctx, &pbGameServ.GetLfgPlayerLockInfoRequest{
		Api:            root.SupportedGameServerVer,
		PlayerGUID:     playerGUID,
		DungeonEntries: dungeons,
	})
}

func (s *GameSession) sendLfgPartyInfo(members []lfgPartyMemberLocks) {
	resp := packet.NewWriter(packet.SMsgLFGPartyInfo)
	resp.Uint8(uint8(len(members)))
	for _, member := range members {
		resp.Uint64(playerObjectGUIDForRealm(lfgMemberRealmID(member.realmID), member.playerGUID))
		resp.Uint32(uint32(len(member.locks)))
		for _, lock := range member.locks {
			resp.Uint32(lock.GetDungeonEntry())
			resp.Uint32(lock.GetLockStatus())
		}
	}
	s.gameSocket.Send(resp)
}

func (s *GameSession) sendLfgPlayerInfo(info *pbGameServ.GetLfgPlayerInfoResponse) {
	resp := packet.NewWriter(packet.SMsgLFGPlayerInfo)
	resp.Uint8(uint8(len(info.GetRandomDungeons())))
	for _, dungeon := range info.GetRandomDungeons() {
		resp.Uint32(dungeon.GetDungeonEntry())
		resp.Uint8(boolToUint8(dungeon.GetDone()))
		resp.Uint32(dungeon.GetRewardMoney())
		resp.Uint32(dungeon.GetRewardXP())
		resp.Uint32(dungeon.GetRewardUnknown1())
		resp.Uint32(dungeon.GetRewardUnknown2())
		resp.Uint8(uint8(len(dungeon.GetRewardItems())))
		for _, item := range dungeon.GetRewardItems() {
			resp.Uint32(item.GetItemID())
			resp.Uint32(item.GetDisplayID())
			resp.Uint32(item.GetCount())
		}
	}
	resp.Uint32(uint32(len(info.GetLocks())))
	for _, lock := range info.GetLocks() {
		resp.Uint32(lock.GetDungeonEntry())
		resp.Uint32(lock.GetLockStatus())
	}
	s.gameSocket.Send(resp)
}

func (s *GameSession) forwardLfgPacketToWorldserver(p *packet.Packet) {
	if s.worldSocket != nil && p != nil {
		s.worldSocket.WriteChannel() <- p
	}
}

func (s *GameSession) forwardPacketToWorldserverIfLfgDungeon(ctx context.Context, p *packet.Packet) (bool, error) {
	if s.worldSocket == nil || s.character == nil || p == nil {
		return false, nil
	}

	if s.lfgDungeonActive {
		s.forwardLfgPacketToWorldserver(p)
		return true, nil
	}

	if s.matchmakingServiceClient == nil {
		return false, nil
	}

	status, err := s.matchmakingServiceClient.LfgStatus(ctx, &pbMM.LfgStatusRequest{
		Api:        root.SupportedMatchmakingServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
	})
	if err != nil || status == nil || !lfgProtoStateUsesNativeDungeon(status.GetStatus().GetState()) {
		return false, nil
	}

	s.lfgDungeonActive = true
	s.forwardLfgPacketToWorldserver(p)
	return true, nil
}

func (s *GameSession) HandleEventMMLfgStatusChanged(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.MatchmakingEventLfgStatusChangedPayload)
	previousStatus := s.currentLfgMatchmakingStatus()
	wasDungeonActive := s.lfgDungeonActive
	s.sendLfgStatus(previousStatus, eventData.Status, wasDungeonActive)
	if err := s.persistOrClearLfgDungeonRouteFromStatus(ctx, eventData.Status); err != nil {
		return err
	}
	s.trackLfgStatus(eventData.Status)
	return nil
}

func (s *GameSession) HandleEventMMLfgProposalAccepted(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.MatchmakingEventLfgProposalAcceptedPayload)
	if eventData == nil || eventData.LeaderWorldserverID == "" || s.character == nil {
		return nil
	}
	if !lfgProposalAcceptedTargetsPlayer(eventData, s.character.GUID) {
		return nil
	}

	opCtx, cancel := s.lfgProposalAcceptedOperationContext(ctx)
	defer cancel()

	targetServer, dungeonInfo, err := s.lfgProposalAcceptedTargetServer(opCtx, eventData)
	if err != nil {
		return err
	}
	dungeonMapID := dungeonInfo.GetMapID()
	effectiveDungeonEntry := lfgEffectiveDungeonEntry(eventData.DungeonEntry, dungeonInfo)
	targetWorldserverID := targetServer.GetId()
	if targetWorldserverID == "" {
		return fmt.Errorf("can't redirect LFG member %d: target worldserver is empty", s.character.GUID)
	}

	transferRouting := &mapTransferRouting{
		realmID:      lfgMaterializationRealmID(eventData),
		isCrossRealm: eventData.CrossRealm || targetServer.GetIsCrossRealm(),
		feature:      clusterTransferFeatureLFG,
	}
	s.rememberLfgDungeonTransport(effectiveDungeonEntry, transferRouting, dungeonMapID, dungeonInfo.GetDifficulty())
	if err := s.recordLfgDungeonRoute(opCtx, effectiveDungeonEntry, dungeonMapID, dungeonInfo.GetDifficulty(), transferRouting, true); err != nil {
		return err
	}

	loginPlayerGUID := mapTransferLoginPlayerGUID(s.character.GUID, transferRouting)
	if s.worldserverID == targetWorldserverID {
		return s.startClusterOwnerNativeWorldportTransport(opCtx, clusterOwnerNativeWorldportTransport{
			feature:                         clusterTransferFeatureLFG,
			operation:                       "accepted LFG dungeon transport",
			sessionPlayerGUID:               s.character.GUID,
			loginPlayerGUID:                 loginPlayerGUID,
			targetWorldserverID:             targetWorldserverID,
			routing:                         transferRouting,
			reloadManagedGroupAfterTransfer: true,
		})
	}
	if s.worldSocket == nil {
		return fmt.Errorf("can't redirect LFG member %d to worldserver %q: world socket is nil", s.character.GUID, targetWorldserverID)
	}
	targetAddress := targetServer.GetAddress()
	if targetAddress == "" {
		return fmt.Errorf("worldserver %q has empty address", targetWorldserverID)
	}
	if s.worldSocket.Address() == targetAddress {
		return s.startClusterOwnerNativeWorldportTransport(opCtx, clusterOwnerNativeWorldportTransport{
			feature:                         clusterTransferFeatureLFG,
			operation:                       "accepted LFG dungeon transport",
			sessionPlayerGUID:               s.character.GUID,
			loginPlayerGUID:                 loginPlayerGUID,
			targetAddress:                   targetAddress,
			targetWorldserverID:             targetWorldserverID,
			routing:                         transferRouting,
			reloadManagedGroupAfterTransfer: true,
		})
	}

	var targetGRPCClient pbGameServ.WorldServerServiceClient
	if s.gameServerGRPCConnMgr != nil {
		s.gameServerGRPCConnMgr.AddAddressMapping(targetAddress, targetServer.GetGrpcAddress())
		targetGRPCClient, err = s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(targetAddress)
		if err != nil {
			return fmt.Errorf("can't resolve LFG target worldserver %q grpc client: %w", targetWorldserverID, err)
		}
	}

	return s.startClusterOwnerNativeWorldportTransport(opCtx, clusterOwnerNativeWorldportTransport{
		feature:                         clusterTransferFeatureLFG,
		operation:                       "accepted LFG dungeon transport",
		sessionPlayerGUID:               s.character.GUID,
		loginPlayerGUID:                 loginPlayerGUID,
		targetAddress:                   targetAddress,
		targetWorldserverID:             targetWorldserverID,
		routing:                         transferRouting,
		reloadManagedGroupAfterTransfer: true,
		onOwnerPlaced: func(context.Context) error {
			if targetGRPCClient != nil {
				s.gameServerGRPCClient = targetGRPCClient
			}
			return nil
		},
	})
}

func (s *GameSession) lfgProposalAcceptedOperationContext(parent context.Context) (context.Context, context.CancelFunc) {
	base := s.ctx
	if base == nil {
		base = parent
	}
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, lfgProposalAcceptedTimeout)
}

func (s *GameSession) lfgProposalAcceptedTargetServer(ctx context.Context, eventData *events.MatchmakingEventLfgProposalAcceptedPayload) (*pbServ.Server, *pbGameServ.GetLfgDungeonInfoResponse, error) {
	if s.gameServerGRPCClient == nil {
		return nil, nil, fmt.Errorf("can't resolve LFG dungeon owner: worldserver grpc client is nil")
	}
	if s.serversRegistryClient == nil {
		return nil, nil, fmt.Errorf("can't resolve LFG dungeon owner: servers registry client is nil")
	}

	dungeonInfo, err := s.gameServerGRPCClient.GetLfgDungeonInfo(ctx, &pbGameServ.GetLfgDungeonInfoRequest{
		Api:          root.SupportedGameServerVer,
		DungeonEntry: eventData.DungeonEntry,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("get LFG dungeon info for proposal %d failed: %w", eventData.ProposalID, err)
	}
	if dungeonInfo == nil {
		return nil, nil, fmt.Errorf("get LFG dungeon info for proposal %d returned nil response", eventData.ProposalID)
	}
	if dungeonInfo.GetStatus() != pbGameServ.GetLfgDungeonInfoResponse_Success {
		return nil, nil, fmt.Errorf("get LFG dungeon info for proposal %d returned status %s", eventData.ProposalID, dungeonInfo.GetStatus())
	}

	servers, err := s.serversRegistryClient.AvailableGameServersForMapAndRealm(ctx, &pbServ.AvailableGameServersForMapAndRealmRequest{
		Api:          root.SupportedServerRegistryVer,
		RealmID:      lfgMaterializationRealmID(eventData),
		MapID:        dungeonInfo.GetMapID(),
		IsCrossRealm: eventData.CrossRealm,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list game servers for LFG dungeon map %d in realm %d crossrealm=%t failed: %w", dungeonInfo.GetMapID(), lfgMaterializationRealmID(eventData), eventData.CrossRealm, err)
	}
	if servers == nil {
		return nil, nil, fmt.Errorf("list game servers for LFG dungeon map %d in realm %d crossrealm=%t returned nil response", dungeonInfo.GetMapID(), lfgMaterializationRealmID(eventData), eventData.CrossRealm)
	}

	target := chooseLFGProposalTargetServer(servers.GetGameServers(), eventData.LeaderWorldserverID)
	if target == nil {
		return nil, nil, fmt.Errorf("no worldserver available for LFG dungeon map %d in realm %d crossrealm=%t", dungeonInfo.GetMapID(), lfgMaterializationRealmID(eventData), eventData.CrossRealm)
	}
	return target, dungeonInfo, nil
}

func chooseLFGProposalTargetServer(servers []*pbServ.Server, preferredWorldserverID string) *pbServ.Server {
	for _, server := range servers {
		if server.GetId() == preferredWorldserverID {
			return server
		}
	}

	candidates := make([]*pbServ.Server, 0, len(servers))
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

func lfgProposalAcceptedTargetsPlayer(payload *events.MatchmakingEventLfgProposalAcceptedPayload, playerGUID uint64) bool {
	if payload != nil && len(payload.Members) > 0 {
		for _, member := range payload.Members {
			if lfgMemberRealmID(member.RealmID) == root.RealmID && uint64(member.PlayerGUID) == playerGUID {
				return true
			}
		}
		return false
	}
	for _, guid := range payload.PlayersGUID {
		if uint64(guid) == playerGUID {
			return true
		}
	}
	return false
}

func lfgMaterializationRealmID(payload *events.MatchmakingEventLfgProposalAcceptedPayload) uint32 {
	if payload != nil && payload.CrossRealm {
		return 0
	}
	if payload == nil {
		return root.RealmID
	}
	return payload.RealmID
}

func (s *GameSession) lfgControlPlayerGUID() uint64 {
	if s.character == nil {
		return 0
	}
	if s.lfgUsesCrossrealmOwner() {
		return guid.PlayerGUIDForRealm(0, root.RealmID, s.character.GUID)
	}
	return s.character.GUID
}

func (s *GameSession) lfgUsesCrossrealmOwner() bool {
	return mapTransferRoutingUsesCrossrealmOwner(s.currentMapTransferRouting) ||
		mapTransferRoutingUsesCrossrealmOwner(s.activeMapTransferRouting) ||
		mapTransferRoutingUsesCrossrealmOwner(s.pendingMapTransferRouting)
}

func (s *GameSession) normalizeLfgTeleportDirection(out bool) bool {
	if out || !s.lfgDungeonActive || !s.lfgUsesCrossrealmOwner() {
		return out
	}

	if s.logger != nil {
		s.logger.Debug().
			Uint32("accountID", s.accountID).
			Uint64("playerGUID", s.character.GUID).
			Msg("treating stale LFG dungeon enter request as dungeon exit")
	}
	return true
}

func (s *GameSession) sendLfgStatus(previousStatus, status events.MatchmakingLfgStatusPayload, wasDungeonActive bool) {
	s.lfgDungeonActive = lfgStateUsesNativeDungeon(status.State)

	switch status.State {
	case events.MatchmakingLfgStateNone:
		s.sendLfgNoneStatus(previousStatus, wasDungeonActive)
	case events.MatchmakingLfgStateRoleCheck:
		s.sendLfgRoleCheckUpdate(status)
	case events.MatchmakingLfgStateQueued:
		s.sendLfgJoinResult(lfgJoinOK, lfgRolecheckDefault)
		if s.lfgQueuedStatusIsParty(status) {
			s.sendLfgUpdateParty(lfgUpdateTypeAddedToQueue, true, true, status.SelectedDungeons)
		} else {
			s.sendLfgUpdatePlayer(lfgUpdateTypeJoinQueue, true, status.SelectedDungeons)
		}
		s.sendLfgQueueStatus(status)
	case events.MatchmakingLfgStateProposal:
		if status.ProposalState == events.MatchmakingLfgProposalFailed {
			s.sendLfgProposalUpdate(status)
			s.sendLfgProposalFailureStatus(status)
			return
		}
		s.lastLfgProposalSuccessID = 0
		if s.lfgQueuedStatusIsParty(status) {
			s.sendLfgUpdateParty(lfgUpdateTypeProposalBegin, true, false, status.SelectedDungeons)
		} else {
			s.sendLfgUpdatePlayer(lfgUpdateTypeProposalBegin, false, status.SelectedDungeons)
		}
		s.sendLfgProposalUpdate(status)
	case events.MatchmakingLfgStateDungeon:
		if status.ProposalID == 0 || s.lastLfgProposalSuccessID == status.ProposalID {
			return
		}
		s.lastLfgProposalSuccessID = status.ProposalID
		s.sendLfgProposalUpdate(status)
		if s.lfgQueuedStatusIsParty(status) {
			s.sendLfgUpdateParty(lfgUpdateTypeGroupFound, false, false, nil)
		} else {
			s.sendLfgUpdatePlayer(lfgUpdateTypeGroupFound, false, nil)
		}
		s.sendLfgUpdatePlayer(lfgUpdateTypeRemovedFromQueue, false, nil)
		s.sendLfgUpdateParty(lfgUpdateTypeRemovedFromQueue, false, false, nil)
		s.sendLfgStatusRefresh(status)
	case events.MatchmakingLfgStateFinishedDungeon:
		return
	}
}

func (s *GameSession) currentLfgMatchmakingStatus() events.MatchmakingLfgStatusPayload {
	if s.character == nil {
		return events.MatchmakingLfgStatusPayload{}
	}
	return s.character.lastLfgStatus
}

func (s *GameSession) sendLfgNoneStatus(previousStatus events.MatchmakingLfgStatusPayload, wasDungeonActive bool) {
	if wasDungeonActive {
		return
	}

	switch previousStatus.State {
	case events.MatchmakingLfgStateDungeon, events.MatchmakingLfgStateFinishedDungeon:
		return
	case events.MatchmakingLfgStateRoleCheck:
		s.sendLfgUpdateParty(lfgUpdateTypeRolecheckAborted, false, false, nil)
	case events.MatchmakingLfgStateQueued:
		if s.lfgQueuedStatusIsParty(previousStatus) {
			s.sendLfgUpdateParty(lfgUpdateTypeRemovedFromQueue, false, false, nil)
		} else {
			s.sendLfgUpdatePlayer(lfgUpdateTypeRemovedFromQueue, false, nil)
		}
	case events.MatchmakingLfgStateProposal, events.MatchmakingLfgStateBoot:
		s.sendLfgUpdateParty(lfgUpdateTypeProposalFailed, false, false, nil)
	default:
		s.sendLfgUpdatePlayer(lfgUpdateTypeRemovedFromQueue, false, nil)
		s.sendLfgUpdateParty(lfgUpdateTypeRolecheckAborted, false, false, nil)
	}
}

func (s *GameSession) sendLfgProposalFailureStatus(status events.MatchmakingLfgStatusPayload) {
	localLeader, ok := s.lfgLocalQueueLeader(status)
	if !ok {
		s.sendLfgUpdateParty(lfgProposalFailureUpdateType(status), false, false, nil)
		return
	}

	removedLeaders := lfgProposalRemovedQueueLeaders(status)
	if _, removed := removedLeaders[localLeader]; removed {
		updateType := lfgUpdateTypeRemovedFromQueue
		if s.lfgLocalProposalMemberFailed(status) {
			updateType = lfgProposalFailureUpdateType(status)
		}
		if s.lfgQueuedStatusIsParty(status) {
			s.sendLfgUpdateParty(updateType, false, false, nil)
		} else {
			s.sendLfgUpdatePlayer(updateType, false, nil)
		}
		return
	}

	if s.lfgQueuedStatusIsParty(status) {
		s.sendLfgUpdateParty(lfgUpdateTypeAddedToQueue, true, true, status.SelectedDungeons)
		return
	}
	s.sendLfgUpdatePlayer(lfgUpdateTypeAddedToQueue, true, status.SelectedDungeons)
}

func (s *GameSession) trackLfgPendingJoin() {
	if s.character == nil {
		return
	}

	s.character.lfgPendingJoin = true
	s.clearLfgDungeonTransport()
}

func (s *GameSession) trackLfgStatus(status events.MatchmakingLfgStatusPayload) {
	if s.character == nil {
		return
	}

	s.character.lfgPendingJoin = false
	if lfgStateUsesNativeDungeon(status.State) {
		s.rememberLfgDungeonTransport(status.DungeonEntry, lfgStatusTransportRouting(status))
		s.character.lfgMatchmakingActive = false
		s.character.lastLfgStatus = status
		return
	}
	if status.State == events.MatchmakingLfgStateNone {
		s.clearLfgDungeonTransport()
	}
	if lfgMatchmakingStatusIsActive(status.State) {
		s.character.lfgMatchmakingActive = true
		s.character.lastLfgStatus = status
		return
	}

	s.clearLfgMatchmakingTracking()
}

func (s *GameSession) sendNativeLfgStatusRefresh(ctx context.Context) {
	if s.gameSocket == nil {
		return
	}

	if status, ok := s.currentNativeLfgStatus(); ok {
		s.sendLfgStatusRefresh(status)
		return
	}

	if s.matchmakingServiceClient == nil || s.character == nil {
		return
	}

	res, err := s.matchmakingServiceClient.LfgStatus(ctx, &pbMM.LfgStatusRequest{
		Api:        root.SupportedMatchmakingServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
	})
	if err != nil {
		return
	}

	status := lfgStatusPayloadFromProto(res.GetStatus())
	if !lfgStateUsesNativeDungeon(status.State) {
		return
	}

	s.trackLfgStatus(status)
	s.sendLfgStatusRefresh(status)
}

func (s *GameSession) currentNativeLfgStatus() (events.MatchmakingLfgStatusPayload, bool) {
	if s.character == nil {
		return events.MatchmakingLfgStatusPayload{}, false
	}

	status := s.character.lastLfgStatus
	if !lfgStateUsesNativeDungeon(status.State) || len(status.SelectedDungeons) == 0 {
		return events.MatchmakingLfgStatusPayload{}, false
	}

	return status, true
}

func (s *GameSession) clearLfgMatchmakingTracking() {
	if s.character == nil {
		return
	}

	s.character.lfgPendingJoin = false
	s.character.lfgMatchmakingActive = false
	s.character.lastLfgStatus = events.MatchmakingLfgStatusPayload{}
}

func (s *GameSession) rememberLfgDungeonTransport(dungeonEntry uint32, routing *mapTransferRouting, mapAndDifficulty ...uint32) {
	if s.character == nil || dungeonEntry == 0 {
		return
	}

	var mapID uint32
	var difficulty uint32
	if len(mapAndDifficulty) > 0 {
		mapID = mapAndDifficulty[0]
	}
	if len(mapAndDifficulty) > 1 {
		difficulty = mapAndDifficulty[1]
	}
	if current := s.character.lfgDungeonTransport; current != nil {
		if routing == nil {
			routing = current.routing
		}
		if mapID == 0 && current.dungeonEntry == dungeonEntry {
			mapID = current.mapID
		}
		if difficulty == 0 && current.dungeonEntry == dungeonEntry {
			difficulty = current.difficulty
		}
	}

	s.character.lfgDungeonTransport = &lfgDungeonTransportState{
		dungeonEntry: dungeonEntry,
		mapID:        mapID,
		difficulty:   difficulty,
		routing:      cloneMapTransferRouting(routing),
	}
}

func (s *GameSession) persistOrClearLfgDungeonRouteFromStatus(ctx context.Context, status events.MatchmakingLfgStatusPayload) error {
	if status.State == events.MatchmakingLfgStateNone {
		if s != nil && s.character != nil && s.character.lfgDungeonTransport != nil && mapTransferRoutingUsesCrossrealmOwner(s.character.lfgDungeonTransport.routing) {
			return nil
		}
		return s.clearUnboundLfgDungeonRoute(ctx, 0)
	}
	return s.persistLfgDungeonRouteFromStatus(ctx, status)
}

func (s *GameSession) persistLfgDungeonRouteFromStatus(ctx context.Context, status events.MatchmakingLfgStatusPayload) error {
	if s == nil || s.character == nil || status.DungeonEntry == 0 {
		return nil
	}
	routing := lfgStatusTransportRouting(status)
	if !mapTransferRoutingUsesCrossrealmOwner(routing) {
		return nil
	}
	info, err := s.resolveLfgDungeonInfo(ctx, status.DungeonEntry)
	if err != nil || info == nil {
		return err
	}
	dungeonEntry := lfgEffectiveDungeonEntry(status.DungeonEntry, info)
	s.rememberLfgDungeonTransport(dungeonEntry, routing, info.GetMapID(), info.GetDifficulty())
	return s.recordLfgDungeonRoute(ctx, dungeonEntry, info.GetMapID(), info.GetDifficulty(), routing, true)
}

func lfgEffectiveDungeonEntry(requestedDungeonEntry uint32, dungeonInfo *pbGameServ.GetLfgDungeonInfoResponse) uint32 {
	if dungeonInfo != nil && dungeonInfo.GetDungeonEntry() != 0 {
		return dungeonInfo.GetDungeonEntry()
	}
	return requestedDungeonEntry
}

func (s *GameSession) recordLfgDungeonRoute(ctx context.Context, dungeonEntry, mapID, difficulty uint32, routing *mapTransferRouting, requiresBoundInstance bool) error {
	if s == nil || s.character == nil || s.charServiceClient == nil || dungeonEntry == 0 || mapID == 0 || !mapTransferRoutingUsesCrossrealmOwner(routing) {
		return nil
	}
	_, err := s.charServiceClient.RecordLfgDungeonRoute(ctx, &pbChar.RecordLfgDungeonRouteRequest{
		Api: root.SupportedCharServiceVer,
		Route: &pbChar.LfgDungeonRoute{
			RealmID:               root.RealmID,
			PlayerGUID:            s.character.GUID,
			DungeonEntry:          dungeonEntry,
			MapID:                 mapID,
			Difficulty:            difficulty,
			OwnerRealmID:          routing.realmID,
			IsCrossRealm:          routing.isCrossRealm,
			RequiresBoundInstance: requiresBoundInstance,
		},
	})
	return err
}

func (s *GameSession) clearUnboundLfgDungeonRoute(ctx context.Context, mapID uint32) error {
	if s == nil || s.character == nil || s.charServiceClient == nil {
		return nil
	}
	_, err := s.charServiceClient.ClearUnboundLfgDungeonRoute(ctx, &pbChar.ClearUnboundLfgDungeonRouteRequest{
		Api:        root.SupportedCharServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		MapID:      mapID,
	})
	return err
}

func (s *GameSession) confirmLfgDungeonRouteEntered(ctx context.Context, mapID uint32) error {
	if s == nil || s.character == nil || s.charServiceClient == nil || mapID == 0 {
		return nil
	}
	if s.currentMapTransferRouting == nil || s.currentMapTransferRouting.feature != clusterTransferFeatureLFG || !mapTransferRoutingUsesCrossrealmOwner(s.currentMapTransferRouting) {
		return nil
	}
	res, err := s.charServiceClient.ConfirmLfgDungeonRouteEntered(ctx, &pbChar.ConfirmLfgDungeonRouteEnteredRequest{
		Api:        root.SupportedCharServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		MapID:      mapID,
		Difficulty: s.currentLfgDungeonRouteDifficulty(mapID),
	})
	if err != nil || res == nil || res.GetRoute() == nil {
		return err
	}
	route := res.GetRoute()
	s.rememberLfgDungeonTransport(route.GetDungeonEntry(), lfgRouteMapTransferRouting(route), route.GetMapID(), route.GetDifficulty())
	return nil
}

func (s *GameSession) currentLfgDungeonRouteDifficulty(mapID uint32) uint32 {
	if s == nil || s.character == nil || s.character.lfgDungeonTransport == nil {
		return 0
	}
	transport := s.character.lfgDungeonTransport
	if transport.mapID != 0 && mapID != 0 && transport.mapID != mapID {
		return 0
	}
	return transport.difficulty
}

func (s *GameSession) clearLfgDungeonTransport() {
	if s.character == nil {
		return
	}

	s.character.lfgDungeonTransport = nil
}

func (s *GameSession) lfgTeleportDungeonEntry(ctx context.Context, out bool) (uint32, *mapTransferRouting) {
	if out || s.character == nil {
		return 0, nil
	}

	if transport := s.character.lfgDungeonTransport; transport != nil && transport.dungeonEntry != 0 {
		return transport.dungeonEntry, cloneMapTransferRouting(transport.routing)
	}

	if s.matchmakingServiceClient == nil {
		return 0, nil
	}

	status, err := s.matchmakingServiceClient.LfgStatus(ctx, &pbMM.LfgStatusRequest{
		Api:        root.SupportedMatchmakingServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
	})
	if err != nil || status == nil {
		return 0, nil
	}

	payload := lfgStatusPayloadFromProto(status.GetStatus())
	if !lfgStateUsesNativeDungeon(payload.State) || payload.DungeonEntry == 0 {
		return 0, nil
	}

	routing := lfgStatusTransportRouting(payload)
	s.rememberLfgDungeonTransport(payload.DungeonEntry, routing)
	return payload.DungeonEntry, routing
}

func (s *GameSession) lfgRoutingForNativeWorldport(ctx context.Context, mapID uint32) (*mapTransferRouting, bool, error) {
	if s == nil || s.character == nil || mapID == 0 {
		return nil, false, nil
	}

	transport := s.character.lfgDungeonTransport
	if transport != nil && transport.dungeonEntry != 0 && mapTransferRoutingUsesCrossrealmOwner(transport.routing) {
		dungeonMapID := s.resolveLfgDungeonMapID(ctx, transport)
		if dungeonMapID == mapID {
			return cloneMapTransferRouting(transport.routing), false, nil
		}
	}

	route, blocked, err := s.lfgPersistentRouteForMap(ctx, s.character.GUID, mapID)
	if err != nil || route == nil || blocked {
		return nil, blocked, err
	}
	routing := lfgRouteMapTransferRouting(route)
	s.rememberLfgDungeonTransport(route.GetDungeonEntry(), routing, route.GetMapID(), route.GetDifficulty())
	return routing, false, nil
}

func (s *GameSession) resolveLfgDungeonMapID(ctx context.Context, transport *lfgDungeonTransportState) uint32 {
	if s == nil || transport == nil || transport.dungeonEntry == 0 {
		return 0
	}
	if transport.mapID != 0 {
		return transport.mapID
	}
	dungeonInfo, err := s.resolveLfgDungeonInfo(ctx, transport.dungeonEntry)
	if err != nil || dungeonInfo == nil || dungeonInfo.GetMapID() == 0 {
		if s.logger != nil {
			event := s.logger.Debug().
				Uint32("dungeonEntry", transport.dungeonEntry)
			if dungeonInfo != nil {
				event = event.Str("status", dungeonInfo.GetStatus().String())
			}
			event.Err(err).Msg("TC9 could not resolve LFG dungeon map for native worldport routing")
		}
		return 0
	}

	transport.mapID = dungeonInfo.GetMapID()
	transport.difficulty = dungeonInfo.GetDifficulty()
	return transport.mapID
}

func (s *GameSession) resolveLfgDungeonInfo(ctx context.Context, dungeonEntry uint32) (*pbGameServ.GetLfgDungeonInfoResponse, error) {
	if s == nil || dungeonEntry == 0 || s.gameServerGRPCClient == nil {
		return nil, nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	dungeonInfo, err := s.gameServerGRPCClient.GetLfgDungeonInfo(ctx, &pbGameServ.GetLfgDungeonInfoRequest{
		Api:          root.SupportedGameServerVer,
		DungeonEntry: dungeonEntry,
	})
	if err != nil || dungeonInfo == nil || dungeonInfo.GetStatus() != pbGameServ.GetLfgDungeonInfoResponse_Success {
		return nil, err
	}
	return dungeonInfo, nil
}

func (s *GameSession) lfgPersistentRouteForMap(ctx context.Context, playerGUID uint64, mapID uint32) (*pbChar.LfgDungeonRoute, bool, error) {
	if s == nil || s.charServiceClient == nil || playerGUID == 0 || mapID == 0 || !lfgLoginMapMayNeedPersistentRoute(mapID) {
		return nil, false, nil
	}
	res, err := s.charServiceClient.GetLfgDungeonRoute(ctx, &pbChar.GetLfgDungeonRouteRequest{
		Api:        root.SupportedCharServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: playerGUID,
		MapID:      mapID,
	})
	if err != nil || res == nil || !res.GetFound() || res.GetRoute() == nil {
		return nil, false, err
	}
	return res.GetRoute(), !res.GetAvailable(), nil
}

const (
	lfgWorldMapEasternKingdoms uint32 = 0
	lfgWorldMapKalimdor        uint32 = 1
	lfgWorldMapOutland         uint32 = 530
	lfgWorldMapNorthrend       uint32 = 571
)

func lfgLoginMapMayNeedPersistentRoute(mapID uint32) bool {
	switch mapID {
	case lfgWorldMapEasternKingdoms, lfgWorldMapKalimdor, lfgWorldMapOutland, lfgWorldMapNorthrend:
		return false
	default:
		return true
	}
}

func lfgRouteMapTransferRouting(route *pbChar.LfgDungeonRoute) *mapTransferRouting {
	if route == nil || !route.GetIsCrossRealm() {
		return nil
	}
	return &mapTransferRouting{
		realmID:      route.GetOwnerRealmID(),
		isCrossRealm: route.GetIsCrossRealm(),
		feature:      clusterTransferFeatureLFG,
	}
}

func lfgStatusTransportRouting(status events.MatchmakingLfgStatusPayload) *mapTransferRouting {
	if !lfgStateUsesNativeDungeon(status.State) || status.DungeonEntry == 0 || !lfgStatusHasMixedRealms(status) {
		return nil
	}

	return &mapTransferRouting{
		realmID:      0,
		isCrossRealm: true,
		feature:      clusterTransferFeatureLFG,
	}
}

func lfgStatusHasMixedRealms(status events.MatchmakingLfgStatusPayload) bool {
	var realmID uint32
	for _, member := range status.QueuedMembers {
		if lfgStatusMemberIntroducesForeignRealm(&realmID, member.RealmID) {
			return true
		}
	}
	for _, member := range status.ProposalMembers {
		if lfgStatusMemberIntroducesForeignRealm(&realmID, member.RealmID) {
			return true
		}
	}
	return false
}

func lfgStatusMemberIntroducesForeignRealm(current *uint32, memberRealmID uint32) bool {
	if current == nil || memberRealmID == 0 {
		return false
	}
	if *current == 0 {
		*current = memberRealmID
		return false
	}
	return *current != memberRealmID
}

func lfgMatchmakingStatusIsActive(state events.MatchmakingLfgState) bool {
	switch state {
	case events.MatchmakingLfgStateRoleCheck,
		events.MatchmakingLfgStateQueued,
		events.MatchmakingLfgStateProposal,
		events.MatchmakingLfgStateBoot:
		return true
	default:
		return false
	}
}

func (s *GameSession) clearLfgAfterMatchmakingUnavailable() bool {
	if s.character == nil || s.lfgDungeonActive {
		return false
	}
	if !s.character.lfgPendingJoin && !s.character.lfgMatchmakingActive {
		return false
	}

	status := s.character.lastLfgStatus
	s.sendLfgJoinResult(lfgJoinInternalError, lfgRolecheckDefault)
	switch status.State {
	case events.MatchmakingLfgStateRoleCheck:
		s.sendLfgUpdateParty(lfgUpdateTypeRolecheckAborted, false, false, nil)
	case events.MatchmakingLfgStateQueued:
		if s.lfgQueuedStatusIsParty(status) {
			s.sendLfgUpdateParty(lfgUpdateTypeRemovedFromQueue, false, false, nil)
		} else {
			s.sendLfgUpdatePlayer(lfgUpdateTypeRemovedFromQueue, false, nil)
		}
	case events.MatchmakingLfgStateProposal, events.MatchmakingLfgStateBoot:
		s.sendLfgUpdateParty(lfgUpdateTypeProposalFailed, false, false, nil)
		if status.ProposalID != 0 {
			status.ProposalState = events.MatchmakingLfgProposalFailed
			s.sendLfgProposalUpdate(status)
		}
	default:
		s.sendLfgUpdatePlayer(lfgUpdateTypeRemovedFromQueue, false, nil)
		s.sendLfgUpdateParty(lfgUpdateTypeRolecheckAborted, false, false, nil)
	}
	s.clearLfgMatchmakingTracking()
	return true
}

func (s *GameSession) sendLfgStatusRefresh(status events.MatchmakingLfgStatusPayload) {
	s.lfgDungeonActive = lfgStateUsesNativeDungeon(status.State)

	dungeons := status.SelectedDungeons
	if s.lfgStatusRefreshUsesParty(status) {
		join := status.State != events.MatchmakingLfgStateRoleCheck && status.State != events.MatchmakingLfgStateNone
		queued := status.State == events.MatchmakingLfgStateQueued
		s.sendLfgUpdateParty(lfgUpdateTypeUpdateStatus, join, queued, dungeons)
		s.sendLfgUpdatePlayer(lfgUpdateTypeUpdateStatus, false, nil)
		return
	}

	queued := status.State == events.MatchmakingLfgStateQueued
	s.sendLfgUpdatePlayer(lfgUpdateTypeUpdateStatus, queued, dungeons)
	s.sendLfgUpdateParty(lfgUpdateTypeUpdateStatus, false, false, nil)
}

func (s *GameSession) lfgStatusRefreshUsesParty(status events.MatchmakingLfgStatusPayload) bool {
	if lfgStateUsesNativeDungeon(status.State) {
		return true
	}

	return s.lfgQueuedStatusIsParty(status)
}

type lfgStatusPlayerKey struct {
	realmID    uint32
	playerGUID guid.LowType
}

func (s *GameSession) lfgQueuedStatusIsParty(status events.MatchmakingLfgStatusPayload) bool {
	local, found := s.lfgLocalQueuedMember(status)
	if !found {
		return false
	}

	leader := lfgQueuedMemberLeaderKey(status.QueuedMembers, local)
	if leader.playerGUID == 0 {
		return false
	}

	groupSize := 0
	for _, member := range status.QueuedMembers {
		memberLeader := lfgQueuedMemberLeaderKey(status.QueuedMembers, member)
		if memberLeader == leader {
			groupSize++
		}
	}

	return groupSize > 1
}

func (s *GameSession) lfgLocalQueuedMember(status events.MatchmakingLfgStatusPayload) (events.MatchmakingLfgMember, bool) {
	if s.character == nil {
		return events.MatchmakingLfgMember{}, false
	}
	for _, member := range status.QueuedMembers {
		if lfgMemberRealmID(member.RealmID) == root.RealmID && uint64(member.PlayerGUID) == s.character.GUID {
			return member, true
		}
	}
	return events.MatchmakingLfgMember{}, false
}

func (s *GameSession) lfgLocalQueueLeader(status events.MatchmakingLfgStatusPayload) (lfgStatusPlayerKey, bool) {
	local, found := s.lfgLocalQueuedMember(status)
	if !found {
		return lfgStatusPlayerKey{}, false
	}
	leader := lfgQueuedMemberLeaderKey(status.QueuedMembers, local)
	if leader.playerGUID == 0 {
		return lfgStatusPlayerKey{}, false
	}
	return leader, true
}

func (s *GameSession) lfgLocalProposalMemberFailed(status events.MatchmakingLfgStatusPayload) bool {
	if s.character == nil {
		return false
	}
	local := lfgStatusPlayerKey{realmID: root.RealmID, playerGUID: guid.LowType(s.character.GUID)}
	allAcceptedFailure := lfgProposalFailure(status) == events.MatchmakingLfgProposalFailureFailed &&
		!lfgProposalHasUnacceptedMember(status)
	for _, member := range status.ProposalMembers {
		if lfgProposalMemberKey(member) != local {
			continue
		}
		if allAcceptedFailure {
			return true
		}
		switch lfgProposalFailure(status) {
		case events.MatchmakingLfgProposalFailureDeclined:
			return member.Answered && !member.Accepted
		case events.MatchmakingLfgProposalFailureFailed:
			return !member.Accepted
		default:
			return false
		}
	}
	return allAcceptedFailure
}

func lfgProposalFailure(status events.MatchmakingLfgStatusPayload) events.MatchmakingLfgProposalFailure {
	if status.ProposalFailure != events.MatchmakingLfgProposalFailureNone {
		return status.ProposalFailure
	}
	for _, member := range status.ProposalMembers {
		if member.Answered && !member.Accepted {
			return events.MatchmakingLfgProposalFailureDeclined
		}
	}
	return events.MatchmakingLfgProposalFailureFailed
}

func lfgProposalFailureUpdateType(status events.MatchmakingLfgStatusPayload) uint8 {
	if lfgProposalFailure(status) == events.MatchmakingLfgProposalFailureDeclined {
		return lfgUpdateTypeProposalDeclined
	}
	return lfgUpdateTypeProposalFailed
}

func lfgProposalRemovedQueueLeaders(status events.MatchmakingLfgStatusPayload) map[lfgStatusPlayerKey]struct{} {
	res := map[lfgStatusPlayerKey]struct{}{}
	failure := lfgProposalFailure(status)
	for _, member := range status.ProposalMembers {
		queueLeader := lfgProposalMemberQueueLeader(status, member)
		switch failure {
		case events.MatchmakingLfgProposalFailureDeclined:
			if member.Answered && !member.Accepted {
				res[queueLeader] = struct{}{}
			}
		case events.MatchmakingLfgProposalFailureFailed:
			if !member.Accepted {
				res[queueLeader] = struct{}{}
			}
		}
	}
	if failure == events.MatchmakingLfgProposalFailureFailed && len(res) == 0 {
		for _, member := range status.QueuedMembers {
			res[lfgQueuedMemberLeaderKey(status.QueuedMembers, member)] = struct{}{}
		}
	}
	return res
}

func lfgProposalHasUnacceptedMember(status events.MatchmakingLfgStatusPayload) bool {
	for _, member := range status.ProposalMembers {
		if !member.Accepted {
			return true
		}
	}
	return false
}

func lfgProposalMemberQueueLeader(status events.MatchmakingLfgStatusPayload, proposalMember events.MatchmakingLfgProposalMember) lfgStatusPlayerKey {
	proposalKey := lfgProposalMemberKey(proposalMember)
	for _, queuedMember := range status.QueuedMembers {
		if lfgQueuedMemberKey(queuedMember) == proposalKey {
			return lfgQueuedMemberLeaderKey(status.QueuedMembers, queuedMember)
		}
	}
	return proposalKey
}

func lfgQueuedMemberLeaderKey(members []events.MatchmakingLfgMember, member events.MatchmakingLfgMember) lfgStatusPlayerKey {
	leaderRealmID := lfgMemberRealmID(member.QueueLeaderRealmID)
	leaderGUID := member.QueueLeaderGUID
	if leaderGUID == 0 {
		leaderRealmID, leaderGUID = lfgInferQueueLeader(members, member)
	}
	return lfgStatusPlayerKey{realmID: leaderRealmID, playerGUID: leaderGUID}
}

func lfgQueuedMemberKey(member events.MatchmakingLfgMember) lfgStatusPlayerKey {
	return lfgStatusPlayerKey{realmID: lfgMemberRealmID(member.RealmID), playerGUID: member.PlayerGUID}
}

func lfgProposalMemberKey(member events.MatchmakingLfgProposalMember) lfgStatusPlayerKey {
	return lfgStatusPlayerKey{realmID: lfgMemberRealmID(member.RealmID), playerGUID: member.PlayerGUID}
}

func lfgInferQueueLeader(members []events.MatchmakingLfgMember, member events.MatchmakingLfgMember) (uint32, guid.LowType) {
	if member.Leader {
		return lfgMemberRealmID(member.RealmID), member.PlayerGUID
	}

	var leader events.MatchmakingLfgMember
	leaderCount := 0
	for _, candidate := range members {
		if !candidate.Leader {
			continue
		}
		leader = candidate
		leaderCount++
	}
	if leaderCount == 1 {
		return lfgMemberRealmID(leader.RealmID), leader.PlayerGUID
	}

	return lfgMemberRealmID(member.RealmID), member.PlayerGUID
}

func (s *GameSession) sendLfgUpdatePlayer(updateType uint8, queued bool, dungeons []uint32) {
	resp := packet.NewWriter(packet.SMsgLFGUpdatePlayer)
	resp.Uint8(updateType)
	resp.Uint8(boolToUint8(len(dungeons) > 0))
	if len(dungeons) > 0 {
		resp.Uint8(boolToUint8(queued))
		resp.Uint8(0)
		resp.Uint8(0)
		resp.Uint8(uint8(len(dungeons)))
		for _, dungeon := range dungeons {
			resp.Uint32(lfgDungeonID(dungeon))
		}
		resp.String("")
	}
	s.gameSocket.Send(resp)
}

func (s *GameSession) sendLfgUpdateParty(updateType uint8, join bool, queued bool, dungeons []uint32) {
	resp := packet.NewWriter(packet.SMsgLFGUpdateParty)
	resp.Uint8(updateType)
	resp.Uint8(boolToUint8(len(dungeons) > 0))
	if len(dungeons) > 0 {
		resp.Uint8(boolToUint8(join))
		resp.Uint8(boolToUint8(queued))
		resp.Uint8(0)
		resp.Uint8(0)
		for i := 0; i < 3; i++ {
			resp.Uint8(0)
		}
		resp.Uint8(uint8(len(dungeons)))
		for _, dungeon := range dungeons {
			resp.Uint32(lfgDungeonID(dungeon))
		}
		resp.String("")
	}
	s.gameSocket.Send(resp)
}

func (s *GameSession) sendLfgRoleCheckUpdate(status events.MatchmakingLfgStatusPayload) {
	resp := packet.NewWriter(packet.SMsgLFGRoleCheckUpdate)
	resp.Uint32(lfgRolecheckInitialited)
	resp.Uint8(1)
	resp.Uint8(uint8(len(status.SelectedDungeons)))
	for _, dungeon := range status.SelectedDungeons {
		resp.Uint32(dungeon)
	}

	members := lfgMembersLeaderFirst(status.QueuedMembers)
	resp.Uint8(uint8(len(members)))
	for _, member := range members {
		resp.Uint64(playerObjectGUIDForRealm(lfgMemberRealmID(member.RealmID), uint64(member.PlayerGUID)))
		resp.Uint8(boolToUint8(member.Roles > 0))
		resp.Uint32(uint32(member.Roles))
		resp.Uint8(s.lfgMemberLevel(member.RealmID, member.PlayerGUID))
	}
	s.gameSocket.Send(resp)
}

func lfgMembersLeaderFirst(members []events.MatchmakingLfgMember) []events.MatchmakingLfgMember {
	res := append([]events.MatchmakingLfgMember(nil), members...)
	for i, member := range res {
		if member.Leader {
			res[0], res[i] = res[i], res[0]
			break
		}
	}
	return res
}

func lfgMemberRealmID(realmID uint32) uint32 {
	if realmID == 0 {
		return root.RealmID
	}
	return realmID
}

func (s *GameSession) lfgMemberLevel(realmID uint32, playerGUID guid.LowType) uint8 {
	if lfgMemberRealmID(realmID) == root.RealmID && uint64(playerGUID) == s.character.GUID {
		return s.character.Level
	}
	return 0
}

func (s *GameSession) sendLfgJoinResult(result uint32, state uint32) {
	resp := packet.NewWriter(packet.SMsgLFGJoinResult)
	resp.Uint32(result)
	resp.Uint32(state)
	s.gameSocket.Send(resp)
}

func (s *GameSession) sendLfgQueueStatus(status events.MatchmakingLfgStatusPayload) {
	dungeon := uint32(0)
	if len(status.SelectedDungeons) > 0 {
		dungeon = lfgDungeonID(status.SelectedDungeons[0])
	}

	resp := packet.NewWriter(packet.SMsgLFGQueueStatus)
	resp.Uint32(dungeon)
	resp.Int32(-1)
	resp.Int32(-1)
	resp.Int32(-1)
	resp.Int32(-1)
	resp.Int32(-1)
	resp.Uint8(status.TanksNeeded)
	resp.Uint8(status.HealersNeeded)
	resp.Uint8(status.DamageNeeded)
	resp.Uint32(lfgQueueStatusQueuedTimeSeconds(status))
	s.gameSocket.Send(resp)
}

func lfgDungeonID(entry uint32) uint32 {
	return entry & lfgDungeonIDMask
}

func lfgQueueStatusQueuedTimeSeconds(status events.MatchmakingLfgStatusPayload) uint32 {
	// The payload field is historic; the client packet expects elapsed seconds.
	if status.QueuedTimeMilliseconds == 0 {
		return 1
	}
	return status.QueuedTimeMilliseconds
}

func (s *GameSession) sendLfgProposalUpdate(status events.MatchmakingLfgStatusPayload) {
	resp := packet.NewWriter(packet.SMsgLFGProposalUpdate)
	resp.Uint32(lfgProposalDisplayDungeon(status))
	resp.Uint8(uint8(status.ProposalState))
	resp.Uint32(status.ProposalID)
	resp.Uint32(0)
	resp.Uint8(0)
	resp.Uint8(uint8(len(status.ProposalMembers)))

	for _, member := range status.ProposalMembers {
		resp.Uint32(uint32(member.AssignedRole))
		resp.Uint8(boolToUint8(lfgMemberRealmID(member.RealmID) == root.RealmID && uint64(member.PlayerGUID) == s.character.GUID))
		resp.Uint8(0)
		resp.Uint8(0)
		resp.Uint8(boolToUint8(member.Answered))
		resp.Uint8(boolToUint8(member.Accepted))
	}
	s.gameSocket.Send(resp)
}

func lfgProposalDisplayDungeon(status events.MatchmakingLfgStatusPayload) uint32 {
	if status.DungeonEntry == 0 {
		return 0
	}
	for _, entry := range status.SelectedDungeons {
		if entry&lfgDungeonIDMask == status.DungeonEntry&lfgDungeonIDMask {
			return entry
		}
	}
	return status.DungeonEntry
}

func boolToUint8(v bool) uint8 {
	if v {
		return 1
	}
	return 0
}

func lfgStateUsesNativeDungeon(state events.MatchmakingLfgState) bool {
	return state == events.MatchmakingLfgStateDungeon || state == events.MatchmakingLfgStateFinishedDungeon
}

func lfgProtoStateUsesNativeDungeon(state pbMM.LfgState) bool {
	return state == pbMM.LfgState_LFG_STATE_DUNGEON || state == pbMM.LfgState_LFG_STATE_FINISHED_DUNGEON
}

func lfgStatusPayloadFromProto(status *pbMM.LfgStatusData) events.MatchmakingLfgStatusPayload {
	if status == nil {
		return events.MatchmakingLfgStatusPayload{State: events.MatchmakingLfgStateNone}
	}

	queuedMembers := make([]events.MatchmakingLfgMember, 0, len(status.QueuedMembers))
	for _, member := range status.QueuedMembers {
		queuedMembers = append(queuedMembers, events.MatchmakingLfgMember{
			RealmID:            lfgMemberRealmID(member.RealmID),
			PlayerGUID:         guid.LowType(member.PlayerGUID),
			Roles:              uint8(member.Roles),
			Leader:             member.Leader,
			QueueLeaderRealmID: lfgMemberRealmID(member.QueueLeaderRealmID),
			QueueLeaderGUID:    guid.LowType(member.QueueLeaderGUID),
		})
	}

	proposalMembers := make([]events.MatchmakingLfgProposalMember, 0, len(status.ProposalMembers))
	for _, member := range status.ProposalMembers {
		proposalMembers = append(proposalMembers, events.MatchmakingLfgProposalMember{
			RealmID:       lfgMemberRealmID(member.RealmID),
			PlayerGUID:    guid.LowType(member.PlayerGUID),
			SelectedRoles: uint8(member.SelectedRoles),
			AssignedRole:  uint8(member.AssignedRole),
			Answered:      member.Answered,
			Accepted:      member.Accepted,
		})
	}

	return events.MatchmakingLfgStatusPayload{
		State:                  events.MatchmakingLfgState(status.State),
		ProposalID:             status.ProposalID,
		ProposalState:          events.MatchmakingLfgProposalState(status.ProposalState),
		DungeonEntry:           status.DungeonEntry,
		SelectedDungeons:       status.SelectedDungeons,
		QueuedMembers:          queuedMembers,
		ProposalMembers:        proposalMembers,
		QueuedTimeMilliseconds: status.QueuedTimeMilliseconds,
		TanksNeeded:            uint8(status.TanksNeeded),
		HealersNeeded:          uint8(status.HealersNeeded),
		DamageNeeded:           uint8(status.DamageNeeded),
	}
}
