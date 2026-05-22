package session

import (
	"context"
	"time"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/service"
	"github.com/walkline/ToCloud9/shared/groupstatetrace"
)

const (
	maxGroupAuraSlots = 64

	auraFlagCaster   uint8 = 0x08
	auraFlagDuration uint8 = 0x20
)

func (s *GameSession) InterceptAuraUpdate(_ context.Context, p *packet.Packet) error {
	s.gameSocket.SendPacket(p)

	if p.Source != packet.SourceWorldServer {
		s.logSkippedAuraState("packet source is not worldserver", p)
		return nil
	}
	if s.character == nil {
		s.logSkippedAuraState("session has no character", p)
		return nil
	}
	if s.playerStateUpdatesBarrier == nil {
		s.logSkippedAuraState("player state barrier is not configured", p)
		return nil
	}

	if s.pendingRedirectID != "" {
		s.logSkippedAuraState("redirect is pending", p)
		return nil
	}
	if !s.playerWorldActive {
		s.logSkippedAuraState("player is not world-active", p)
		return nil
	}

	memberGUID, updated, err := s.applyPlayerAuraUpdatePacket(p)
	if err != nil {
		if s.logger != nil {
			s.logger.Debug().Err(err).Msg("can't parse aura update packet for player aura state extraction")
		}
		return nil
	}
	if !updated {
		return nil
	}

	s.publishPlayerAuraState(memberGUID)
	return nil
}

func (s *GameSession) applyPlayerAuraUpdatePacket(p *packet.Packet) (uint64, bool, error) {
	r := p.Reader()
	rawGUID := r.ReadGUID()
	if err := r.Error(); err != nil {
		return 0, false, err
	}

	currentMemberGUID := currentCharacterMemberGUID(s.character.GUID)

	memberGUID := playerDBGUIDFromObjectUpdateGUID(rawGUID)
	if memberGUID == 0 {
		if s.logger != nil {
			s.logger.Debug().
				Str("opcode", p.Opcode.String()).
				Uint64("rawGUID", rawGUID).
				Uint64("sessionCharacterGUID", s.character.GUID).
				Msg("TC9 skipping player aura state: guid is not a player")
		}
		return 0, false, nil
	}

	auras := s.ensurePlayerAuraState(memberGUID)
	if p.Opcode == packet.SMsgAuraUpdateAll {
		auras = map[uint8]service.PlayerAuraSnapshot{}
		s.setPlayerAuraState(memberGUID, auras)
		for r.Left() > 0 {
			aura, active, err := readPlayerAuraUpdate(r)
			if err != nil {
				return 0, false, err
			}
			if active && aura.Slot < maxGroupAuraSlots {
				auras[aura.Slot] = aura
			}
		}
		return memberGUID, true, nil
	}

	aura, active, err := readPlayerAuraUpdate(r)
	if err != nil {
		return 0, false, err
	}
	if aura.Slot >= maxGroupAuraSlots {
		if s.logger != nil {
			s.logger.Debug().
				Str("opcode", p.Opcode.String()).
				Uint8("slot", aura.Slot).
				Uint64("memberGUID", memberGUID).
				Msg("TC9 skipping player aura state: aura slot is outside group frame range")
		}
		return memberGUID, false, nil
	}
	if active {
		auras[aura.Slot] = aura
	} else {
		delete(auras, aura.Slot)
	}

	if memberGUID == currentMemberGUID {
		s.playerAuraState = auras
	}

	return memberGUID, true, nil
}

func (s *GameSession) logSkippedAuraState(reason string, p *packet.Packet) {
	if s.logger == nil {
		return
	}

	s.logger.Debug().
		Str("reason", reason).
		Str("opcode", p.Opcode.String()).
		Uint32("source", uint32(p.Source)).
		Uint64("sessionCharacterGUID", func() uint64 {
			if s.character == nil {
				return 0
			}
			return s.character.GUID
		}()).
		Msg("TC9 skipping player aura state")
}

func readPlayerAuraUpdate(r *packet.Reader) (service.PlayerAuraSnapshot, bool, error) {
	slot := r.Uint8()
	spellID := r.Uint32()
	if err := r.Error(); err != nil {
		return service.PlayerAuraSnapshot{}, false, err
	}

	aura := service.PlayerAuraSnapshot{Slot: slot, SpellID: spellID}
	if spellID == 0 {
		return aura, false, nil
	}

	flags := r.Uint8()
	_ = r.Uint8() // caster level, not used by party/raid member stats
	_ = r.Uint8() // stack amount or charges, not used by party/raid member stats
	if flags&auraFlagCaster == 0 {
		_ = r.ReadGUID()
	}
	if flags&auraFlagDuration != 0 {
		_ = r.Uint32()
		_ = r.Uint32()
	}
	if err := r.Error(); err != nil {
		return service.PlayerAuraSnapshot{}, false, err
	}

	aura.Flags = flags
	return aura, true, nil
}

