package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/sockets"
	"github.com/walkline/ToCloud9/gen/characters/pb"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	pbGameServ "github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

func (s *GameSession) InterceptInitWorldStates(ctx context.Context, p *packet.Packet) error {
	s.gameSocket.SendPacket(p)

	reader := p.Reader()
	mapID := reader.Int32()
	zoneID := reader.Int32()
	areaID := reader.Int32()

	if s.character.Map != uint32(mapID) {
		s.charsUpdsBarrier.UpdateMap(s.character.GUID, uint32(mapID))
		s.character.Map = uint32(mapID)
	}

	if s.character.Zone != uint32(zoneID) {
		s.charsUpdsBarrier.UpdateZone(s.character.GUID, uint32(areaID), uint32(zoneID))
		s.character.Zone = uint32(zoneID)
	}

	if s.pendingRedirectID != "" {
		now := time.Now()
		s.logger.Info().
			Str("redirect", s.pendingRedirectID).
			Uint32("account", s.accountID).
			Uint64("character", s.character.GUID).
			Uint32("map", uint32(mapID)).
			Uint32("zone", uint32(zoneID)).
			Uint32("area", uint32(areaID)).
			Dur("totalDuration", now.Sub(s.pendingRedirectAt)).
			Msg("Cross-worldserver redirect reached target world")
		s.pendingRedirectID = ""
		s.pendingRedirectAt = time.Time{}
	}

	if err := s.confirmLfgDungeonRouteEntered(ctx, uint32(mapID)); err != nil {
		return err
	}

	return s.reloadManagedGroupAfterMapTransfer(ctx)
}

func (s *GameSession) InterceptLevelUpInfo(ctx context.Context, p *packet.Packet) error {
	s.gameSocket.SendPacket(p)

	reader := p.Reader()
	lvl := reader.Int32()

	if s.character.Level != uint8(lvl) {
		s.charsUpdsBarrier.UpdateLevel(s.character.GUID, uint8(lvl))
		s.character.Level = uint8(lvl)
	}

	return nil
}

func (s *GameSession) InterceptTransferPending(_ context.Context, p *packet.Packet) error {
	if s.shouldSuppressRedirectWorldPortPacket(p) {
		return nil
	}

	s.gameSocket.SendPacket(p)
	return nil
}

func (s *GameSession) InterceptNewWorld(ctx context.Context, p *packet.Packet) error {
	if s.shouldSuppressRedirectWorldPortPacket(p) {
		return nil
	}

	mapID := p.Reader().Uint32()
	return s.forwardNewWorldPacket(ctx, p, mapID)
}

func (s *GameSession) forwardNewWorldPacket(ctx context.Context, p *packet.Packet, mapID uint32) error {
	if s.character.ignoreNextInterceptToNewMap == nil || mapID != *s.character.ignoreNextInterceptToNewMap {
		s.teleportingToNewMap = &mapID
		if s.pendingMapTransferRouting == nil {
			routing, blocked, err := s.lfgRoutingForNativeWorldport(ctx, mapID)
			if err != nil {
				return err
			}
			if blocked {
				s.teleportingToNewMap = nil
				s.clearPendingMapTransferRouting()
				s.sendTransferAborted(mapID, transferAbortNotFound)
				return nil
			}
			if routing != nil {
				s.setPendingMapTransferRouting(routing)
				s.character.GroupMangedByGameServer = true
			}
		}
		s.activatePendingMapTransferRouting()
	}
	s.gameSocket.SendPacket(p)
	return nil
}

func (s *GameSession) shouldSuppressRedirectWorldPortPacket(p *packet.Packet) bool {
	if s == nil || p == nil || p.Source != packet.SourceWorldServer {
		return false
	}

	characterGUID := uint64(0)
	if s.character != nil {
		characterGUID = s.character.GUID
	}

	if s.pendingRedirectID != "" {
		if s.logger != nil {
			s.logger.Debug().
				Str("redirect", s.pendingRedirectID).
				Uint32("account", s.accountID).
				Uint64("character", characterGUID).
				Uint16("opcode", uint16(p.Opcode)).
				Msg("TC9 suppressing redirect target native worldport packet")
		}
		return true
	}
	return false
}

