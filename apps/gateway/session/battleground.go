package session

import (
	"context"
	"fmt"

	"github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/sockets"
	pbGroup "github.com/walkline/ToCloud9/gen/group/pb"
	"github.com/walkline/ToCloud9/gen/matchmaking/pb"
	pbGameServ "github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type BattlegroundStatus uint8

const (
	BattlegroundStatusNone BattlegroundStatus = iota
	BattlegroundStatusInQueue
	BattlegroundStatusInvited
	BattlegroundStatusPlaying
	BattlegroundStatusLeaving
)

const (
	nagrandArenaBattlegroundTypeID  uint32 = 4
	bladesEdgeBattlegroundTypeID    uint32 = 5
	allArenasBattlegroundTypeID     uint32 = 6
	ruinsOfLordaeronBattlegroundID  uint32 = 8
	dalaranSewersBattlegroundTypeID uint32 = 10
	ringOfValorBattlegroundTypeID   uint32 = 11
	defaultBattlegroundQueueSlot    uint8  = 0

	// AzerothCore SharedDefines.h: PLAYER_MAX_BATTLEGROUND_QUEUES.
	playerMaxBattlegroundQueues uint8 = 2
)

const (
	nagrandArenaMapID       uint32 = 559
	bladesEdgeArenaMapID    uint32 = 562
	ruinsOfLordaeronMapID   uint32 = 572
	dalaranSewersArenaMapID uint32 = 617
	ringOfValorArenaMapID   uint32 = 618
)

var arenaNativeWorldportMapIDs = []uint32{
	nagrandArenaMapID,
	bladesEdgeArenaMapID,
	ruinsOfLordaeronMapID,
	dalaranSewersArenaMapID,
	ringOfValorArenaMapID,
}

const battlegroundQueueUnavailableMessage = "Battleground queue was reset because the matchmaking service restarted. Please queue again."

func (s *GameSession) HandleEnqueueToBattleground(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	/*battlemasterGUID*/ _ = r.Uint64()
	bgTypeID := r.Uint32()
	/*instanceID*/ _ = r.Uint32()
	joinAsGroup := r.Uint8()

	return s.enqueueToPVPQueue(ctx, bgTypeID, joinAsGroup, 0, false)
}

func (s *GameSession) HandleEnqueueToArena(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	battlemasterGUID := r.Uint64()
	arenaSlot := r.Uint8()
	joinAsGroup := r.Uint8()
	isRated := r.Uint8() != 0

	if isRated && joinAsGroup == 0 {
		return nil
	}

	arenaType, ok := arenaTypeBySlot(arenaSlot)
	if !ok {
		return fmt.Errorf("unknown arena slot: %d", arenaSlot)
	}

	if s.logger != nil && s.character != nil {
		s.logger.Debug().
			Uint64("playerGUID", s.character.GUID).
			Uint64("battlemasterGUID", battlemasterGUID).
			Uint8("arenaSlot", arenaSlot).
			Uint32("arenaType", arenaType).
			Uint8("joinAsGroup", joinAsGroup).
			Bool("isRated", isRated).
			Msg("TC9 arena queue request")
	}

	if err := s.enqueueToPVPQueue(ctx, allArenasBattlegroundTypeID, joinAsGroup, arenaType, isRated); err != nil {
		sendGroupJoinedBattlegroundError(s.gameSocket)
		return nil
	}

	return nil
}

func sendGroupJoinedBattlegroundError(gameSocket sockets.Socket) {
	const errBattlegroundJoinFailed int32 = -12
	resp := packet.NewWriterWithSize(packet.SMsgGroupJoinedBattleground, 4)
	resp.Int32(errBattlegroundJoinFailed)
	gameSocket.Send(resp)
}

func isArenaBattlegroundTypeID(bgTypeID uint32) bool {
	switch bgTypeID {
	case nagrandArenaBattlegroundTypeID,
		bladesEdgeBattlegroundTypeID,
		allArenasBattlegroundTypeID,
		ruinsOfLordaeronBattlegroundID,
		dalaranSewersBattlegroundTypeID,
		ringOfValorBattlegroundTypeID:
		return true
	default:
		return false
	}
}

func arenaTypeBySlot(arenaSlot uint8) (uint32, bool) {
	switch arenaSlot {
	case 0:
		return 2, true
	case 1:
		return 3, true
	case 2:
		return 5, true
	default:
		return 0, false
	}
}

func (s *GameSession) enqueueToPVPQueue(ctx context.Context, bgTypeID uint32, joinAsGroup uint8, arenaType uint32, isRated bool) error {
	members := []uint64{}
	if joinAsGroup > 0 {
		groupResp, err := s.groupServiceClient.GetGroupByMember(ctx, &pbGroup.GetGroupByMemberRequest{
			Api:     gateway.SupportedGroupServiceVer,
			RealmID: gateway.RealmID,
			Player:  s.character.GUID,
		})
		if err != nil {
			return NewGroupServiceUnavailableErr(err)
		}
		if groupResp.GetGroup() == nil || groupResp.GetGroup().GetLeader() != s.character.GUID {
			sendGroupJoinedBattlegroundError(s.gameSocket)
			return nil
		}

		for _, member := range groupResp.Group.Members {
			// Don't add leader, since we have the leader field
			if member.Guid == s.character.GUID {
				continue
			}
			members = append(members, member.Guid)
		}
	}

	// TODO: figure out what to do with others member check...
	gameClient, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(s.worldSocket.Address())
	if err != nil {
		return fmt.Errorf("can't get gameServiceClient, err: %w", err)
	}

	res, err := gameClient.CanPlayerJoinBattlegroundQueue(ctx, &pbGameServ.CanPlayerJoinBattlegroundQueueRequest{
		Api:        gateway.SupportedGameServerVer,
		PlayerGUID: s.character.GUID,
	})
	if err != nil || res.Status != pbGameServ.CanPlayerJoinBattlegroundQueueResponse_Success {
		// TODO: handle the error more granular
		const ErrGroupJoinBattlegroundDeserters int32 = -2
		resp := packet.NewWriterWithSize(packet.SMsgGroupJoinedBattleground, 4)
		resp.Int32(ErrGroupJoinBattlegroundDeserters)
		s.gameSocket.Send(resp)
		return nil
	}

	teamID := pb.PVPTeamID_Alliance
	if wow.DefaultRaces[s.character.Race].Team == wow.TeamHorde {
		teamID = pb.PVPTeamID_Horde
	}

	if s.logger != nil {
		s.logger.Debug().
			Uint64("playerGUID", s.character.GUID).
			Uint32("realmID", gateway.RealmID).
			Uint32("bgTypeID", bgTypeID).
			Uint32("arenaType", arenaType).
			Uint8("joinAsGroup", joinAsGroup).
			Bool("isRated", isRated).
			Interface("partyMembers", members).
			Str("teamID", teamID.String()).
			Msg("TC9 PVP enqueue resolved")
	}

	_, err = s.matchmakingServiceClient.EnqueueToBattleground(ctx, &pb.EnqueueToBattlegroundRequest{
		Api:          gateway.SupportedMatchmakingServiceVer,
		RealmID:      gateway.RealmID,
		LeaderGUID:   s.character.GUID,
		PartyMembers: members,
		LeadersLvl:   uint32(s.character.Level),
		BgTypeID:     bgTypeID,
		TeamID:       teamID,
		ArenaType:    arenaType,
		IsRated:      isRated,
	})
	if err != nil {
		return err
	}

	s.character.bgInviteOrderingFix.waitingJoinToQueue = true

	return nil
}

func (s *GameSession) HandleBattlegroundPort(ctx context.Context, p *packet.Packet) error {
	type portActionType uint8

	const (
		LeaveQueue portActionType = iota
		EnterBattle
	)

	r := p.Reader()
	/*arenaType*/ _ = r.Uint8()
	/*unk1*/ _ = r.Uint8()
	bgTypeID := r.Uint32()
	/*unk2*/ _ = r.Uint16()
	action := portActionType(r.Uint8())

	switch action {
	case LeaveQueue:
		return s.leaveBattlegroundQueue(ctx, bgTypeID)
	case EnterBattle:
		return s.enterBattleground(ctx)
	default:
		return fmt.Errorf("unknown action: %d", action)
	}
}

func (s *GameSession) leaveBattlegroundQueue(ctx context.Context, bgTypeID uint32) error {
	_, err := s.matchmakingServiceClient.RemovePlayerFromQueue(ctx, &pb.RemovePlayerFromQueueRequest{
		Api:              gateway.SupportedMatchmakingServiceVer,
		RealmID:          gateway.RealmID,
		PlayerGUID:       s.character.GUID,
		BattlegroundType: bgTypeID,
	})
	if err != nil {
		return fmt.Errorf("error on removing player from queue: %w", err)
	}

	s.clearPVPQueueForType(bgTypeID)

	return nil
}

func (s *GameSession) enterBattleground(ctx context.Context) error {
	res, err := s.matchmakingServiceClient.BattlegroundQueueDataForPlayer(ctx, &pb.BattlegroundQueueDataForPlayerRequest{
		Api:        gateway.SupportedMatchmakingServiceVer,
		RealmID:    gateway.RealmID,
		PlayerGUID: s.character.GUID,
	})
	if err != nil {
		return err
	}

	if len(res.Slots) == 0 {
		return nil
	}

	selectedSlot := res.Slots[0]
	bgData := selectedSlot.AssignedBattlegroundData

	if bgData == nil {
		return fmt.Errorf("no battleground data found in HandleBattlegroundPort")
	}

	s.gameServerGRPCConnMgr.AddAddressMapping(bgData.GameserverAddress, bgData.GameserverGRPCAddress)

	desiredServerAddress := bgData.GameserverAddress

	crossrealmAdjustedPlayerGUID := s.character.GUID
	isCrossrealm := bgData.BattlegroupID != 0
	if isCrossrealm {
		crossrealmAdjustedPlayerGUID = guid.NewCrossrealmPlayerGUID(uint16(gateway.RealmID), guid.LowType(s.character.GUID)).GetRawValue()
	}

	grpcClient, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(bgData.GameserverAddress)
	if err != nil {
		return fmt.Errorf("gameServerGRPCConnMgr.GRPCConnByGameServerAddress failed: %w", err)
	}

	feature := clusterTransferFeatureBattleground
	operation := "battleground owner native worldport"
	if isArenaBattlegroundTypeID(selectedSlot.BgTypeID) {
		feature = clusterTransferFeatureArena
		operation = "arena owner native worldport"
	}
	transferRouting := pvpOwnerMapTransferRouting(feature, isCrossrealm)

	if s.logger != nil {
		s.logger.Debug().
			Uint64("playerGUID", s.character.GUID).
			Uint64("loginPlayerGUID", crossrealmAdjustedPlayerGUID).
			Uint32("realmID", gateway.RealmID).
			Uint32("bgTypeID", selectedSlot.BgTypeID).
			Uint32("instanceID", bgData.AssignedBattlegroundInstanceID).
			Uint32("mapID", bgData.MapID).
			Uint32("battlegroupID", bgData.BattlegroupID).
			Str("gameserverAddress", bgData.GameserverAddress).
			Str("gameserverGRPCAddress", bgData.GameserverGRPCAddress).
			Str("feature", feature.String()).
			Bool("isCrossrealm", isCrossrealm).
			Msg("TC9 PVP owner placement selected")
	}

	forwardOptions := nativeWorldportForwardOptions{
		synthesizeTransferPendingForNewMap: true,
		expectedMapID:                      bgData.MapID,
	}
	if feature == clusterTransferFeatureArena {
		forwardOptions.acceptedMapIDs = arenaNativeWorldportMapIDs
	}

	if err := s.startClusterOwnerNativeWorldportTransport(ctx, clusterOwnerNativeWorldportTransport{
		feature:                         feature,
		operation:                       operation,
		sessionPlayerGUID:               s.character.GUID,
		loginPlayerGUID:                 crossrealmAdjustedPlayerGUID,
		targetAddress:                   desiredServerAddress,
		routing:                         transferRouting,
		forwardAfterPlacement:           true,
		reloadManagedGroupAfterTransfer: true,
		forwardOptions:                  forwardOptions,
		onOwnerPlaced: func(ctx context.Context) error {
			var alliancePlayers []uint64
			var hordePlayers []uint64
			switch bgData.AssignedTeamID {
			case pb.PVPTeamID_Horde:
				hordePlayers = []uint64{crossrealmAdjustedPlayerGUID}
			default:
				alliancePlayers = []uint64{crossrealmAdjustedPlayerGUID}
			}

			_, err := grpcClient.AddPlayersToBattleground(ctx, &pbGameServ.AddPlayersToBattlegroundRequest{
				Api:                  "0.0.1",
				BattlegroundTypeID:   pbGameServ.BattlegroundType(selectedSlot.BgTypeID),
				InstanceID:           bgData.AssignedBattlegroundInstanceID,
				PlayersToAddAlliance: alliancePlayers,
				PlayersToAddHorde:    hordePlayers,
			})
			if err != nil {
				return fmt.Errorf("AddPlayersToBattleground failed: %w", err)
			}

			_, err = s.matchmakingServiceClient.PlayerJoinedBattleground(ctx, &pb.PlayerJoinedBattlegroundRequest{
				Api:          gateway.SupportedMatchmakingServiceVer,
				RealmID:      gateway.RealmID,
				PlayerGUID:   s.character.GUID,
				InstanceID:   bgData.AssignedBattlegroundInstanceID,
				IsCrossRealm: isCrossrealm,
			})
			if err != nil {
				return fmt.Errorf("PlayerJoinedBattleground failed: %w", err)
			}

			return nil
		},
	}); err != nil {
		return fmt.Errorf("%s failed: %w", operation, err)
	}

	return nil
}

func pvpOwnerMapTransferRouting(feature clusterTransferFeature, isCrossrealm bool) *mapTransferRouting {
	routing := &mapTransferRouting{
		realmID: gateway.RealmID,
		feature: feature,
	}
	if isCrossrealm {
		routing.realmID = 0
		routing.isCrossRealm = true
	}
	return routing
}

func (s *GameSession) HandleEventMMJoinedPVPQueue(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.MatchmakingEventPlayersQueuedPayload)
	queueSlot := eventData.QueueSlotByPlayer[guid.LowType(s.character.GUID)]
	resp := packet.NewWriterWithSize(packet.SMsgBattlefieldStatus, 0)
	resp.Uint32(uint32(queueSlot))
	resp.Uint8(eventData.ArenaType)
	if eventData.ArenaType == 0 {
		resp.Uint8(0) // unk flag
	} else {
		resp.Uint8(0xE) // unk flag
	}
	resp.Uint32(uint32(eventData.TypeID))
	resp.Uint16(0x1F90) // magic flag
	resp.Uint8(eventData.PVPQueueMinLVL)
	resp.Uint8(eventData.PVPQueueMaxLVL)
	resp.Uint32(0)

	if eventData.IsRated {
		resp.Uint8(1)
	} else {
		resp.Uint8(0)
	}

	resp.Uint32(uint32(BattlegroundStatusInQueue))
	resp.Uint32(eventData.AverageWaitingTimeMilliseconds)
	resp.Uint32(0)

	s.gameSocket.Send(resp)
	s.rememberPVPQueueSlot(uint32(eventData.TypeID), queueSlot)

	s.character.bgInviteOrderingFix.waitingJoinToQueue = false
	if s.character.bgInviteOrderingFix.pendingInvitePacket != nil {
		s.gameSocket.SendPacket(s.character.bgInviteOrderingFix.pendingInvitePacket)
		s.character.bgInviteOrderingFix.pendingInvitePacket = nil
	}

	return nil
}

