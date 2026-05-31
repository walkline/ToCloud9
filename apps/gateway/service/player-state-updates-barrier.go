package service

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/gen/group/pb"
	"github.com/walkline/ToCloud9/shared/groupstatetrace"
	"github.com/walkline/ToCloud9/shared/wow"
)

const incompletePlayerStateRetention = 30 * time.Second

type PlayerStateSnapshot struct {
	MemberGUID          uint64
	SourceWorldserverID string
	Online              *bool
	Level               *uint8
	Class               *uint8
	ZoneID              *uint32
	MapID               *uint32
	InstanceID          *uint32
	Health              *uint32
	MaxHealth           *uint32
	PowerType           *uint8
	Power               *uint32
	MaxPower            *uint32
	AurasKnown          bool
	Auras               []PlayerAuraSnapshot
	TimestampMs         uint64
	Dead                *bool
	Ghost               *bool
}

type PlayerAuraSnapshot struct {
	Slot    uint8
	SpellID uint32
	Flags   uint8
}

type PlayerStateUpdatesBarrier struct {
	logger *zerolog.Logger

	groupServiceClient pb.GroupServiceClient
	api                string
	realmID            uint32
	sourceGatewayID    string
	updsChan           chan queuedPlayerStateSnapshot

	flushInterval time.Duration
}

type queuedPlayerStateSnapshot struct {
	snapshot PlayerStateSnapshot
	flush    bool
}

func NewPlayerStateUpdatesBarrier(
	logger *zerolog.Logger,
	groupServiceClient pb.GroupServiceClient,
	api string,
	realmID uint32,
	sourceGatewayID string,
	flushInterval time.Duration,
) *PlayerStateUpdatesBarrier {
	return &PlayerStateUpdatesBarrier{
		logger:             logger,
		groupServiceClient: groupServiceClient,
		api:                api,
		realmID:            realmID,
		sourceGatewayID:    sourceGatewayID,
		updsChan:           make(chan queuedPlayerStateSnapshot, 1000),
		flushInterval:      flushInterval,
	}
}

func (b *PlayerStateUpdatesBarrier) Update(snapshot PlayerStateSnapshot) {
	b.update(snapshot, false)
}

func (b *PlayerStateUpdatesBarrier) UpdateAndFlush(snapshot PlayerStateSnapshot) {
	b.update(snapshot, true)
}

func (b *PlayerStateUpdatesBarrier) update(snapshot PlayerStateSnapshot, flush bool) {
	if snapshot.MemberGUID == 0 {
		return
	}

	select {
	case b.updsChan <- queuedPlayerStateSnapshot{snapshot: snapshot, flush: flush}:
	default:
		b.logger.Warn().
			Uint64("memberGUID", snapshot.MemberGUID).
			Str("sourceWorldserverID", snapshot.SourceWorldserverID).
			Msg("dropping player state snapshot because barrier queue is full")
	}
}

func (b *PlayerStateUpdatesBarrier) Run(ctx context.Context) {
	t := time.NewTicker(b.flushInterval)
	defer t.Stop()

	buffer := map[string]map[uint64]PlayerStateSnapshot{}
	lastSent := map[uint64]PlayerStateSnapshot{}

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := b.send(ctx, buffer, lastSent); err != nil {
				b.logger.Error().Err(err).Msg("can't send player state updates")
				continue
			}
		case queued := <-b.updsChan:
			snapshot := queued.snapshot
			bufferPlayerStateSnapshot(buffer, lastSent, snapshot)
			if (queued.flush || snapshot.AurasKnown) && hasSendableBufferedSnapshot(buffer, snapshot.MemberGUID) {
				if err := b.send(ctx, buffer, lastSent); err != nil {
					b.logger.Error().Err(err).Msg("can't send player state updates")
					continue
				}
			}
		}
	}
}

func hasSendableBufferedSnapshot(buffer map[string]map[uint64]PlayerStateSnapshot, memberGUID uint64) bool {
	for _, updatesByMember := range buffer {
		if snapshot, ok := updatesByMember[memberGUID]; ok {
			return snapshot.IsSendable()
		}
	}

	return false
}

