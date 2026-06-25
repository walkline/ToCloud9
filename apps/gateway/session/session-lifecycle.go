package session

import (
	"context"
	"time"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/service"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/groupstatetrace"
)

func (s *GameSession) HandlePlayerWorldActivePacket(_ context.Context, p *packet.Packet) error {
	s.markPlayerWorldActive()

	if p.Source == packet.SourceGameClient && s.worldSocket != nil {
		s.worldSocket.SendPacket(p)
		return nil
	}

	if p.Source == packet.SourceWorldServer && s.gameSocket != nil {
		s.gameSocket.SendPacket(p)
	}

	return nil
}

func (s *GameSession) InterceptLogoutComplete(_ context.Context, p *packet.Packet) error {
	if s.pendingRedirectID != "" {
		if s.logger != nil {
			s.logger.Debug().
				Str("redirect", s.pendingRedirectID).
				Uint32("account", s.accountID).
				Msg("TC9 suppressing logout complete during cross-worldserver redirect")
		}
		if s.worldSocket != nil {
			socket := s.worldSocket
			s.worldSocket = nil
			socket.Close()
		}
		return nil
	}

	s.gameSocket.SendPacket(p)
	s.onLoggedOut()

	if s.worldSocket != nil {
		socket := s.worldSocket
		s.worldSocket = nil
		socket.Close()
	}

	return nil
}

func (s *GameSession) markPlayerWorldActive() {
	if s.character == nil {
		return
	}

	s.playerWorldActive = true
	s.publishCharacterLoggedIn()
}

func (s *GameSession) publishCharacterLoggedIn() {
	if s.character == nil || s.characterLoggedInPublished || s.eventsProducer == nil {
		return
	}

	err := s.eventsProducer.CharacterLoggedIn(&events.GWEventCharacterLoggedInPayload{
		RealmID:     root.RealmID,
		GatewayID:   root.RetrievedGatewayID,
		CharGUID:    s.character.GUID,
		CharName:    s.character.Name,
		CharRace:    s.character.Race,
		CharClass:   s.character.Class,
		CharGender:  s.character.Gender,
		CharLevel:   s.character.Level,
		CharZone:    s.character.Zone,
		CharMap:     s.character.Map,
		CharPosX:    s.character.PositionX,
		CharPosY:    s.character.PositionY,
		CharPosZ:    s.character.PositionZ,
		CharGuildID: s.character.GuildID,
		AccountID:   s.character.AccountID,
	})
	if err != nil {
		s.logger.Err(err).Msg("can't send login event")
		return
	}

	s.characterLoggedInPublished = true
}

func (s *GameSession) publishOfflinePlayerStateSnapshot() {
	if s.character == nil || s.playerStateUpdatesBarrier == nil {
		return
	}

	online := false
	level := s.character.Level
	classID := s.character.Class
	zoneID := s.character.Zone
	mapID := s.character.Map
	zero := uint32(0)
	powerType := uint8(0)

	snapshot := service.PlayerStateSnapshot{
		MemberGUID:          currentCharacterMemberGUID(s.character.GUID),
		SourceWorldserverID: s.currentWorldserverSourceID(),
		Online:              &online,
		Level:               &level,
		Class:               &classID,
		ZoneID:              &zoneID,
		MapID:               &mapID,
		Health:              &zero,
		MaxHealth:           &zero,
		PowerType:           &powerType,
		Power:               &zero,
		MaxPower:            &zero,
		AurasKnown:          true,
		TimestampMs:         uint64(time.Now().UnixMilli()),
	}
	if event := groupstatetrace.Event(s.logger, "gateway.offline.snapshot", snapshot.MemberGUID); event != nil {
		traceSessionPlayerStateSnapshot(event, snapshot).
			Uint32("accountID", s.accountID).
			Msg(groupstatetrace.Message)
	}
	s.playerStateUpdatesBarrier.Update(snapshot)
}

func (s *GameSession) currentWorldserverSourceID() string {
	if s.worldserverID != "" {
		return s.worldserverID
	}
	if s.worldSocket != nil {
		return s.worldSocket.Address()
	}
	return ""
}

func (s *GameSession) isCurrentWorldserverSourceID(sourceWorldserverID string) bool {
	if sourceWorldserverID == "" {
		return false
	}
	if s.worldserverID != "" && s.worldserverID == sourceWorldserverID {
		return true
	}
	return s.worldSocket != nil && s.worldSocket.Address() == sourceWorldserverID
}
