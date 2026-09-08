package session

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
)

const (
	corpseSnapshotProtocolVersion = uint8(1)
	corpseSnapshotUpsert          = uint8(1)
	corpseSnapshotRemove          = uint8(2)
	maxCorpseSnapshotsPerSession  = 1024
	maxCorpseSnapshotRecords      = 4096
	maxCorpseSnapshotSize         = 64 * 1024
)

type corpseSnapshot struct {
	operation  uint8
	id         uint64
	revision   uint32
	mapID      uint32
	instanceID uint32
	carrier    uint64
	expiresAt  uint64
	payload    []byte
}

func (s *GameSession) DropInternalTC9Packet(_ context.Context, _ *packet.Packet) error {
	return nil
}

func decodeCorpseSnapshotEnvelope(p *packet.Packet) (uint8, corpseSnapshot, error) {
	if p == nil || len(p.Data) > maxCorpseSnapshotSize {
		return 0, corpseSnapshot{}, fmt.Errorf("invalid corpse snapshot size")
	}

	r := p.Reader()
	version := r.Uint8()
	operation := r.Uint8()
	snapshot := corpseSnapshot{
		operation:  operation,
		id:         r.Uint64(),
		revision:   r.Uint32(),
		mapID:      r.Uint32(),
		instanceID: r.Uint32(),
		carrier:    r.Uint64(),
		expiresAt:  r.Uint64(),
	}
	if err := r.Error(); err != nil {
		return 0, corpseSnapshot{}, fmt.Errorf("decode corpse snapshot envelope: %w", err)
	}
	if version != corpseSnapshotProtocolVersion {
		return 0, corpseSnapshot{}, fmt.Errorf("unsupported corpse snapshot version %d", version)
	}
	if operation != corpseSnapshotUpsert && operation != corpseSnapshotRemove {
		return 0, corpseSnapshot{}, fmt.Errorf("unsupported corpse snapshot operation %d", operation)
	}
	if snapshot.id == 0 || snapshot.carrier == 0 {
		return 0, corpseSnapshot{}, fmt.Errorf("corpse snapshot has an empty id or carrier")
	}

	snapshot.payload = append([]byte(nil), p.Data...)
	return operation, snapshot, nil
}

// InterceptCorpseSnapshot stores the opaque, versioned core payload. The
// gateway only owns transfer lifecycle; AzerothCore remains the authority for
// loot representation and validation.
func (s *GameSession) InterceptCorpseSnapshot(_ context.Context, p *packet.Packet) error {
	if p.Source != packet.SourceWorldServer || s.character == nil {
		return nil
	}

	operation, snapshot, err := decodeCorpseSnapshotEnvelope(p)
	if err != nil {
		return err
	}
	if snapshot.carrier != s.character.GUID {
		return nil
	}

	if s.corpseSnapshots == nil {
		s.corpseSnapshots = make(map[uint64]corpseSnapshot)
	}
	current, exists := s.corpseSnapshots[snapshot.id]
	if exists && current.revision >= snapshot.revision {
		return nil
	}
	if operation == corpseSnapshotRemove {
		s.makeRoomForCorpseSnapshotRecord()
		s.corpseSnapshots[snapshot.id] = snapshot
		return nil
	}
	if snapshot.expiresAt <= uint64(time.Now().Unix()) {
		delete(s.corpseSnapshots, snapshot.id)
		return nil
	}
	wasActive := exists && current.operation == corpseSnapshotUpsert
	if !wasActive && s.activeCorpseSnapshotCount() >= maxCorpseSnapshotsPerSession {
		s.pruneCorpseSnapshots(time.Now())
	}
	if !wasActive && s.activeCorpseSnapshotCount() >= maxCorpseSnapshotsPerSession {
		return fmt.Errorf("corpse snapshot limit reached")
	}
	s.makeRoomForCorpseSnapshotRecord()
	s.corpseSnapshots[snapshot.id] = snapshot
	return nil
}

func (s *GameSession) activeCorpseSnapshotCount() int {
	count := 0
	for _, snapshot := range s.corpseSnapshots {
		if snapshot.operation == corpseSnapshotUpsert {
			count++
		}
	}
	return count
}

func (s *GameSession) makeRoomForCorpseSnapshotRecord() {
	if len(s.corpseSnapshots) < maxCorpseSnapshotRecords {
		return
	}
	s.pruneCorpseSnapshots(time.Now())
	if len(s.corpseSnapshots) < maxCorpseSnapshotRecords {
		return
	}

	var oldestID uint64
	var oldestExpiry uint64
	for id, snapshot := range s.corpseSnapshots {
		if snapshot.operation != corpseSnapshotRemove {
			continue
		}
		if oldestID == 0 || snapshot.expiresAt < oldestExpiry {
			oldestID = id
			oldestExpiry = snapshot.expiresAt
		}
	}
	if oldestID != 0 {
		delete(s.corpseSnapshots, oldestID)
	}
}

func (s *GameSession) pruneCorpseSnapshots(now time.Time) {
	for id, snapshot := range s.corpseSnapshots {
		if snapshot.expiresAt <= uint64(now.Unix()) {
			delete(s.corpseSnapshots, id)
		}
	}
}

func (s *GameSession) clearCorpseSnapshots() {
	clear(s.corpseSnapshots)
}

func (s *GameSession) corpseSnapshotIDsForCurrentMap() []uint64 {
	if s.character == nil {
		return nil
	}
	s.pruneCorpseSnapshots(time.Now())
	ids := make([]uint64, 0, len(s.corpseSnapshots))
	for id, snapshot := range s.corpseSnapshots {
		if snapshot.operation == corpseSnapshotUpsert && snapshot.mapID == s.character.Map {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (s *GameSession) prepareForRedirectPacket(detachCurrentMapCorpses bool) *packet.Writer {
	w := packet.NewWriter(packet.TC9CMsgPrepareForRedirect)
	if !detachCurrentMapCorpses {
		return w
	}

	ids := s.corpseSnapshotIDsForCurrentMap()
	w.Uint8(corpseSnapshotProtocolVersion).Uint16(uint16(len(ids)))
	for _, id := range ids {
		w.Uint64(id)
	}
	return w
}

func (s *GameSession) restoreCorpseSnapshots() {
	if s.worldSocket == nil || s.character == nil {
		return
	}
	s.pruneCorpseSnapshots(time.Now())
	for _, snapshot := range s.corpseSnapshots {
		if snapshot.operation != corpseSnapshotUpsert || snapshot.mapID != s.character.Map {
			continue
		}
		w := packet.NewWriterWithSize(packet.TC9CMsgRestoreCorpse, uint32(len(snapshot.payload)))
		w.Bytes(snapshot.payload)
		s.worldSocket.Send(w)
	}
}