func bufferPlayerStateSnapshot(buffer map[string]map[uint64]PlayerStateSnapshot, lastSent map[uint64]PlayerStateSnapshot, snapshot PlayerStateSnapshot) {
	sourceWorldserverID := snapshot.SourceWorldserverID
	if sourceWorldserverID == "" {
		sourceWorldserverID = lastSent[snapshot.MemberGUID].SourceWorldserverID
		snapshot.SourceWorldserverID = sourceWorldserverID
	}

	base := lastSent[snapshot.MemberGUID]
	for bufferedSource, updatesByMember := range buffer {
		if oldSnapshot, ok := updatesByMember[snapshot.MemberGUID]; ok {
			base = oldSnapshot
			if bufferedSource != sourceWorldserverID {
				delete(updatesByMember, snapshot.MemberGUID)
				if len(updatesByMember) == 0 {
					delete(buffer, bufferedSource)
				}
			}
		}
	}

	if shouldDropInactiveDirectPowerSnapshot(base, snapshot) {
		return
	}
	snapshot = normalizeFixedClassPlayerPower(snapshot)

	if buffer[sourceWorldserverID] == nil {
		buffer[sourceWorldserverID] = map[uint64]PlayerStateSnapshot{}
	}

	buffer[sourceWorldserverID][snapshot.MemberGUID] = normalizeFixedClassPlayerPower(mergePlayerStateSnapshots(base, snapshot))
}

func shouldDropInactiveDirectPowerSnapshot(base, update PlayerStateSnapshot) bool {
	if base.PowerType == nil || update.PowerType == nil || update.Power == nil || *base.PowerType == *update.PowerType {
		return false
	}

	return update.InstanceID == nil &&
		update.Health == nil &&
		update.MaxHealth == nil &&
		update.MaxPower == nil &&
		!update.AurasKnown
}

func (b *PlayerStateUpdatesBarrier) send(ctx context.Context, buffer map[string]map[uint64]PlayerStateSnapshot, lastSent map[uint64]PlayerStateSnapshot) error {
	nowMs := uint64(time.Now().UnixMilli())
	for sourceWorldserverID, updatesByMember := range buffer {
		snapshots := make([]*pb.PlayerStateSnapshot, 0, len(updatesByMember))
		for memberGUID, snapshot := range updatesByMember {
			if !snapshot.IsSendable() || isInitialAuraOnlySnapshot(snapshot, lastSent) {
				if event := groupstatetrace.Event(b.logger, "gateway.barrier.skip", memberGUID); event != nil {
					tracePlayerStateSnapshot(event, snapshot).
						Str("sourceWorldserverID", sourceWorldserverID).
						Bool("sendable", snapshot.IsSendable()).
						Bool("initialAuraOnly", isInitialAuraOnlySnapshot(snapshot, lastSent)).
						Bool("hasLastSent", lastSent[memberGUID].MemberGUID != 0).
						Msg(groupstatetrace.Message)
				}
				if snapshot.AurasKnown && b.logger != nil {
					b.logger.Debug().
						Uint64("memberGUID", memberGUID).
						Str("sourceWorldserverID", sourceWorldserverID).
						Bool("hasOnline", snapshot.Online != nil).
						Bool("hasLevel", snapshot.Level != nil).
						Bool("hasClass", snapshot.Class != nil).
						Bool("hasZone", snapshot.ZoneID != nil).
						Bool("hasMap", snapshot.MapID != nil).
						Bool("hasHealth", snapshot.Health != nil).
						Bool("hasMaxHealth", snapshot.MaxHealth != nil).
						Bool("hasPowerType", snapshot.PowerType != nil).
						Bool("hasPower", snapshot.Power != nil).
						Bool("hasMaxPower", snapshot.MaxPower != nil).
						Bool("hasLastSent", lastSent[memberGUID].MemberGUID != 0).
						Msg("TC9 skipping player aura state: player state snapshot has no complete baseline")
				}
				continue
			}

			if oldSnapshot, ok := lastSent[memberGUID]; ok && snapshot.Equal(oldSnapshot) {
				continue
			}

			snapshot = ensureMonotonicPlayerStateTimestamp(snapshot, lastSent)
			transmitSnapshot := snapshotForPlayerStateTransmission(snapshot, lastSent[memberGUID])
			updatesByMember[memberGUID] = snapshot
			if event := groupstatetrace.Event(b.logger, "gateway.barrier.send", memberGUID); event != nil {
				tracePlayerStateSnapshot(event, transmitSnapshot).
					Str("sourceWorldserverID", sourceWorldserverID).
					Msg(groupstatetrace.Message)
			}
			snapshots = append(snapshots, transmitSnapshot.ToProto())
		}

		if len(snapshots) == 0 {
			pruneCompleteBufferedSnapshots(buffer, lastSent, sourceWorldserverID)
			continue
		}

		if b.logger != nil {
			b.logger.Debug().
				Str("sourceWorldserverID", sourceWorldserverID).
				Int("snapshotCount", len(snapshots)).
				Msg("TC9 sending player state batch")
		}

		if _, err := b.groupServiceClient.BulkUpdateMemberStates(ctx, &pb.BulkUpdateMemberStatesRequest{
			Api:                 b.api,
			RealmID:             b.realmID,
			SourceGatewayID:     b.sourceGatewayID,
			SourceWorldserverID: sourceWorldserverID,
			Snapshots:           snapshots,
		}); err != nil {
			return err
		}

		pruneCompleteBufferedSnapshots(buffer, lastSent, sourceWorldserverID)
	}

	pruneStaleIncompleteBufferedSnapshots(buffer, nowMs)
	return nil
}

