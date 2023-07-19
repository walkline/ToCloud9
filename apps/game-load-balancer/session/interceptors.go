package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	root "github.com/walkline/ToCloud9/apps/game-load-balancer"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/sockets"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
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

	if s.character.Level != uint32(lvl) {
		s.charsUpdsBarrier.UpdateLevel(s.character.GUID, uint8(lvl))
		s.character.Level = uint32(lvl)
	}

	return nil
}

func (s *GameSession) InterceptNewWorld(ctx context.Context, p *packet.Packet) error {
	mapID := p.Reader().Uint32()
	s.teleportingToNewMap = &mapID
	s.gameSocket.SendPacket(p)
	return nil
}

func (s *GameSession) InterceptMoveWorldPortAck(ctx context.Context, p *packet.Packet) error {
	if s.worldSocket == nil {
		return errors.New("can't handle InterceptMoveWorldPortAck, worldSocket is nil")
	}
	s.worldSocket.SendPacket(p)

	if s.teleportingToNewMap == nil {
		return nil
	}
	mapID := *s.teleportingToNewMap
	s.teleportingToNewMap = nil

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

	if serversResult.GameServers[0].Address == s.worldSocket.Address() {
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
		_, socket, err = s.connectToGameServer(context.Background(), charGUID, &mapID)
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
		}
	}(s.character.GUID)

	return nil
}