func (s *GameSession) InterceptMoveWorldPortAck(ctx context.Context, p *packet.Packet) error {
	if s.worldSocket == nil {
		return errors.New("can't handle InterceptMoveWorldPortAck, worldSocket is nil")
	}

	if s.teleportingToNewMap == nil {
		s.worldSocket.SendPacket(p)
		return nil
	}
	mapID := *s.teleportingToNewMap
	s.teleportingToNewMap = nil
	s.character.ignoreNextInterceptToNewMap = nil
	transferRouting := s.activeMapTransferRouting
	s.clearActiveMapTransferRouting()
	nextMapTransferRouting := cloneMapTransferRouting(transferRouting)

	realmID := root.RealmID
	isCrossRealm := false
	if transferRouting != nil {
		realmID = transferRouting.realmID
		isCrossRealm = transferRouting.isCrossRealm
	}
	if transferRouting != nil && transferRouting.ownerAddress != "" && shouldKeepMapTransferOnCurrentOwner(transferRouting, s.worldSocket.Address()) {
		s.setCurrentMapTransferRouting(nextMapTransferRouting)
		s.worldSocket.SendPacket(p)
		s.logger.Debug().
			Uint64("character", s.character.GUID).
			Uint32("map", mapID).
			Str("worldserver", s.worldSocket.Address()).
			Str("feature", transferRouting.feature.String()).
			Msg("World port ack remains on owner worldserver")
		return s.reloadManagedGroupAfterMapTransfer(ctx)
	}

	registryStarted := time.Now()
	serversResult, err := s.serversRegistryClient.AvailableGameServersForMapAndRealm(s.ctx, &pbServ.AvailableGameServersForMapAndRealmRequest{
		Api:          root.SupportedServerRegistryVer,
		RealmID:      realmID,
		MapID:        mapID,
		IsCrossRealm: isCrossRealm,
	})

	if err != nil {
		if shouldBlockLocalFallbackForMapTransfer(transferRouting) {
			s.sendTransferAborted(mapID, transferAbortNotFound)
			return err
		}
		s.worldSocket.SendPacket(p)
		return err
	}

	if len(serversResult.GameServers) == 0 {
		if shouldBlockLocalFallbackForMapTransfer(transferRouting) {
			s.sendTransferAborted(mapID, transferAbortNotFound)
			return fmt.Errorf("%w, mapID %v, realmID %d", worldConnectErrInstanceNotFound, mapID, realmID)
		}
		s.worldSocket.SendPacket(p)
		return fmt.Errorf("%w, mapID %v, realmID %d", worldConnectErrInstanceNotFound, mapID, realmID)
	}

	oldServerAddress := s.worldSocket.Address()
	desiredServer := serversResult.GameServers[0]
	desiredServerAddress := desiredServer.Address
	desiredWorldserverID := gameServerSourceID(desiredServer)

	if desiredServerAddress == oldServerAddress {
		s.setCurrentMapTransferRouting(nextMapTransferRouting)
		s.worldSocket.SendPacket(p)
		s.logger.Debug().
			Uint64("character", s.character.GUID).
			Uint32("map", mapID).
			Str("worldserver", oldServerAddress).
			Dur("registryDuration", time.Since(registryStarted)).
			Msg("World port ack remains on current worldserver")
		return s.reloadManagedGroupAfterMapTransfer(ctx)
	}

	redirectID := fmt.Sprintf("%d-%d-%d", s.accountID, s.character.GUID, time.Now().UnixNano())
	redirectStarted := time.Now()
	if !s.canReconnectCharacter(s.character.GUID) {
		return fmt.Errorf("stale redirect for account %d, character %d, map %d: session no longer owns character", s.accountID, s.character.GUID, mapID)
	}
	sessionCharacterGUID := s.character.GUID
	loginCharacterGUID := mapTransferLoginPlayerGUID(sessionCharacterGUID, transferRouting)
	var gameServerGRPCClient pbGameServ.WorldServerServiceClient
	if s.gameServerGRPCConnMgr != nil {
		s.gameServerGRPCConnMgr.AddAddressMapping(desiredServer.Address, desiredServer.GrpcAddress)
		gameServerGRPCClient, err = s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(desiredServer.Address)
		if err != nil {
			return fmt.Errorf("can't get game server grpc client for redirect target %s, err: %w", desiredServerAddress, err)
		}
	}
	s.pendingRedirectID = redirectID
	s.pendingRedirectAt = redirectStarted
	s.logger.Info().
		Str("redirect", redirectID).
		Uint32("account", s.accountID).
		Uint64("character", sessionCharacterGUID).
		Uint64("loginCharacter", loginCharacterGUID).
		Uint32("map", mapID).
		Str("source", oldServerAddress).
		Str("target", desiredServerAddress).
		Str("targetWorldserver", desiredWorldserverID).
		Int("candidates", len(serversResult.GameServers)).
		Dur("registryDuration", time.Since(registryStarted)).
		Msg("Starting cross-worldserver redirect")

	redirectFeature := clusterTransferFeatureGeneric
	if transferRouting != nil {
		redirectFeature = transferRouting.feature
	}
	saveAndClosePacket := clusterTransferPrepareRedirectPacket(redirectFeature)
	prepareStarted := time.Now()
	s.logger.Info().
		Str("redirect", redirectID).
		Uint32("account", s.accountID).
		Uint64("character", sessionCharacterGUID).
		Uint32("map", mapID).
		Str("source", oldServerAddress).
		Msg("Sending TC9CMsgPrepareForRedirect")
	s.worldSocket.Send(saveAndClosePacket)

	confirmationContext, cancel := context.WithTimeout(ctx, s.packetProcessTimeout)
	defer cancel()
	if err := s.waitForSourceRedirectReady(confirmationContext, s.worldSocket, redirectID, sessionCharacterGUID, mapID, oldServerAddress, desiredServerAddress); err != nil {
		if s.pendingRedirectID == redirectID {
			s.pendingRedirectID = ""
			s.pendingRedirectAt = time.Time{}
		}
		return fmt.Errorf("failed to redirect player with account %d, redirect %s, map %d, source %s, target %s after %s: %w", s.accountID, redirectID, mapID, oldServerAddress, desiredServerAddress, time.Since(prepareStarted), err)
	}
	s.logger.Info().
		Str("redirect", redirectID).
		Uint32("account", s.accountID).
		Uint64("character", sessionCharacterGUID).
		Uint32("map", mapID).
		Str("source", oldServerAddress).
		Dur("prepareDuration", time.Since(prepareStarted)).
		Msg("Received TC9SMsgReadyForRedirect")

	s.worldSocket.Close()
	s.worldSocket = nil

	go func(sessionGUID uint64, targetLoginGUID uint64) {
		var err error
		var socket sockets.Socket
		connectStarted := time.Now()
		if !s.canReconnectCharacter(sessionGUID) {
			return
		}
		connectCtx, connectCancel := context.WithTimeout(s.ctx, s.packetProcessTimeout)
		defer connectCancel()
		socket, err = s.connectToGameServerWithAddressRetry(connectCtx, targetLoginGUID, desiredServerAddress, nil)
		if err != nil {
			if !s.canReconnectCharacter(sessionGUID) {
				return
			}
			s.logger.Error().
				Err(err).
				Str("redirect", redirectID).
				Uint32("account", s.accountID).
				Uint64("character", sessionGUID).
				Uint64("loginCharacter", targetLoginGUID).
				Uint32("map", mapID).
				Str("target", desiredServerAddress).
				Dur("connectDuration", time.Since(connectStarted)).
				Msg("failed to reconnect player to the world")
			resp := packet.NewWriterWithSize(packet.SMsgCharacterLoginFailed, 1)
			resp.Uint8(uint8(packet.LoginErrorCodeWorldServerIsDown))
			s.gameSocket.Send(resp)
			clearRedirect := func(session *GameSession) {
				if session.pendingRedirectID == redirectID {
					session.pendingRedirectID = ""
					session.pendingRedirectAt = time.Time{}
				}
			}
			select {
			case s.sessionSafeFuChan <- clearRedirect:
			case <-s.ctx.Done():
			}
			return
		}

		// we need to modify session in a safe thread (goroutine)
		updateSession := func(session *GameSession) {
			if session.character == nil || session.character.GUID != sessionGUID || !session.canReconnectCharacter(sessionGUID) {
				socket.Close()
				return
			}

			session.worldSocket = socket
			session.worldserverID = desiredWorldserverID
			if gameServerGRPCClient != nil {
				session.gameServerGRPCClient = gameServerGRPCClient
			}
			session.setCurrentMapTransferRouting(transferRouting)
			session.pendingRedirectID = redirectID
			session.pendingRedirectAt = redirectStarted
			session.resetPlayerAuraState()

			if session.showGameserverConnChangeToClient {
				session.SendSysMessage(fmt.Sprintf("You have been redirected from %s to %s gameserver.", oldServerAddress, desiredServerAddress))
			}
			session.logger.Info().
				Str("redirect", redirectID).
				Uint32("account", session.accountID).
				Uint64("character", sessionGUID).
				Uint64("loginCharacter", targetLoginGUID).
				Uint32("map", mapID).
				Str("source", oldServerAddress).
				Str("target", desiredServerAddress).
				Str("targetWorldserver", desiredWorldserverID).
				Dur("connectDuration", time.Since(connectStarted)).
				Dur("totalDuration", time.Since(redirectStarted)).
				Msg("Cross-worldserver redirect connected to target")

			go func() {
				timer := time.NewTimer(time.Millisecond * 500)
				defer timer.Stop()

				select {
				case <-timer.C:
				case <-session.ctx.Done():
					return
				}

				rejoinSession := func(session *GameSession) {
					if !session.canReconnectCharacter(sessionGUID) {
						return
					}
					rejoinCtx, rejoinCancel := context.WithTimeout(session.ctx, session.packetProcessTimeout)
					defer rejoinCancel()
					if err := session.RejoinWorldserverToSystemChannels(rejoinCtx); err != nil {
						session.logger.Error().
							Err(err).
							Str("redirect", redirectID).
							Uint64("character", sessionGUID).
							Msg("failed to rejoin worldserver system channels after redirect")
					}
				}
				select {
				case session.sessionSafeFuChan <- rejoinSession:
				case <-session.ctx.Done():
				}
			}()
		}
		select {
		case s.sessionSafeFuChan <- updateSession:
		case <-s.ctx.Done():
			socket.Close()
		}
	}(sessionCharacterGUID, loginCharacterGUID)

	return nil
}