func tracePlayerStateSnapshot(event *zerolog.Event, snapshot PlayerStateSnapshot) *zerolog.Event {
	event = event.
		Uint64("memberGUID", snapshot.MemberGUID).
		Str("snapshotSourceWorldserverID", snapshot.SourceWorldserverID).
		Bool("hasOnline", snapshot.Online != nil).
		Bool("hasLevel", snapshot.Level != nil).
		Bool("hasClass", snapshot.Class != nil).
		Bool("hasZone", snapshot.ZoneID != nil).
		Bool("hasMap", snapshot.MapID != nil).
		Bool("hasInstance", snapshot.InstanceID != nil).
		Bool("hasHealth", snapshot.Health != nil).
		Bool("hasMaxHealth", snapshot.MaxHealth != nil).
		Bool("hasPowerType", snapshot.PowerType != nil).
		Bool("hasPower", snapshot.Power != nil).
		Bool("hasMaxPower", snapshot.MaxPower != nil).
		Bool("hasDead", snapshot.Dead != nil).
		Bool("hasGhost", snapshot.Ghost != nil).
		Bool("aurasKnown", snapshot.AurasKnown).
		Int("auraCount", len(snapshot.Auras)).
		Uint64("timestampMs", snapshot.TimestampMs)
	if auraSpells := FormatPlayerAuraTrace(snapshot.Auras); auraSpells != "" {
		event = event.Str("auraSpells", auraSpells)
	}

	if snapshot.Online != nil {
		event = event.Bool("online", *snapshot.Online)
	}
	if snapshot.Level != nil {
		event = event.Uint8("level", *snapshot.Level)
	}
	if snapshot.Class != nil {
		event = event.Uint8("class", *snapshot.Class)
	}
	if snapshot.ZoneID != nil {
		event = event.Uint32("zoneID", *snapshot.ZoneID)
	}
	if snapshot.MapID != nil {
		event = event.Uint32("mapID", *snapshot.MapID)
	}
	if snapshot.InstanceID != nil {
		event = event.Uint32("instanceID", *snapshot.InstanceID)
	}
	if snapshot.Health != nil {
		event = event.Uint32("health", *snapshot.Health)
	}
	if snapshot.MaxHealth != nil {
		event = event.Uint32("maxHealth", *snapshot.MaxHealth)
	}
	if snapshot.PowerType != nil {
		event = event.Uint8("powerType", *snapshot.PowerType)
	}
	if snapshot.Power != nil {
		event = event.Uint32("power", *snapshot.Power)
	}
	if snapshot.MaxPower != nil {
		event = event.Uint32("maxPower", *snapshot.MaxPower)
	}
	if snapshot.Dead != nil {
		event = event.Bool("dead", *snapshot.Dead)
	}
	if snapshot.Ghost != nil {
		event = event.Bool("ghost", *snapshot.Ghost)
	}

	return event
}

func normalizeFixedClassPlayerPower(snapshot PlayerStateSnapshot) PlayerStateSnapshot {
	if snapshot.Class == nil || snapshot.PowerType == nil {
		return snapshot
	}
	if !wow.IsFixedClassInactivePowerType(*snapshot.Class, *snapshot.PowerType) {
		return snapshot
	}
	if snapshot.Power == nil && snapshot.MaxPower == nil {
		return snapshot
	}

	powerType, _ := wow.FixedPrimaryPowerTypeForClass(*snapshot.Class)
	snapshot.PowerType = &powerType
	if snapshot.Power != nil {
		power := uint32(0)
		snapshot.Power = &power
	}
	if snapshot.MaxPower == nil || *snapshot.MaxPower == 0 {
		if maxPower := wow.DefaultMaxPowerForClass(*snapshot.Class); maxPower != 0 {
			snapshot.MaxPower = &maxPower
		}
	}

	return snapshot
}