func (s *GameSession) resetPlayerAuraState() {
	s.playerAuraState = map[uint8]service.PlayerAuraSnapshot{}
	s.observedPlayerAuraStates = map[uint64]map[uint8]service.PlayerAuraSnapshot{}
}

func (s *GameSession) publishPlayerAuraState(memberGUID uint64) {
	if memberGUID == 0 || s.character == nil || s.playerStateUpdatesBarrier == nil {
		return
	}

	currentMemberGUID := currentCharacterMemberGUID(s.character.GUID)

	snapshot := service.PlayerStateSnapshot{
		MemberGUID:  memberGUID,
		AurasKnown:  true,
		Auras:       s.playerAuraStateSlice(memberGUID),
		TimestampMs: uint64(time.Now().UnixMilli()),
	}
	ghost := playerAurasContainGhost(snapshot.Auras)
	snapshot.Ghost = &ghost
	if memberGUID == currentMemberGUID {
		s.fillPlayerStateSnapshotSessionFields(&snapshot)
	} else {
		snapshot.SourceWorldserverID = s.currentWorldserverSourceID()
	}

	if s.logger != nil {
		s.logger.Debug().
			Uint64("sessionMemberGUID", currentMemberGUID).
			Uint64("memberGUID", memberGUID).
			Bool("ownMember", memberGUID == currentMemberGUID).
			Str("sourceWorldserverID", snapshot.SourceWorldserverID).
			Int("auraCount", len(snapshot.Auras)).
			Msg("TC9 publishing player aura state")
	}
	if event := groupstatetrace.Event(s.logger, "gateway.aura.snapshot", snapshot.MemberGUID); event != nil {
		traceSessionPlayerStateSnapshot(event, snapshot).
			Uint32("accountID", s.accountID).
			Bool("ownMember", memberGUID == currentMemberGUID).
			Msg(groupstatetrace.Message)
	}

	s.playerStateUpdatesBarrier.Update(snapshot)
}

func playerAurasContainGhost(auras []service.PlayerAuraSnapshot) bool {
	for _, aura := range auras {
		switch aura.SpellID {
		case ghostAuraSpellID, wispSpiritAuraSpellID:
			return true
		}
	}
	return false
}

func (s *GameSession) playerAuraStateSlice(memberGUID uint64) []service.PlayerAuraSnapshot {
	aurasBySlot := s.playerAuraStateForMember(memberGUID)
	if len(aurasBySlot) == 0 {
		return nil
	}

	auras := make([]service.PlayerAuraSnapshot, 0, len(aurasBySlot))
	for _, aura := range aurasBySlot {
		auras = append(auras, aura)
	}

	return auras
}

func (s *GameSession) playerAuraStateForMember(memberGUID uint64) map[uint8]service.PlayerAuraSnapshot {
	if s.observedPlayerAuraStates != nil {
		if auras := s.observedPlayerAuraStates[memberGUID]; auras != nil {
			return auras
		}
	}

	if s.character != nil && memberGUID == currentCharacterMemberGUID(s.character.GUID) {
		return s.playerAuraState
	}

	return nil
}

func (s *GameSession) ensurePlayerAuraState(memberGUID uint64) map[uint8]service.PlayerAuraSnapshot {
	if s.observedPlayerAuraStates == nil {
		s.observedPlayerAuraStates = map[uint64]map[uint8]service.PlayerAuraSnapshot{}
	}

	auras := s.observedPlayerAuraStates[memberGUID]
	if auras == nil && s.character != nil && memberGUID == currentCharacterMemberGUID(s.character.GUID) {
		auras = s.playerAuraState
	}
	if auras == nil {
		auras = map[uint8]service.PlayerAuraSnapshot{}
	}
	s.setPlayerAuraState(memberGUID, auras)

	return auras
}

func (s *GameSession) setPlayerAuraState(memberGUID uint64, auras map[uint8]service.PlayerAuraSnapshot) {
	if s.observedPlayerAuraStates == nil {
		s.observedPlayerAuraStates = map[uint64]map[uint8]service.PlayerAuraSnapshot{}
	}

	s.observedPlayerAuraStates[memberGUID] = auras
	if s.character != nil && memberGUID == currentCharacterMemberGUID(s.character.GUID) {
		s.playerAuraState = auras
	}
}
