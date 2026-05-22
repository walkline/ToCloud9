package session

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/gen/group/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/groupstatetrace"
	wowguid "github.com/walkline/ToCloud9/shared/wow/guid"
)

const (
	readyCheckDefaultDurationMs uint32 = 35000

	groupMemberFlagAssistant  uint8 = 0x01
	groupMemberFlagMainTank   uint8 = 0x02
	groupMemberFlagMainAssist uint8 = 0x04

	subGroupSwapPendingFlag uint32 = 0x80

	memberStatusOnline uint16 = 0x0001
	memberStatusDead   uint16 = 0x0004
	memberStatusGhost  uint16 = 0x0008

	ghostAuraSpellID      uint32 = 8326
	wispSpiritAuraSpellID uint32 = 20584

	groupUpdateFlagStatus    uint32 = 0x00000001
	groupUpdateFlagCurHP     uint32 = 0x00000002
	groupUpdateFlagMaxHP     uint32 = 0x00000004
	groupUpdateFlagPowerType uint32 = 0x00000008
	groupUpdateFlagCurPower  uint32 = 0x00000010
	groupUpdateFlagMaxPower  uint32 = 0x00000020
	groupUpdateFlagLevel     uint32 = 0x00000040
	groupUpdateFlagZone      uint32 = 0x00000080
	groupUpdateFlagAuras     uint32 = 0x00000200
)

type groupMemberAuraRenderKey struct {
	realmID    uint32
	memberGUID uint64
}