func (s *GameSession) HandleEventMMInvitedToBGOrArena(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.MatchmakingEventPlayersInvitedPayload)
	queueSlot := eventData.QueueSlotByPlayer[guid.LowType(s.character.GUID)]
	resp := packet.NewWriterWithSize(packet.SMsgBattlefieldStatus, 0)
	resp.Uint32(uint32(queueSlot))
	resp.Uint8(eventData.ArenaType)
	if eventData.ArenaType == 0 {
		resp.Uint8(0) // unk flag
	} else {
		resp.Uint8(0xE) // unk flag
	}
	resp.Uint32(uint32(eventData.TypeID))
	resp.Uint16(0x1F90) // magic flag
	resp.Uint8(eventData.PVPQueueMinLVL)
	resp.Uint8(eventData.PVPQueueMaxLVL)
	resp.Uint32(0)

	if eventData.IsRated {
		resp.Uint8(1)
	} else {
		resp.Uint8(0)
	}

	resp.Uint32(uint32(BattlegroundStatusInvited))
	resp.Uint32(eventData.MapID)
	resp.Uint64(0)
	resp.Uint32(eventData.TimeToAcceptMilliseconds)

	s.rememberPVPQueueSlot(uint32(eventData.TypeID), queueSlot)

	// In some cases Invite event can arrive faster than JoinToQueue.
	// In this case wait for JoinToQueue event and then send Invite.
	if s.character.bgInviteOrderingFix.waitingJoinToQueue {
		s.character.bgInviteOrderingFix.pendingInvitePacket = resp.ToPacket()
	} else {
		s.gameSocket.Send(resp)
	}

	return nil
}