func FormatPlayerAuraTrace(auras []PlayerAuraSnapshot) string {
	auras = normalizePlayerAuras(auras)
	if len(auras) == 0 {
		return ""
	}

	parts := make([]string, 0, len(auras))
	for _, aura := range auras {
		parts = append(parts, strconv.Itoa(int(aura.Slot))+":"+strconv.FormatUint(uint64(aura.SpellID), 10)+":"+strconv.Itoa(int(aura.Flags)))
	}

	return strings.Join(parts, ",")
}

func isInitialAuraOnlySnapshot(snapshot PlayerStateSnapshot, lastSent map[uint64]PlayerStateSnapshot) bool {
	if snapshot.IsComplete() || !snapshot.isAuraOnlySendable() {
		return false
	}
	if snapshot.hasVitalEvidence() {
		return false
	}

	return lastSent[snapshot.MemberGUID].MemberGUID == 0
}

func ensureMonotonicPlayerStateTimestamp(snapshot PlayerStateSnapshot, lastSent map[uint64]PlayerStateSnapshot) PlayerStateSnapshot {
	if snapshot.TimestampMs == 0 {
		return snapshot
	}

	last, ok := lastSent[snapshot.MemberGUID]
	if !ok || last.TimestampMs == 0 || last.TimestampMs == ^uint64(0) || snapshot.TimestampMs > last.TimestampMs {
		return snapshot
	}

	snapshot.TimestampMs = last.TimestampMs + 1
	return snapshot
}

func pruneCompleteBufferedSnapshots(buffer map[string]map[uint64]PlayerStateSnapshot, lastSent map[uint64]PlayerStateSnapshot, sourceWorldserverID string) {
	updatesByMember := buffer[sourceWorldserverID]
	for memberGUID, snapshot := range updatesByMember {
		if !snapshot.IsSendable() || isInitialAuraOnlySnapshot(snapshot, lastSent) {
			continue
		}

		lastSent[memberGUID] = snapshot
		delete(updatesByMember, memberGUID)
	}

	if len(updatesByMember) == 0 {
		delete(buffer, sourceWorldserverID)
	}
}

func pruneStaleIncompleteBufferedSnapshots(buffer map[string]map[uint64]PlayerStateSnapshot, nowMs uint64) {
	retentionMs := uint64(incompletePlayerStateRetention / time.Millisecond)
	for sourceWorldserverID, updatesByMember := range buffer {
		for memberGUID, snapshot := range updatesByMember {
			if snapshot.IsSendable() || snapshot.TimestampMs == 0 {
				continue
			}

			if snapshot.TimestampMs+retentionMs < nowMs {
				delete(updatesByMember, memberGUID)
			}
		}

		if len(updatesByMember) == 0 {
			delete(buffer, sourceWorldserverID)
		}
	}
}

func mergePlayerStateSnapshots(base, update PlayerStateSnapshot) PlayerStateSnapshot {
	sourceChanged := update.SourceWorldserverID != "" && base.SourceWorldserverID != "" && update.SourceWorldserverID != base.SourceWorldserverID
	mapChanged := update.MapID != nil && base.MapID != nil && *update.MapID != *base.MapID
	powerTypeChanged := update.PowerType != nil && base.PowerType != nil && *update.PowerType != *base.PowerType
	if sourceChanged || mapChanged {
		base.InstanceID = nil
	}
	if sourceChanged {
		base.Health = nil
		base.MaxHealth = nil
		base.PowerType = nil
		base.Power = nil
		base.MaxPower = nil
		base.Dead = nil
		base.Ghost = nil
	}
	if powerTypeChanged {
		base.Power = nil
		base.MaxPower = nil
	}

	if update.MemberGUID != 0 {
		base.MemberGUID = update.MemberGUID
	}
	if update.SourceWorldserverID != "" {
		base.SourceWorldserverID = update.SourceWorldserverID
	}
	if update.Online != nil {
		base.Online = update.Online
	}
	if update.Level != nil {
		base.Level = update.Level
	}
	if update.Class != nil {
		base.Class = update.Class
	}
	if update.ZoneID != nil {
		base.ZoneID = update.ZoneID
	}
	if update.MapID != nil {
		base.MapID = update.MapID
	}
	if update.InstanceID != nil {
		base.InstanceID = update.InstanceID
	}
	if update.Health != nil {
		base.Health = update.Health
	}
	if update.MaxHealth != nil {
		base.MaxHealth = update.MaxHealth
	}
	if update.PowerType != nil {
		base.PowerType = update.PowerType
	}
	if update.Power != nil {
		base.Power = update.Power
	}
	if update.MaxPower != nil {
		base.MaxPower = update.MaxPower
	}
	if update.Dead != nil {
		base.Dead = update.Dead
	}
	if update.Ghost != nil {
		base.Ghost = update.Ghost
	}
	if update.AurasKnown {
		base.AurasKnown = true
		base.Auras = normalizePlayerAuras(update.Auras)
	}
	if update.TimestampMs != 0 {
		base.TimestampMs = update.TimestampMs
	}

	return fillMissingDeadPlayerPower(base)
}

