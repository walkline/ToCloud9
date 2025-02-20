package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	root "github.com/walkline/ToCloud9/apps/game-load-balancer"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/sockets"
	"github.com/walkline/ToCloud9/gen/characters/pb"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
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

	return nil
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

func (s *GameSession) InterceptNewWorld(ctx context.Context, p *packet.Packet) error {
	mapID := p.Reader().Uint32()
	if s.character.ignoreNextInterceptToNewMap == nil || mapID != *s.character.ignoreNextInterceptToNewMap {
		s.teleportingToNewMap = &mapID
	}
	s.gameSocket.SendPacket(p)
	return nil
}

func (s *GameSession) InterceptMoveWorldPortAck(ctx context.Context, p *packet.Packet) error {
	if s.worldSocket == nil {
		return errors.New("can't handle InterceptMoveWorldPortAck, worldSocket is nil")
	}
	s.worldSocket.SendPacket(p)

	if s.teleportingToNewMap != nil && s.character.GroupMangedByGameServer {
		s.character.GroupMangedByGameServer = false
		if err := s.LoadGroupForPlayer(ctx); err != nil {
			return err
		}
	}

	if s.teleportingToNewMap == nil {
		return nil
	}
	mapID := *s.teleportingToNewMap
	s.teleportingToNewMap = nil
	s.character.ignoreNextInterceptToNewMap = nil

	serversResult, err := s.serversRegistryClient.AvailableGameServersForMapAndRealm(s.ctx, &pbServ.AvailableGameServersForMapAndRealmRequest{
		Api:     root.SupportedCharServiceVer,
		RealmID: root.RealmID,
		MapID:   mapID,
	})

	if err != nil {
		return err
	}

	if len(serversResult.GameServers) == 0 {
		return fmt.Errorf("%w, mapID %v", worldConnectErrInstanceNotFound, mapID)
	}

	oldServerAddress := s.worldSocket.Address()
	desiredServerAddress := serversResult.GameServers[0].Address

	if desiredServerAddress == oldServerAddress {
		return nil
	}

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

	go func(charGUID uint64) {
		var err error
		var socket sockets.Socket
		_, socket, err = s.connectToGameServer(context.Background(), charGUID, &mapID, nil)
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to reconnect player to the world")
			resp := packet.NewWriterWithSize(packet.SMsgCharacterLoginFailed, 1)
			resp.Uint8(uint8(packet.LoginErrorCodeWorldServerIsDown))
			s.gameSocket.Send(resp)
			return
		}

		s.gameSocket.SendPacket(p)

		// we need to modify session in a safe thread (goroutine)
		s.sessionSafeFuChan <- func(session *GameSession) {
			if session.character != nil {
				session.worldSocket = socket
			}

			if session.showGameserverConnChangeToClient {
				session.SendSysMessage(fmt.Sprintf("You have been redirected from %s to %s gameserver.", oldServerAddress, desiredServerAddress))
			}
		}
	}(s.character.GUID)

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
	newPckt := packet.NewWriterWithSize(packet.SMsgNameQueryResponse, 0)
	newPckt.GUID(charGUID)

	newPckt.Uint8(0)
	newPckt.String(playerData.CharName)
	if g.GetRealmID() == uint16(root.RealmID) {
		newPckt.Uint8(0)
	} else {
		var name string
		name, err = s.realmNamesService.NameByID(ctx, uint32(g.GetRealmID()))
		if err != nil {
			name = "unknown realm"
		}
		newPckt.String(name)
	}
	newPckt.Uint8(uint8(playerData.CharRace))
	newPckt.Uint8(uint8(playerData.CharGender))
	newPckt.Uint8(uint8(playerData.CharClass))
	newPckt.Uint8(0)

	s.gameSocket.Send(newPckt)

	return nil
}

func (s *GameSession) HandleReadyForRedirectRequest(ctx context.Context, p *packet.Packet) error {
	oldConnection := s.worldSocket.Address()

	char, socket, err := s.connectToGameServer(context.TODO(), s.character.GUID, nil, nil)
	if err != nil {
		return errors.New("failed to connect player to the new gameserver")
	}

	resp := packet.NewWriterWithSize(packet.SMsgNewWorld, 0)
	resp.Uint32(char.Map)
	resp.Float32(char.PositionX)
	resp.Float32(char.PositionY)
	resp.Float32(char.PositionZ)
	resp.Float32(0.0)
	s.gameSocket.Send(resp)

	if s.character != nil {
		s.worldSocket = socket
	}

	if s.showGameserverConnChangeToClient {
		s.SendSysMessage(fmt.Sprintf("You have been redirected from %s to %s gameserver.", oldConnection, s.worldSocket.Address()))
	}

	return nil
}