func (s *GameSession) HandleRaidReadyCheck(ctx context.Context, p *packet.Packet) error {
	if p.Source == packet.SourceWorldServer {
		// Gateway-owned ready-check events already render these packets.
		return nil
	}

	r := p.Reader()

	if r.Left() == 0 {
		_, err := s.groupServiceClient.StartReadyCheck(ctx, &pb.StartReadyCheckRequest{
			Api:        root.SupportedGroupServiceVer,
			RealmID:    root.RealmID,
			LeaderGUID: s.character.GUID,
			DurationMs: readyCheckDefaultDurationMs,
		})
		if err != nil {
			return NewGroupServiceUnavailableErr(err)
		}

		return nil
	}

	state := r.Uint8()
	if err := r.Error(); err != nil {
		return err
	}

	_, err := s.groupServiceClient.SetReadyCheckMemberState(ctx, &pb.SetReadyCheckMemberStateRequest{
		Api:        root.SupportedGroupServiceVer,
		RealmID:    root.RealmID,
		MemberGUID: s.character.GUID,
		State:      uint32(state),
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	return nil
}

func (s *GameSession) HandleRaidReadyCheckConfirm(_ context.Context, _ *packet.Packet) error {
	return nil
}

func (s *GameSession) HandleRaidReadyCheckFinished(ctx context.Context, p *packet.Packet) error {
	if p.Source == packet.SourceWorldServer {
		// Gateway-owned ready-check events already render these packets.
		return nil
	}

	_, err := s.groupServiceClient.FinishReadyCheck(ctx, &pb.FinishReadyCheckRequest{
		Api:        root.SupportedGroupServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	return nil
}

func (s *GameSession) HandleGroupChangeSubGroup(ctx context.Context, p *packet.Packet) error {
	if p.Source == packet.SourceWorldServer {
		s.gameSocket.WriteChannel() <- p
		return nil
	}

	r := p.Reader()
	memberName := r.String()
	subGroup := r.Uint8()

	if err := r.Error(); err != nil {
		return err
	}

	groupResp, err := s.groupServiceClient.GetGroupByMember(ctx, &pb.GetGroupByMemberRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}
	if groupResp.Group == nil {
		return nil
	}

	member := s.groupMemberByName(ctx, groupResp.Group, memberName)
	if member == nil {
		return fmt.Errorf("group member %q not found", memberName)
	}

	_, err = s.groupServiceClient.ChangeMemberSubGroup(ctx, &pb.ChangeMemberSubGroupRequest{
		Api:         root.SupportedGroupServiceVer,
		RealmID:     root.RealmID,
		UpdaterGUID: s.character.GUID,
		MemberGUID:  member.Guid,
		SubGroup:    uint32(subGroup),
	})
	if err != nil {
		return s.deniedGroupMutationErr(ctx, err)
	}

	return nil
}

func (s *GameSession) HandleGroupSwapSubGroup(ctx context.Context, p *packet.Packet) error {
	if p.Source == packet.SourceWorldServer {
		s.gameSocket.WriteChannel() <- p
		return nil
	}

	r := p.Reader()
	memberName1 := r.String()
	memberName2 := r.String()

	if err := r.Error(); err != nil {
		return err
	}

	groupResp, err := s.groupServiceClient.GetGroupByMember(ctx, &pb.GetGroupByMemberRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}
	if groupResp.Group == nil {
		return nil
	}

	member1 := s.groupMemberByName(ctx, groupResp.Group, memberName1)
	if member1 == nil {
		return fmt.Errorf("group member %q not found", memberName1)
	}

	member2 := s.groupMemberByName(ctx, groupResp.Group, memberName2)
	if member2 == nil {
		return fmt.Errorf("group member %q not found", memberName2)
	}

	if member1.SubGroup == member2.SubGroup {
		return nil
	}

	if _, err = s.groupServiceClient.ChangeMemberSubGroup(ctx, &pb.ChangeMemberSubGroupRequest{
		Api:         root.SupportedGroupServiceVer,
		RealmID:     root.RealmID,
		UpdaterGUID: s.character.GUID,
		MemberGUID:  member1.Guid,
		SubGroup:    member2.SubGroup | subGroupSwapPendingFlag,
	}); err != nil {
		return s.deniedGroupMutationErr(ctx, err)
	}

	_, err = s.groupServiceClient.ChangeMemberSubGroup(ctx, &pb.ChangeMemberSubGroupRequest{
		Api:         root.SupportedGroupServiceVer,
		RealmID:     root.RealmID,
		UpdaterGUID: s.character.GUID,
		MemberGUID:  member2.Guid,
		SubGroup:    member1.SubGroup,
	})
	if err != nil {
		return s.deniedGroupMutationErr(ctx, err)
	}

	return nil
}

func (s *GameSession) deniedGroupMutationErr(ctx context.Context, err error) error {
	if isGroupPermissionError(err) {
		if syncErr := s.SendCurrentGroupUpdate(ctx); syncErr != nil {
			s.logger.Debug().Err(syncErr).Msg("can't resync group after denied group mutation")
		}
	}

	return NewGroupServiceUnavailableErr(err)
}

func (s *GameSession) HandleGroupAssistantLeader(ctx context.Context, p *packet.Packet) error {
	if p.Source == packet.SourceWorldServer {
		s.gameSocket.WriteChannel() <- p
		return nil
	}

	r := p.Reader()

	memberGUID := readGUIDThenBoolCompatible(r)
	apply := readLastBoolCompatible(r)

	if err := r.Error(); err != nil {
		return err
	}

	return s.setGroupMemberFlag(ctx, memberGUID, groupMemberFlagAssistant, apply)
}

func (s *GameSession) HandlePartyAssignment(ctx context.Context, p *packet.Packet) error {
	if p.Source == packet.SourceWorldServer {
		s.gameSocket.WriteChannel() <- p
		return nil
	}

	r := p.Reader()

	assignment := r.Uint8()
	apply := r.Uint8() != 0
	memberGUID := readRemainingGUIDCompatible(r)

	if err := r.Error(); err != nil {
		return err
	}

	var flag uint8
	switch assignment {
	case 0:
		flag = groupMemberFlagMainTank
	case 1:
		flag = groupMemberFlagMainAssist
	default:
		return nil
	}

	return s.setGroupMemberFlag(ctx, memberGUID, flag, apply)
}

func (s *GameSession) HandleResetInstances(ctx context.Context, p *packet.Packet) error {
	if p.Source == packet.SourceWorldServer {
		s.gameSocket.WriteChannel() <- p
		return nil
	}

	_, err := s.groupServiceClient.ResetInstance(ctx, &pb.ResetInstanceRequest{
		Api:        root.SupportedGroupServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		MapID:      0,
		Difficulty: 0,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	return nil
}

func (s *GameSession) HandleSetSavedInstanceExtend(ctx context.Context, p *packet.Packet) error {
	if p.Source == packet.SourceWorldServer {
		s.gameSocket.WriteChannel() <- p
		return nil
	}

	r := p.Reader()

	mapID := r.Uint32()
	difficulty := r.Uint32()
	extended := r.Uint8() != 0

	if err := r.Error(); err != nil {
		return err
	}

	_, err := s.groupServiceClient.SetInstanceBindExtension(ctx, &pb.SetInstanceBindExtensionRequest{
		Api:        root.SupportedGroupServiceVer,
		RealmID:    root.RealmID,
		PlayerGUID: s.character.GUID,
		MapID:      mapID,
		Difficulty: difficulty,
		Extended:   extended,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	return nil
}

func (s *GameSession) HandleEventGroupReadyCheckStarted(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventReadyCheckStartedPayload)

	resp := packet.NewWriterWithSize(packet.MsgRaidReadyCheck, 8)
	resp.Uint64(playerObjectGUIDForRealm(eventData.RealmID, eventData.LeaderGUID))
	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) HandleEventGroupReadyCheckMemberState(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*events.GroupEventReadyCheckMemberStatePayload)

	if ok, err := s.isCurrentCharacterGroupLeaderOrAssistant(ctx, eventData.RealmID, eventData.GroupID); err != nil || !ok {
		return err
	}

	resp := packet.NewWriterWithSize(packet.MsgRaidReadyCheckConfirm, 9)
	resp.Uint64(playerObjectGUIDForRealm(eventData.RealmID, eventData.MemberGUID))
	if eventData.State == 1 {
		resp.Uint8(1)
	} else {
		resp.Uint8(0)
	}
	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) HandleEventGroupReadyCheckFinished(_ context.Context, _ *eBroadcaster.Event) error {
	resp := packet.NewWriterWithSize(packet.MsgRaidReadyCheckFinished, 0)
	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) HandleEventGroupMemberSubGroupChanged(ctx context.Context, e *eBroadcaster.Event) error {
	if s.character == nil {
		return nil
	}

	// AzerothCore applies this event through libsidecar and emits the roster.
	return nil
}

func (s *GameSession) HandleEventGroupMemberFlagsChanged(ctx context.Context, e *eBroadcaster.Event) error {
	if s.character == nil {
		return nil
	}

	// AzerothCore applies this event through libsidecar and emits the roster.
	return nil
}

func (s *GameSession) HandleEventGroupMemberStateChanged(ctx context.Context, e *eBroadcaster.Event) error {
	if s.clusterGroupPresentationBlocked() {
		return nil
	}

	eventData := e.Payload.(*events.GroupEventMemberStateChangedPayload)
	return s.sendGroupMemberStateChanged(eventData)
}

func (s *GameSession) HandleEventGroupMemberStatesChanged(ctx context.Context, e *eBroadcaster.Event) error {
	if s.clusterGroupPresentationBlocked() {
		return nil
	}

	eventData := e.Payload.(*events.GroupEventMemberStatesChangedPayload)
	for _, state := range eventData.States {
		payload := groupMemberStateChangedPayloadFromBatch(eventData, state)
		if err := s.sendGroupMemberStateChanged(&payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *GameSession) sendGroupMemberStateChanged(eventData *events.GroupEventMemberStateChangedPayload) error {
	if s.shouldDropSourceWorldMemberStateEcho(eventData) {
		return nil
	}

	resp := packet.NewWriterWithSize(packet.SMsgPartyMemberStats, 64)
	resp.GUID(playerObjectGUIDForRealm(eventData.RealmID, eventData.MemberGUID))

	updateMask := groupUpdateFlagStatus
	hasHealth := eventData.Online && eventData.MaxHealth > 0
	hasPower := eventData.Online && eventData.MaxPower > 0

	if hasHealth {
		updateMask |= groupUpdateFlagCurHP | groupUpdateFlagMaxHP
	}

	if hasPower {
		updateMask |= groupUpdateFlagPowerType | groupUpdateFlagCurPower | groupUpdateFlagMaxPower
	}

	if eventData.Level != 0 {
		updateMask |= groupUpdateFlagLevel
	}

	if eventData.ZoneID != 0 {
		updateMask |= groupUpdateFlagZone
	}

	renderAuras := compactGroupMemberAuras(eventData.Auras)

	if eventData.Online && eventData.AurasKnown {
		updateMask |= groupUpdateFlagAuras
	}

	status := groupMemberStatusFromState(eventData, renderAuras)

	if event := groupstatetrace.Event(s.logger, "gateway.member_state.packet", eventData.MemberGUID); event != nil {
		event.
			Uint32("realmID", eventData.RealmID).
			Uint64("memberGUID", eventData.MemberGUID).
			Uint32("accountID", s.accountID).
			Str("sourceGatewayID", eventData.SourceGatewayID).
			Str("sourceWorldserverID", eventData.SourceWorldserverID).
			Bool("online", eventData.Online).
			Uint8("level", eventData.Level).
			Uint8("class", eventData.Class).
			Uint32("zoneID", eventData.ZoneID).
			Uint32("mapID", eventData.MapID).
			Bool("hasHealth", hasHealth).
			Uint32("health", eventData.Health).
			Uint32("maxHealth", eventData.MaxHealth).
			Bool("hasPower", hasPower).
			Uint8("powerType", eventData.PowerType).
			Uint32("power", eventData.Power).
			Uint32("maxPower", eventData.MaxPower).
			Bool("dead", eventData.Dead).
			Bool("deadKnown", eventData.DeadKnown).
			Bool("ghost", eventData.Ghost).
			Bool("ghostKnown", eventData.GhostKnown).
			Bool("aurasKnown", eventData.AurasKnown).
			Int("auraCount", len(renderAuras)).
			Str("auraSpells", formatGroupMemberAuraTrace(renderAuras)).
			Uint16("status", status).
			Uint32("updateMask", updateMask).
			Msg(groupstatetrace.Message)
	}

	resp.Uint32(updateMask)

	resp.Uint16(status)

	if hasHealth {
		health := eventData.Health
		if health > eventData.MaxHealth {
			health = eventData.MaxHealth
		}

		resp.Uint32(health)
		resp.Uint32(eventData.MaxHealth)
	}

	if hasPower {
		power := eventData.Power
		if power > eventData.MaxPower {
			power = eventData.MaxPower
		}

		resp.Uint8(eventData.PowerType)
		resp.Uint16(clampGroupPacketPower(power))
		resp.Uint16(clampGroupPacketPower(eventData.MaxPower))
	}

	if updateMask&groupUpdateFlagLevel != 0 {
		resp.Uint16(uint16(eventData.Level))
	}

	if updateMask&groupUpdateFlagZone != 0 {
		resp.Uint16(uint16(eventData.ZoneID))
	}

	if updateMask&groupUpdateFlagAuras != 0 {
		s.writeGroupMemberAuraDelta(resp, eventData.RealmID, eventData.MemberGUID, renderAuras)
	} else if !eventData.Online {
		s.clearRenderedGroupMemberAuras(eventData.RealmID, eventData.MemberGUID)
	}

	s.gameSocket.Send(resp)
	return nil
}

func formatGroupMemberAuraTrace(auras []events.GroupMemberAuraState) string {
	auras = compactGroupMemberAuras(auras)
	if len(auras) == 0 {
		return ""
	}

	parts := make([]string, 0, len(auras))
	for _, aura := range auras {
		parts = append(parts, strconv.Itoa(int(aura.Slot))+":"+strconv.FormatUint(uint64(aura.SpellID), 10)+":"+strconv.Itoa(int(aura.Flags)))
	}

	return strings.Join(parts, ",")
}

func groupMemberStatusFromState(eventData *events.GroupEventMemberStateChangedPayload, auras []events.GroupMemberAuraState) uint16 {
	if !eventData.Online {
		return 0
	}

	status := memberStatusOnline
	if eventData.Ghost || hasGhostAura(auras) {
		status |= memberStatusGhost
	} else if eventData.Dead || (!eventData.DeadKnown && eventData.MaxHealth > 0 && eventData.Health == 0) {
		status |= memberStatusDead
	}

	return status
}

func hasGhostAura(auras []events.GroupMemberAuraState) bool {
	for _, aura := range auras {
		switch aura.SpellID {
		case ghostAuraSpellID, wispSpiritAuraSpellID:
			return true
		}
	}

	return false
}

func groupMemberStateChangedPayloadFromBatch(batch *events.GroupEventMemberStatesChangedPayload, state events.GroupMemberStateUpdate) events.GroupEventMemberStateChangedPayload {
	return events.GroupEventMemberStateChangedPayload{
		ServiceID:           batch.ServiceID,
		RealmID:             batch.RealmID,
		GroupID:             batch.GroupID,
		SourceGatewayID:     batch.SourceGatewayID,
		SourceWorldserverID: batch.SourceWorldserverID,
		MemberGUID:          state.MemberGUID,
		Online:              state.Online,
		Level:               state.Level,
		Class:               state.Class,
		ZoneID:              state.ZoneID,
		MapID:               state.MapID,
		Health:              state.Health,
		MaxHealth:           state.MaxHealth,
		PowerType:           state.PowerType,
		Power:               state.Power,
		MaxPower:            state.MaxPower,
		AurasKnown:          state.AurasKnown,
		Auras:               state.Auras,
		DeadKnown:           state.DeadKnown,
		Dead:                state.Dead,
		GhostKnown:          state.GhostKnown,
		Ghost:               state.Ghost,
		Receivers:           batch.Receivers,
	}
}

func (s *GameSession) shouldDropSourceWorldMemberStateEcho(eventData *events.GroupEventMemberStateChangedPayload) bool {
	if eventData.SourceWorldserverID == "" {
		return false
	}

	if !s.isCurrentWorldserverSourceID(eventData.SourceWorldserverID) {
		return false
	}

	return s.character != nil && samePlayerGUID(eventData.RealmID, eventData.MemberGUID, sessionRealmID(eventData.RealmID), s.character.GUID)
}

func (s *GameSession) writeGroupMemberAuraDelta(resp *packet.Writer, realmID uint32, memberGUID uint64, auras []events.GroupMemberAuraState) {
	current := groupMemberAuraStateBySlot(auras)
	previous := s.renderedGroupMemberAurasFor(realmID, memberGUID)

	auraBySlot := make(map[uint8]events.GroupMemberAuraState, len(current))
	var auraMask uint64
	for slot, aura := range current {
		auraBySlot[slot] = aura
		auraMask |= uint64(1) << slot
	}
	for slot := range previous {
		if _, found := current[slot]; !found {
			auraMask |= uint64(1) << slot
		}
	}

	resp.Uint64(auraMask)
	for slot := uint8(0); slot < maxGroupAuraSlots; slot++ {
		if auraMask&(uint64(1)<<slot) == 0 {
			continue
		}

		aura := auraBySlot[slot]
		resp.Uint32(aura.SpellID)
		resp.Uint8(aura.Flags)
	}

	s.setRenderedGroupMemberAuras(realmID, memberGUID, current)
}

func (s *GameSession) renderedGroupMemberAurasFor(realmID uint32, memberGUID uint64) map[uint8]events.GroupMemberAuraState {
	if s.renderedGroupMemberAuras == nil {
		return nil
	}

	return s.renderedGroupMemberAuras[groupMemberAuraRenderKey{realmID: realmID, memberGUID: memberGUID}]
}

func (s *GameSession) setRenderedGroupMemberAuras(realmID uint32, memberGUID uint64, auras map[uint8]events.GroupMemberAuraState) {
	key := groupMemberAuraRenderKey{realmID: realmID, memberGUID: memberGUID}
	if len(auras) == 0 {
		if s.renderedGroupMemberAuras != nil {
			delete(s.renderedGroupMemberAuras, key)
		}
		return
	}

	if s.renderedGroupMemberAuras == nil {
		s.renderedGroupMemberAuras = make(map[groupMemberAuraRenderKey]map[uint8]events.GroupMemberAuraState)
	}

	s.renderedGroupMemberAuras[key] = auras
}

func (s *GameSession) clearRenderedGroupMemberAuras(realmID uint32, memberGUID uint64) {
	if s.renderedGroupMemberAuras == nil {
		return
	}

	delete(s.renderedGroupMemberAuras, groupMemberAuraRenderKey{realmID: realmID, memberGUID: memberGUID})
}

func groupMemberAuraStateBySlot(auras []events.GroupMemberAuraState) map[uint8]events.GroupMemberAuraState {
	auras = compactGroupMemberAuras(auras)
	if len(auras) == 0 {
		return nil
	}

	bySlot := make(map[uint8]events.GroupMemberAuraState, len(auras))
	for _, aura := range auras {
		bySlot[aura.Slot] = aura
	}

	return bySlot
}

func compactGroupMemberAuras(auras []events.GroupMemberAuraState) []events.GroupMemberAuraState {
	if len(auras) == 0 {
		return nil
	}

	byOriginalSlot := make(map[uint8]events.GroupMemberAuraState, len(auras))
	for _, aura := range auras {
		if aura.Slot >= maxGroupAuraSlots || aura.SpellID == 0 {
			continue
		}
		byOriginalSlot[aura.Slot] = aura
	}
	if len(byOriginalSlot) == 0 {
		return nil
	}

	normalized := make([]events.GroupMemberAuraState, 0, len(byOriginalSlot))
	for _, aura := range byOriginalSlot {
		normalized = append(normalized, aura)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].Slot < normalized[j].Slot
	})

	return normalized
}

func (s *GameSession) setGroupMemberFlag(ctx context.Context, memberGUID uint64, flag uint8, apply bool) error {
	memberGUID = playerDBGUIDFromClientGUID(memberGUID)

	groupResp, err := s.groupServiceClient.GetGroupByMember(ctx, &pb.GetGroupByMemberRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: root.RealmID,
		Player:  s.character.GUID,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	if groupResp.Group == nil {
		return nil
	}

	groupRealmID := groupHomeRealmIDFromPB(groupResp.Group)
	var current *pb.GetGroupResponse_GroupMember
	for _, member := range groupResp.Group.Members {
		if samePlayerGUID(groupRealmID, member.Guid, root.RealmID, memberGUID) {
			current = member
			break
		}
	}

	if current == nil {
		return nil
	}

	flags := uint8(current.Flags)
	if apply {
		flags |= flag
	} else {
		flags &^= flag
	}

	_, err = s.groupServiceClient.SetMemberFlags(ctx, &pb.SetMemberFlagsRequest{
		Api:         root.SupportedGroupServiceVer,
		RealmID:     root.RealmID,
		UpdaterGUID: s.character.GUID,
		MemberGUID:  current.Guid,
		Flags:       uint32(flags),
		Roles:       current.Roles,
	})
	if err != nil {
		return NewGroupServiceUnavailableErr(err)
	}

	return nil
}

func (s *GameSession) isCurrentCharacterGroupLeaderOrAssistant(ctx context.Context, realmID uint32, groupID uint) (bool, error) {
	groupResp, err := s.groupServiceClient.GetGroup(ctx, &pb.GetGroupRequest{
		Api:     root.SupportedGroupServiceVer,
		RealmID: realmID,
		GroupID: uint32(groupID),
	})
	if err != nil {
		return false, NewGroupServiceUnavailableErr(err)
	}

	if groupResp.Group == nil {
		return false, nil
	}

	groupRealmID := groupHomeRealmIDFromPB(groupResp.Group)
	if samePlayerGUID(groupRealmID, groupResp.Group.Leader, root.RealmID, s.character.GUID) {
		return true, nil
	}

	for _, member := range groupResp.Group.Members {
		if samePlayerGUID(groupRealmID, member.Guid, root.RealmID, s.character.GUID) {
			return uint8(member.Flags)&groupMemberFlagAssistant != 0, nil
		}
	}

	return false, nil
}

func (s *GameSession) groupMemberByName(ctx context.Context, group *pb.GetGroupResponse_Group, memberName string) *pb.GetGroupResponse_GroupMember {
	if group == nil || memberName == "" {
		return nil
	}

	groupRealmID := groupHomeRealmIDFromPB(group)
	requestedName, requestedRealm, hasRequestedRealm := strings.Cut(memberName, "-")
	var fallback *pb.GetGroupResponse_GroupMember
	var ambiguousFallback bool

	for _, member := range group.Members {
		if strings.EqualFold(member.Name, memberName) {
			return member
		}

		if !hasRequestedRealm || !strings.EqualFold(member.Name, requestedName) {
			continue
		}

		memberRealmID := groupMemberRealmID(groupRealmID, member)
		if s.realmNamesService != nil {
			if realmName, err := s.realmNamesService.NameByID(ctx, memberRealmID); err == nil &&
				normalizedRealmName(realmName) == normalizedRealmName(requestedRealm) {
				return member
			}
		}

		if fallback == nil && !ambiguousFallback {
			fallback = member
			continue
		}

		ambiguousFallback = true
		fallback = nil
	}

	return fallback
}

func normalizedRealmName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "'", "")
	return name
}

func playerObjectGUIDForRealm(realmID uint32, playerGUID uint64) uint64 {
	// Gateway packet rendering uses client-facing player ObjectGuid values:
	// local members stay low DB GUIDs, foreign members keep realm-scoped
	// ObjectGuids. This is for rendering new packets only; forwarded client
	// packets must keep their original serialized GUIDs.
	playerRealmID := wowguid.PlayerRealmIDOrDefault(realmID, playerGUID)
	return wowguid.PlayerGUIDForRealm(root.RealmID, playerRealmID, playerGUID)
}

func playerObjectGUIDForMember(groupRealmID uint32, member *pb.GetGroupResponse_GroupMember) uint64 {
	if member == nil {
		return 0
	}

	return playerObjectGUIDForRealm(groupMemberRealmID(groupRealmID, member), member.Guid)
}

func groupMemberRealmID(groupRealmID uint32, member *pb.GetGroupResponse_GroupMember) uint32 {
	if member == nil {
		return groupRealmID
	}
	if member.RealmID != 0 {
		return member.RealmID
	}

	return playerRealmIDOrDefault(groupRealmID, member.Guid)
}

func groupHomeRealmIDFromPB(group *pb.GetGroupResponse_Group) uint32 {
	if group != nil && group.RealmID != 0 {
		return group.RealmID
	}

	return root.RealmID
}

func sessionRealmID(fallbackRealmID uint32) uint32 {
	if root.RealmID != 0 {
		return root.RealmID
	}

	return fallbackRealmID
}

func samePlayerGUID(leftDefaultRealmID uint32, leftGUID uint64, rightDefaultRealmID uint32, rightGUID uint64) bool {
	return wowguid.SamePlayer(leftDefaultRealmID, leftGUID, rightDefaultRealmID, rightGUID)
}

func playerRealmIDOrDefault(defaultRealmID uint32, playerGUID uint64) uint32 {
	return wowguid.PlayerRealmIDOrDefault(defaultRealmID, playerGUID)
}

func playerLowGUIDValue(playerGUID uint64) uint64 {
	return wowguid.PlayerLowGUID(playerGUID)
}

func playerDBGUIDFromClientGUID(playerGUID uint64) uint64 {
	if playerGUID == 0 {
		return 0
	}

	if playerGUID>>48 != 0 {
		return playerGUID
	}

	guidRealmID := uint32((playerGUID >> 32) & 0xffff)
	if guidRealmID != 0 && guidRealmID == root.RealmID {
		return playerGUID & 0xffffffff
	}

	return playerGUID
}

func groupObjectGUID(groupID uint64) uint64 {
	if groupID == 0 {
		return 0
	}

	return uint64(0x1F50)<<48 | groupID
}

func clampGroupPacketPower(power uint32) uint16 {
	if power > uint32(^uint16(0)) {
		return ^uint16(0)
	}

	return uint16(power)
}

func readGUIDThenBoolCompatible(r *packet.Reader) uint64 {
	if r.Left() == 9 {
		return r.Uint64()
	}

	return r.ReadGUID()
}

func readLastBoolCompatible(r *packet.Reader) bool {
	if r.Left() == 0 {
		return false
	}

	return r.Uint8() != 0
}

func readRemainingGUIDCompatible(r *packet.Reader) uint64 {
	if r.Left() == 8 {
		return r.Uint64()
	}

	return r.ReadGUID()
}