func fillMissingDeadPlayerPower(snapshot PlayerStateSnapshot) PlayerStateSnapshot {
	if snapshot.Power != nil ||
		snapshot.Health == nil ||
		*snapshot.Health > 1 ||
		snapshot.PowerType == nil ||
		snapshot.MaxPower == nil {
		return snapshot
	}

	zeroPower := uint32(0)
	snapshot.Power = &zeroPower
	return snapshot
}

func snapshotForPlayerStateTransmission(snapshot, lastSent PlayerStateSnapshot) PlayerStateSnapshot {
	if !shouldTransmitAuraOnlySnapshot(snapshot, lastSent) {
		return stripIncompletePowerForTransmission(snapshot)
	}

	snapshot.Health = nil
	snapshot.MaxHealth = nil
	snapshot.PowerType = nil
	snapshot.Power = nil
	snapshot.MaxPower = nil
	return snapshot
}

func stripIncompletePowerForTransmission(snapshot PlayerStateSnapshot) PlayerStateSnapshot {
	if snapshot.Power != nil {
		return snapshot
	}

	snapshot.PowerType = nil
	snapshot.MaxPower = nil
	return snapshot
}

func shouldTransmitAuraOnlySnapshot(snapshot, lastSent PlayerStateSnapshot) bool {
	if !snapshot.AurasKnown || !snapshot.IsComplete() || lastSent.MemberGUID == 0 {
		return false
	}

	return snapshot.MemberGUID == lastSent.MemberGUID &&
		boolValue(snapshot.Online) == boolValue(lastSent.Online) &&
		uint8Value(snapshot.Level) == uint8Value(lastSent.Level) &&
		uint8Value(snapshot.Class) == uint8Value(lastSent.Class) &&
		uint32Value(snapshot.ZoneID) == uint32Value(lastSent.ZoneID) &&
		uint32Value(snapshot.MapID) == uint32Value(lastSent.MapID) &&
		uint32PtrEqual(snapshot.InstanceID, lastSent.InstanceID) &&
		uint32Value(snapshot.Health) == uint32Value(lastSent.Health) &&
		uint32Value(snapshot.MaxHealth) == uint32Value(lastSent.MaxHealth) &&
		uint8Value(snapshot.PowerType) == uint8Value(lastSent.PowerType) &&
		uint32Value(snapshot.Power) == uint32Value(lastSent.Power) &&
		uint32Value(snapshot.MaxPower) == uint32Value(lastSent.MaxPower)
}

func (s PlayerStateSnapshot) IsComplete() bool {
	return s.MemberGUID != 0 &&
		s.Online != nil &&
		s.Level != nil &&
		s.Class != nil &&
		s.ZoneID != nil &&
		s.MapID != nil &&
		s.Health != nil &&
		s.MaxHealth != nil &&
		s.PowerType != nil &&
		s.Power != nil &&
		s.MaxPower != nil
}

func (s PlayerStateSnapshot) IsSendable() bool {
	return s.IsComplete() || s.isAuraOnlySendable()
}

func (s PlayerStateSnapshot) hasVitalEvidence() bool {
	return s.Health != nil ||
		s.MaxHealth != nil ||
		s.PowerType != nil ||
		s.Power != nil ||
		s.MaxPower != nil ||
		s.Dead != nil ||
		s.Ghost != nil
}