func (s *GameSession) HandleEventMMInviteToBGOrArenaExpired(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.MatchmakingEventPlayersInviteExpiredPayload)
	if queueSlot, ok := eventData.QueueSlotByPlayer[guid.LowType(s.character.GUID)]; ok {
		s.sendBattlegroundStatusNone(queueSlot)
		s.forgetPVPQueueSlotBySlot(queueSlot)
	} else {
		s.clearTrackedPVPQueueSlots()
	}

	return nil
}

func (s *GameSession) HandleEventMMServiceUnavailable(ctx context.Context, e *eBroadcaster.Event) error {
	if s.character == nil || s.character.GroupMangedByGameServer {
		return nil
	}

	if s.clearPVPQueuesAfterMatchmakingUnavailable() {
		s.SendSysMessage(battlegroundQueueUnavailableMessage)
	}
	if s.clearLfgAfterMatchmakingUnavailable() {
		s.SendSysMessage(lfgQueueUnavailableMessage)
	}

	return nil
}

func (s *GameSession) rememberPVPQueueSlot(bgTypeID uint32, queueSlot uint8) {
	if queueSlot >= playerMaxBattlegroundQueues {
		return
	}

	if s.character.pvpQueueSlotsByType == nil {
		s.character.pvpQueueSlotsByType = map[uint32]uint8{}
	}

	s.character.pvpQueueSlotsByType[bgTypeID] = queueSlot
}

