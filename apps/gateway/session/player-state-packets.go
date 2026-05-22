package session

import (
	"context"
	"fmt"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/service"
	"github.com/walkline/ToCloud9/shared/groupstatetrace"
	"github.com/walkline/ToCloud9/shared/wow"
)

const (
	groupUpdateFlagPosition uint32 = 0x00000100
)

func (s *GameSession) InterceptDirectPlayerStateUpdate(_ context.Context, p *packet.Packet) error {
	s.gameSocket.SendPacket(p)

	if !s.canObserveOwnPlayerStateFromWorld(p) {
		return nil
	}

	snapshot, err := extractDirectPlayerStateSnapshot(p.Opcode, p.Data)
	if err != nil {
		s.logger.Debug().Err(err).Str("opcode", p.Opcode.String()).Msg("can't parse direct player state update")
		return nil
	}

	s.publishOwnPlayerStateSnapshot(snapshot, "gateway.direct_packet.snapshot", false)
	return nil
}

func (s *GameSession) InterceptPartyMemberStats(_ context.Context, p *packet.Packet) error {
	if s.shouldSuppressWorldGroupPresentation(p) {
		return nil
	}

	s.gameSocket.SendPacket(p)

	if !s.canPublishOwnPlayerStateFromWorld(p) {
		return nil
	}

	snapshot, err := extractPlayerStateSnapshotFromPartyMemberStats(p.Data, p.Opcode == packet.SMsgPartyMemberStatsFull)
	if err != nil {
		s.logger.Debug().Err(err).Str("opcode", p.Opcode.String()).Msg("can't parse party member stats for player state")
		return nil
	}

	s.publishOwnPlayerStateSnapshot(snapshot, "gateway.party_stats.snapshot", true)
	return nil
}

func (s *GameSession) canPublishOwnPlayerStateFromWorld(p *packet.Packet) bool {
	if !s.canObserveOwnPlayerStateFromWorld(p) {
		return false
	}
	return s.playerWorldActive
}

func (s *GameSession) canObserveOwnPlayerStateFromWorld(p *packet.Packet) bool {
	if p.Source != packet.SourceWorldServer || s.character == nil || s.playerStateUpdatesBarrier == nil {
		return false
	}
	if s.pendingRedirectID != "" {
		return false
	}
	return true
}

func (s *GameSession) publishOwnPlayerStateSnapshot(snapshot service.PlayerStateSnapshot, traceStage string, flushIfComplete bool) {
	if snapshot.MemberGUID == 0 {
		return
	}

	if snapshot.MemberGUID != currentCharacterMemberGUID(s.character.GUID) {
		return
	}

	s.fillPlayerStateSnapshotSessionFields(&snapshot)
	if traceStage == "gateway.direct_packet.snapshot" && s.shouldDropInactiveDirectPowerSnapshot(snapshot) {
		if event := groupstatetrace.Event(s.logger, "gateway.direct_packet.drop_inactive_power", snapshot.MemberGUID); event != nil {
			expectedPowerType, _ := wow.FixedPrimaryPowerTypeForClass(*snapshot.Class)
			traceSessionPlayerStateSnapshot(event, snapshot).
				Uint8("expectedPowerType", expectedPowerType).
				Uint32("accountID", s.accountID).
				Msg(groupstatetrace.Message)
		}
		return
	}
	if event := groupstatetrace.Event(s.logger, traceStage, snapshot.MemberGUID); event != nil {
		traceSessionPlayerStateSnapshot(event, snapshot).
			Uint32("accountID", s.accountID).
			Msg(groupstatetrace.Message)
	}
	if flushIfComplete && snapshot.IsComplete() {
		s.playerStateUpdatesBarrier.UpdateAndFlush(snapshot)
		return
	}

	s.playerStateUpdatesBarrier.Update(snapshot)
}

func (s *GameSession) InterceptGroupPresentationPacket(_ context.Context, p *packet.Packet) error {
	if s.shouldSuppressWorldGroupPresentation(p) {
		return nil
	}

	s.gameSocket.SendPacket(p)
	return nil
}

func (s *GameSession) shouldSuppressWorldGroupPresentation(p *packet.Packet) bool {
	if s == nil || p == nil || p.Source != packet.SourceWorldServer {
		return false
	}
	if s.clusterGroupPresentationBlocked() {
		return true
	}

	switch p.Opcode {
	case packet.SMsgGroupDestroyed, packet.SMsgGroupList:
		return mapTransferRoutingUsesCrossrealmOwner(s.currentMapTransferRouting)
	default:
		return false
	}
}

