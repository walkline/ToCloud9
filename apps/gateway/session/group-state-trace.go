package session

import (
	"github.com/rs/zerolog"

	"github.com/walkline/ToCloud9/apps/gateway/service"
)

func traceSessionPlayerStateSnapshot(event *zerolog.Event, snapshot service.PlayerStateSnapshot) *zerolog.Event {
	event = event.
		Uint64("memberGUID", snapshot.MemberGUID).
		Str("sourceWorldserverID", snapshot.SourceWorldserverID).
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
	if auraSpells := service.FormatPlayerAuraTrace(snapshot.Auras); auraSpells != "" {
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