func (s *GameSession) reloadManagedGroupAfterMapTransfer(ctx context.Context) error {
	if s == nil || s.character == nil || !s.character.GroupMangedByGameServer || s.clusterGroupPresentationBlocked() {
		return nil
	}

	if err := s.LoadGroupForPlayer(ctx); err != nil {
		return err
	}

	s.character.GroupMangedByGameServer = false
	return nil
}

func (s *GameSession) InterceptMessageOfTheDay(ctx context.Context, p *packet.Packet) error {
	if s.packetSendingControl.motdSent {
		return nil
	}

	s.packetSendingControl.motdSent = true

	s.gameSocket.SendPacket(p)
	return nil
}

func (s *GameSession) InterceptAccountDataTimes(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	/*unixTime*/ _ = r.Uint32()
	/*someFlag*/ _ = r.Uint8()
	mask := r.Uint32()

	const (
		globalMask  = 0x15
		perCharMask = 0xEA
	)

	switch mask {
	case globalMask:
		if s.packetSendingControl.accountDataTimesGlobalSent {
			return nil
		}
		s.packetSendingControl.accountDataTimesGlobalSent = true
	case perCharMask:
		if s.packetSendingControl.accountDataTimesPerCharSent {
			return nil
		}
		s.packetSendingControl.accountDataTimesPerCharSent = true
	default:
	}
	s.gameSocket.SendPacket(p)
	return nil
}

