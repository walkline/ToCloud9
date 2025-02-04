package session

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	gameloadbalancer "github.com/walkline/ToCloud9/apps/game-load-balancer"
	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
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

func (s *GameSession) HandleEnqueueToBattleground(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	/*battlemasterGUID*/ _ = r.Uint64()
	bgTypeID := r.Uint32()
	/*instanceID*/ _ = r.Uint32()
	joinAsGroup := r.Uint8()

	members := []uint64{}
	if joinAsGroup > 0 {
		groupResp, err := s.groupServiceClient.GetGroupByMember(ctx, &pbGroup.GetGroupByMemberRequest{
			Api:     gameloadbalancer.SupportedGroupServiceVer,
			RealmID: gameloadbalancer.RealmID,
			Player:  s.character.GUID,
		})
		if err != nil {
			return NewGroupServiceUnavailableErr(err)
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
		Api:        gameloadbalancer.SupportedGameServerVer,
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

	_, err = s.matchmakingServiceClient.EnqueueToBattleground(ctx, &pb.EnqueueToBattlegroundRequest{
		Api:          gameloadbalancer.SupportedMatchmakingServiceVer,
		RealmID:      gameloadbalancer.RealmID,
		LeaderGUID:   s.character.GUID,
		PartyMembers: members,
		LeadersLvl:   uint32(s.character.Level),
		BgTypeID:     bgTypeID,
		TeamID:       teamID,
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
		Api:              gameloadbalancer.SupportedMatchmakingServiceVer,
		RealmID:          gameloadbalancer.RealmID,
		PlayerGUID:       s.character.GUID,
		BattlegroundType: bgTypeID,
	})
	if err != nil {
		return fmt.Errorf("error on removing player from queue: %w", err)
	}

	resp := packet.NewWriterWithSize(packet.SMsgBattlefieldStatus, 6+4)
	resp.Uint32(0)
	resp.Uint64(0)
	s.gameSocket.Send(resp)

	return nil
}

func (s *GameSession) enterBattleground(ctx context.Context) error {
	res, err := s.matchmakingServiceClient.BattlegroundQueueDataForPlayer(ctx, &pb.BattlegroundQueueDataForPlayerRequest{
		Api:        gameloadbalancer.SupportedMatchmakingServiceVer,
		RealmID:    gameloadbalancer.RealmID,
		PlayerGUID: s.character.GUID,
	})
	if err != nil {
		return err
	}

	if len(res.Slots) == 0 {
		return nil
	}

	bgData := res.Slots[0].AssignedBattlegroundData

	if bgData == nil {
		return fmt.Errorf("no battleground data found in HandleBattlegroundPort")
	}

	s.gameServerGRPCConnMgr.AddAddressMapping(bgData.GameserverAddress, bgData.GameserverGRPCAddress)

	oldServerAddress := s.worldSocket.Address()
	desiredServerAddress := bgData.GameserverAddress

	s.character.ignoreNextInterceptToNewMap = &bgData.MapID

	crossrealmAdjustedPlayerGUID := s.character.GUID
	isCrossrealm := bgData.BattlegroupID != 0
	if isCrossrealm {
		crossrealmAdjustedPlayerGUID = guid.NewCrossrealmPlayerGUID(uint16(gameloadbalancer.RealmID), guid.LowType(s.character.GUID)).GetRawValue()
	}

	if desiredServerAddress != oldServerAddress {
		err = s.battlegroundPlayerRedirect(ctx, crossrealmAdjustedPlayerGUID, desiredServerAddress)
		if err != nil {
			return fmt.Errorf("battleground player redirect failed: %w", err)
		}
	}

	grpcClient, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(bgData.GameserverAddress)
	if err != nil {
		return fmt.Errorf("gameServerGRPCConnMgr.GRPCConnByGameServerAddress failed: %w", err)
	}

	_, err = grpcClient.AddPlayersToBattleground(ctx, &pbGameServ.AddPlayersToBattlegroundRequest{
		Api:                "0.0.1",
		BattlegroundTypeID: pbGameServ.BattlegroundType(res.Slots[0].BgTypeID),
		InstanceID:         bgData.AssignedBattlegroundInstanceID,
		// TODO: clarify alliance & horde situation
		PlayersToAddAlliance: []uint64{crossrealmAdjustedPlayerGUID},
		PlayersToAddHorde:    nil,
	})
	if err != nil {
		return fmt.Errorf("AddPlayersToBattleground failed: %w", err)
	}

	_, err = s.matchmakingServiceClient.PlayerJoinedBattleground(ctx, &pb.PlayerJoinedBattlegroundRequest{
		Api:          gameloadbalancer.SupportedMatchmakingServiceVer,
		RealmID:      gameloadbalancer.RealmID,
		PlayerGUID:   s.character.GUID,
		InstanceID:   bgData.AssignedBattlegroundInstanceID,
		IsCrossRealm: isCrossrealm,
	})
	if err != nil {
		return fmt.Errorf("PlayerJoinedBattleground failed: %w", err)
	}

	s.character.GroupMangedByGameServer = true

	// ignore all packets except new world
	if err := s.processWorldPacketsInPlace(ctx, func(p *packet.Packet) (stopProcessing bool, err error) {
		if p.Opcode != packet.SMsgNewWorld {
			return false, nil
		}

		mapID := p.Reader().Uint32()
		if mapID != s.character.Map {
			resp := packet.NewWriterWithSize(packet.SMsgTransferPending, 0)
			resp.Uint32(mapID)
			s.gameSocket.Send(resp)
		}
		s.gameSocket.WriteChannel() <- p
		return true, nil
	}); err != nil {
		log.Err(err).Msg("failed to filter character login packets")
	}

	return nil
}

func (s *GameSession) battlegroundPlayerRedirect(ctx context.Context, playerGuid uint64, desiredGameServerAddress string) error {
	oldServerAddress := s.worldSocket.Address()

	saveAndClosePacket := packet.NewWriterWithSize(packet.TC9CMsgPrepareForRedirect, 0)
	s.worldSocket.Send(saveAndClosePacket)

	confirmationIsSuccessfulChan := make(chan bool)
	confirmationContext, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	go func() {
		defer close(confirmationIsSuccessfulChan)
		for {
			select {
			case <-confirmationContext.Done():
				confirmationIsSuccessfulChan <- false
				return
			case p, open := <-s.worldSocket.ReadChannel():
				if !open {
					// If socket closed, then it also not bad, let's assume that as a good sign as well.
					confirmationIsSuccessfulChan <- true
					return
				}
				if p.Opcode == packet.TC9SMsgReadyForRedirect {
					confirmationIsSuccessfulChan <- p.Reader().Uint8() == 0
					return
				}
			}
		}
	}()

	// Waits till new value in chan.
	isReadyForRedirect := <-confirmationIsSuccessfulChan
	if !isReadyForRedirect {
		return fmt.Errorf("failed to redirect player with account %d, world server failed to prepare", s.accountID)
	}

	s.worldSocket.Close()
	s.worldSocket = nil

	newSocket, err := s.connectToGameServerWithAddress(ctx, playerGuid, desiredGameServerAddress)
	if err != nil {
		return fmt.Errorf("connectToGameServerWithAddress failed: %w, address: %s", err, desiredGameServerAddress)
	}

	select {
	case _, open := <-newSocket.ReadChannel():
		if !open {
			return fmt.Errorf("world socket closed")
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	s.worldSocket = newSocket

	if s.showGameserverConnChangeToClient {
		s.SendSysMessage(fmt.Sprintf("You have been redirected from %s to %s gameserver.", oldServerAddress, desiredGameServerAddress))
	}

	return nil
}

func (s *GameSession) HandleEventMMJoinedPVPQueue(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.MatchmakingEventPlayersQueuedPayload)
	resp := packet.NewWriterWithSize(packet.SMsgBattlefieldStatus, 0)
	resp.Uint32(uint32(eventData.QueueSlotByPlayer[guid.LowType(s.character.GUID)]))
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

	s.character.bgInviteOrderingFix.waitingJoinToQueue = false
	if s.character.bgInviteOrderingFix.pendingInvitePacket != nil {
		s.gameSocket.SendPacket(s.character.bgInviteOrderingFix.pendingInvitePacket)
		s.character.bgInviteOrderingFix.pendingInvitePacket = nil
	}

	return nil
}

func (s *GameSession) HandleEventMMInvitedToBGOrArena(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.MatchmakingEventPlayersInvitedPayload)
	resp := packet.NewWriterWithSize(packet.SMsgBattlefieldStatus, 0)
	resp.Uint32(uint32(eventData.QueueSlotByPlayer[guid.LowType(s.character.GUID)]))
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
	resp := packet.NewWriterWithSize(packet.SMsgBattlefieldStatus, 6+4)
	resp.Uint32(0)
	resp.Uint64(0)
	s.gameSocket.Send(resp)

	return nil
}