func (s *GameSession) clearPVPQueueForType(bgTypeID uint32) {
	if s.character.pvpQueueSlotsByType != nil {
		if queueSlot, ok := s.character.pvpQueueSlotsByType[bgTypeID]; ok {
			s.sendBattlegroundStatusNone(queueSlot)
			delete(s.character.pvpQueueSlotsByType, bgTypeID)
			return
		}
	}

	s.sendBattlegroundStatusNone(defaultBattlegroundQueueSlot)
}

func (s *GameSession) forgetPVPQueueSlotBySlot(queueSlot uint8) {
	if s.character.pvpQueueSlotsByType == nil {
		return
	}

	for bgTypeID, storedSlot := range s.character.pvpQueueSlotsByType {
		if storedSlot == queueSlot {
			delete(s.character.pvpQueueSlotsByType, bgTypeID)
		}
	}
}

func (s *GameSession) clearPVPQueuesAfterMatchmakingUnavailable() bool {
	hasPendingQueueJoin := s.character.bgInviteOrderingFix.waitingJoinToQueue ||
		s.character.bgInviteOrderingFix.pendingInvitePacket != nil

	clearedSlots := s.clearTrackedPVPQueueSlots()
	if clearedSlots == 0 && hasPendingQueueJoin {
		s.clearAllPVPQueueSlots()
		clearedSlots = int(playerMaxBattlegroundQueues)
	}

	s.character.bgInviteOrderingFix.waitingJoinToQueue = false
	s.character.bgInviteOrderingFix.pendingInvitePacket = nil

	return clearedSlots > 0 || hasPendingQueueJoin
}

func (s *GameSession) clearTrackedPVPQueueSlots() int {
	if s.character.pvpQueueSlotsByType == nil {
		return 0
	}

	slots := map[uint8]struct{}{}
	for _, queueSlot := range s.character.pvpQueueSlotsByType {
		if queueSlot >= playerMaxBattlegroundQueues {
			continue
		}
		slots[queueSlot] = struct{}{}
	}

	s.character.pvpQueueSlotsByType = map[uint32]uint8{}

	for queueSlot := range slots {
		s.sendBattlegroundStatusNone(queueSlot)
	}

	return len(slots)
}

func (s *GameSession) clearAllPVPQueueSlots() {
	for queueSlot := uint8(0); queueSlot < playerMaxBattlegroundQueues; queueSlot++ {
		s.sendBattlegroundStatusNone(queueSlot)
	}
}

func (s *GameSession) sendBattlegroundStatusNone(queueSlot uint8) {
	resp := packet.NewWriterWithSize(packet.SMsgBattlefieldStatus, 4+8)
	resp.Uint32(uint32(queueSlot))
	resp.Uint64(0)
	s.gameSocket.Send(resp)
}