func (s *GameSession) InterceptSMsgNameQueryResponse(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	charGUID := reader.ReadGUID()
	isNotFound := reader.Uint8()

	// Player found, nothing to do here.
	if isNotFound == 0 {
		s.gameSocket.SendPacket(p)
		return nil
	}

	g := guid.New(charGUID)

	res, err := s.charServiceClient.ShortOnlineCharactersDataByGUIDs(ctx, &pb.ShortCharactersDataByGUIDsRequest{
		Api:     "",
		RealmID: uint32(g.GetRealmID()),
		GUIDs:   []uint64{uint64(g.GetCounter())},
	})
	if err != nil {
		return err
	}

	if len(res.Characters) == 0 {
		s.gameSocket.SendPacket(p)
		return nil
	}

	playerData := res.Characters[0]
	s.sendNameQueryResponse(ctx, charGUID, playerData.CharName, playerData.CharRace, playerData.CharClass, playerData.CharGender)

	return nil
}

func (s *GameSession) HandleReadyForRedirectRequest(ctx context.Context, p *packet.Packet) error {
	if s == nil || s.character == nil {
		return errors.New("can't handle worldserver-initiated redirect, character is nil")
	}
	if s.worldSocket == nil {
		return errors.New("can't handle worldserver-initiated redirect, worldSocket is nil")
	}

	status, err := redirectReadyStatus(p)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("source worldserver rejected worldserver-initiated redirect with status %d, account %d, character %d", status, s.accountID, s.character.GUID)
	}

	sessionCharacterGUID := s.character.GUID
	if !s.canReconnectCharacter(sessionCharacterGUID) {
		return fmt.Errorf("stale worldserver-initiated redirect for account %d, character %d: session no longer owns character", s.accountID, sessionCharacterGUID)
	}

	redirectID := fmt.Sprintf("%d-%d-%d", s.accountID, s.character.GUID, time.Now().UnixNano())
	redirectStarted := time.Now()
	s.pendingRedirectID = redirectID
	s.pendingRedirectAt = redirectStarted
	oldSocket := s.worldSocket
	oldConnection := oldSocket.Address()
	s.logger.Info().
		Str("redirect", redirectID).
		Uint32("account", s.accountID).
		Uint64("character", s.character.GUID).
		Str("source", oldConnection).
		Msg("Starting worldserver-initiated redirect")

	char, socket, worldserverID, err := s.connectToGameServer(ctx, s.character.GUID, nil, nil)
	if err != nil {
		if s.pendingRedirectID == redirectID {
			s.pendingRedirectID = ""
			s.pendingRedirectAt = time.Time{}
		}
		return fmt.Errorf("failed to connect player to the new gameserver during redirect %s: %w", redirectID, err)
	}
	if !s.canReconnectCharacter(sessionCharacterGUID) {
		socket.Close()
		if s.pendingRedirectID == redirectID {
			s.pendingRedirectID = ""
			s.pendingRedirectAt = time.Time{}
		}
		return fmt.Errorf("stale worldserver-initiated redirect for account %d, character %d: session no longer owns character", s.accountID, sessionCharacterGUID)
	}

	resp := packet.NewWriterWithSize(packet.SMsgNewWorld, 0)
	resp.Uint32(char.Map)
	resp.Float32(char.PositionX)
	resp.Float32(char.PositionY)
	resp.Float32(char.PositionZ)
	resp.Float32(0.0)
	s.gameSocket.Send(resp)

	if s.character != nil {
		oldSocket.Close()
		s.worldSocket = socket
		s.worldserverID = worldserverID
		s.pendingRedirectID = redirectID
		s.pendingRedirectAt = redirectStarted
		s.resetPlayerAuraState()
	}

	if s.showGameserverConnChangeToClient {
		s.SendSysMessage(fmt.Sprintf("You have been redirected from %s to %s gameserver.", oldConnection, s.worldSocket.Address()))
	}
	s.logger.Info().
		Str("redirect", redirectID).
		Uint32("account", s.accountID).
		Uint64("character", s.character.GUID).
		Uint32("map", char.Map).
		Str("source", oldConnection).
		Str("target", s.worldSocket.Address()).
		Dur("duration", time.Since(redirectStarted)).
		Msg("Worldserver-initiated redirect connected to target")

	return nil
}
