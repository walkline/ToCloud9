package session

import (
	"context"
	"time"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
)

// layerSafetyState contains transient client/world state that must settle before
// the gateway disconnects a character from one layer and attaches it to another.
type layerSafetyState struct {
	inCombat, falling, looting, trading, casting, releasing bool
	lastDamagedAt                                           time.Time
}

func (s *GameSession) layerSwitchSafe(now time.Time) bool {
	if s.character == nil || s.character.CurHP == 0 || s.layerSafety.inCombat ||
		s.layerSafety.falling || s.layerSafety.looting || s.layerSafety.trading ||
		s.layerSafety.casting || s.layerSafety.releasing {
		return false
	}
	if !s.layerSafety.lastDamagedAt.IsZero() && now.Sub(s.layerSafety.lastDamagedAt) < 30*time.Second {
		return false
	}
	// Layering is only valid in the four open-world continent maps. Every
	// dungeon, raid, battleground and arena in 3.3.5a uses another map ID.
	switch s.character.Map {
	case 0, 1, 530, 571:
		return true
	default:
		return false
	}
}

// HandleLayerSafetyPacket observes interaction state while preserving normal
// packet forwarding in both directions.
func (s *GameSession) HandleLayerSafetyPacket(_ context.Context, p *packet.Packet) error {
	client := p.Source == packet.SourceGameClient
	switch p.Opcode {
	case packet.CMsgLoot:
		s.layerSafety.looting = true
	case packet.CMsgLootRelease, packet.SMsgLootReleaseResponse:
		s.layerSafety.looting = false
	case packet.CMsgInitiateTrade, packet.CMsgBeginTrade:
		s.layerSafety.trading = true
	case packet.CMsgCancelTrade:
		s.layerSafety.trading = false
	case packet.SMsgTradeStatus:
		status := p.Reader().Uint32()
		s.layerSafety.trading = status == 1 || status == 2 || status == 4 || status == 7
	case packet.CMsgCastSpell:
		s.layerSafety.casting = true
	case packet.CMsgCancelCast, packet.CMsgCancelChannelling, packet.SMsgCastFailed,
		packet.SMsgSpellFailure, packet.SMsgSpellFailedOther, packet.SMsgSpellGo, packet.MsgChannelUpdate:
		s.layerSafety.casting = false
	case packet.CMsgRePopRequest:
		s.layerSafety.releasing = true
	case packet.SMsgAttackStart:
		r := p.Reader()
		attacker, victim := r.Uint64(), r.Uint64()
		if attacker == s.character.GUID || victim == s.character.GUID {
			s.layerSafety.inCombat = true
		}
	case packet.SMsgAttackStop, packet.SMsgCancelCombat:
		s.layerSafety.inCombat = false
	}
	if client {
		if s.worldSocket != nil {
			s.worldSocket.SendPacket(p)
		}
	} else if s.gameSocket != nil {
		s.gameSocket.SendPacket(p)
	}
	return nil
}