func (s *GameSession) shouldDropInactiveDirectPowerSnapshot(snapshot service.PlayerStateSnapshot) bool {
	if snapshot.Class == nil || snapshot.PowerType == nil || snapshot.Power == nil {
		return false
	}

	expectedPowerType, fixed := wow.FixedPrimaryPowerTypeForClass(*snapshot.Class)
	return fixed && *snapshot.PowerType != expectedPowerType
}

func extractDirectPlayerStateSnapshot(opcode packet.Opcode, data []byte) (service.PlayerStateSnapshot, error) {
	r := packet.NewReaderWithData(data)
	rawGUID := r.ReadGUID()
	memberGUID := playerDBGUIDFromObjectUpdateGUID(rawGUID)
	if memberGUID == 0 {
		return service.PlayerStateSnapshot{}, nil
	}

	snapshot := service.PlayerStateSnapshot{MemberGUID: memberGUID}
	switch opcode {
	case packet.SMsgHealthUpdate:
		health := r.Uint32()
		dead := health == 0
		snapshot.Health = &health
		snapshot.Dead = &dead
	case packet.SMsgPowerUpdate:
		powerType := r.Uint8()
		power := r.Uint32()
		snapshot.PowerType = &powerType
		snapshot.Power = &power
	default:
		return service.PlayerStateSnapshot{}, fmt.Errorf("unsupported direct player state opcode %s", opcode.String())
	}

	if err := r.Error(); err != nil {
		return service.PlayerStateSnapshot{}, err
	}

	return snapshot, nil
}

func extractPlayerStateSnapshotFromPartyMemberStats(data []byte, full bool) (service.PlayerStateSnapshot, error) {
	r := packet.NewReaderWithData(data)
	if full {
		_ = r.Uint8()
	}

	rawGUID := r.ReadGUID()
	memberGUID := playerDBGUIDFromObjectUpdateGUID(rawGUID)
	if memberGUID == 0 {
		return service.PlayerStateSnapshot{}, nil
	}

	updateMask := r.Uint32()
	snapshot := service.PlayerStateSnapshot{MemberGUID: memberGUID}

	if updateMask&groupUpdateFlagStatus != 0 {
		status := r.Uint16()
		online := status&memberStatusOnline != 0
		dead := status&memberStatusDead != 0
		ghost := status&memberStatusGhost != 0
		snapshot.Online = &online
		snapshot.Dead = &dead
		snapshot.Ghost = &ghost
	}
	if updateMask&groupUpdateFlagCurHP != 0 {
		health := r.Uint32()
		snapshot.Health = &health
	}
	if updateMask&groupUpdateFlagMaxHP != 0 {
		maxHealth := r.Uint32()
		snapshot.MaxHealth = &maxHealth
	}
	if updateMask&groupUpdateFlagPowerType != 0 {
		powerType := r.Uint8()
		snapshot.PowerType = &powerType
	}
	if updateMask&groupUpdateFlagCurPower != 0 {
		power := uint32(r.Uint16())
		snapshot.Power = &power
	}
	if updateMask&groupUpdateFlagMaxPower != 0 {
		maxPower := uint32(r.Uint16())
		snapshot.MaxPower = &maxPower
	}
	if snapshot.PowerType == nil && (snapshot.Power != nil || snapshot.MaxPower != nil) {
		powerType := uint8(0)
		snapshot.PowerType = &powerType
	}
	if updateMask&groupUpdateFlagLevel != 0 {
		level := uint8(r.Uint16())
		snapshot.Level = &level
	}
	if updateMask&groupUpdateFlagZone != 0 {
		zoneID := uint32(r.Uint16())
		snapshot.ZoneID = &zoneID
	}
	if updateMask&groupUpdateFlagPosition != 0 {
		_ = r.Uint16()
		_ = r.Uint16()
	}
	if updateMask&groupUpdateFlagAuras != 0 {
		auras, err := readPlayerAurasFromStats(r)
		if err != nil {
			return service.PlayerStateSnapshot{}, err
		}
		snapshot.AurasKnown = true
		snapshot.Auras = auras
	}

	if err := r.Error(); err != nil {
		return service.PlayerStateSnapshot{}, err
	}

	return snapshot, nil
}

func readPlayerAurasFromStats(r *packet.Reader) ([]service.PlayerAuraSnapshot, error) {
	auraMask := r.Uint64()
	if err := r.Error(); err != nil {
		return nil, err
	}

	auras := make([]service.PlayerAuraSnapshot, 0)
	for slot := 0; slot < maxGroupAuraSlots; slot++ {
		if auraMask&(uint64(1)<<slot) == 0 {
			continue
		}

		spellID := r.Uint32()
		flags := r.Uint8()
		if err := r.Error(); err != nil {
			return nil, err
		}
		auras = append(auras, service.PlayerAuraSnapshot{
			Slot:    uint8(slot),
			SpellID: spellID,
			Flags:   flags,
		})
	}

	return auras, nil
}