func (s PlayerStateSnapshot) isAuraOnlySendable() bool {
	return s.MemberGUID != 0 &&
		s.AurasKnown &&
		s.Online != nil &&
		s.Level != nil &&
		s.Class != nil &&
		s.ZoneID != nil &&
		s.MapID != nil
}

func (s PlayerStateSnapshot) Equal(other PlayerStateSnapshot) bool {
	if !s.IsSendable() || !other.IsSendable() {
		return false
	}

	return s.MemberGUID == other.MemberGUID &&
		s.SourceWorldserverID == other.SourceWorldserverID &&
		boolValue(s.Online) == boolValue(other.Online) &&
		uint8Value(s.Level) == uint8Value(other.Level) &&
		uint8Value(s.Class) == uint8Value(other.Class) &&
		uint32Value(s.ZoneID) == uint32Value(other.ZoneID) &&
		uint32Value(s.MapID) == uint32Value(other.MapID) &&
		uint32PtrEqual(s.InstanceID, other.InstanceID) &&
		uint32Value(s.Health) == uint32Value(other.Health) &&
		uint32Value(s.MaxHealth) == uint32Value(other.MaxHealth) &&
		uint8Value(s.PowerType) == uint8Value(other.PowerType) &&
		uint32Value(s.Power) == uint32Value(other.Power) &&
		uint32Value(s.MaxPower) == uint32Value(other.MaxPower) &&
		s.AurasKnown == other.AurasKnown &&
		playerAurasEqual(s.Auras, other.Auras) &&
		boolPtrEqual(s.Dead, other.Dead) &&
		boolPtrEqual(s.Ghost, other.Ghost)
}

func (s PlayerStateSnapshot) ToProto() *pb.PlayerStateSnapshot {
	s.Auras = normalizePlayerAuras(s.Auras)

	return &pb.PlayerStateSnapshot{
		MemberGUID:  s.MemberGUID,
		Online:      boolValue(s.Online),
		Level:       uint32(uint8Value(s.Level)),
		ClassID:     uint32(uint8Value(s.Class)),
		ZoneID:      uint32Value(s.ZoneID),
		MapID:       uint32Value(s.MapID),
		InstanceID:  s.InstanceID,
		Health:      uint32Value(s.Health),
		MaxHealth:   uint32Value(s.MaxHealth),
		PowerType:   uint32(uint8Value(s.PowerType)),
		Power:       uint32Value(s.Power),
		MaxPower:    uint32Value(s.MaxPower),
		TimestampMs: s.TimestampMs,
		AurasKnown:  s.AurasKnown,
		Auras:       playerAurasToProto(s.Auras),
		Dead:        s.Dead,
		Ghost:       s.Ghost,
	}
}

func boolPtrEqual(a, b *bool) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func boolValue(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func uint8Value(v *uint8) uint8 {
	if v == nil {
		return 0
	}
	return *v
}

func uint32Value(v *uint32) uint32 {
	if v == nil {
		return 0
	}
	return *v
}

func uint32PtrEqual(a, b *uint32) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

func normalizePlayerAuras(auras []PlayerAuraSnapshot) []PlayerAuraSnapshot {
	const maxGroupAuraSlots = 64

	if len(auras) == 0 {
		return nil
	}

	bySlot := make(map[uint8]PlayerAuraSnapshot, len(auras))
	for _, aura := range auras {
		if aura.Slot >= maxGroupAuraSlots || aura.SpellID == 0 {
			continue
		}
		bySlot[aura.Slot] = aura
	}

	out := make([]PlayerAuraSnapshot, 0, len(bySlot))
	for _, aura := range bySlot {
		out = append(out, aura)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Slot < out[j].Slot
	})

	return out
}

func playerAurasEqual(a, b []PlayerAuraSnapshot) bool {
	a = normalizePlayerAuras(a)
	b = normalizePlayerAuras(b)

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func playerAurasToProto(auras []PlayerAuraSnapshot) []*pb.PlayerAuraSnapshot {
	if len(auras) == 0 {
		return nil
	}

	out := make([]*pb.PlayerAuraSnapshot, 0, len(auras))
	for _, aura := range auras {
		out = append(out, &pb.PlayerAuraSnapshot{
			Slot:    uint32(aura.Slot),
			SpellID: aura.SpellID,
			Flags:   uint32(aura.Flags),
		})
	}

	return out
}
