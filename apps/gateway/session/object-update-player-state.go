package session

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/service"
	"github.com/walkline/ToCloud9/shared/groupstatetrace"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

const (
	objectUpdateTypeValues            = 0
	objectUpdateTypeMovement          = 1
	objectUpdateTypeCreateObject      = 2
	objectUpdateTypeCreateObject2     = 3
	objectUpdateTypeOutOfRangeObjects = 4

	objectUpdateFlagSelf               = 0x0001
	objectUpdateFlagTransport          = 0x0002
	objectUpdateFlagHasTarget          = 0x0004
	objectUpdateFlagUnknown            = 0x0008
	objectUpdateFlagLowGUID            = 0x0010
	objectUpdateFlagLiving             = 0x0020
	objectUpdateFlagStationaryPosition = 0x0040
	objectUpdateFlagVehicle            = 0x0080
	objectUpdateFlagPosition           = 0x0100
	objectUpdateFlagRotation           = 0x0200

	movementFlagOnTransport     = 0x00000200
	movementFlagFalling         = 0x00001000
	movementFlagSwimming        = 0x00200000
	movementFlagFlying          = 0x02000000
	movementFlagSplineElevation = 0x04000000
	movementFlagSplineEnabled   = 0x08000000

	movementFlag2AlwaysAllowPitching        = 0x00000020
	movementFlag2InterpolatedMovement       = 0x00000400
	objectTypeIDPlayer                uint8 = 4

	moveSplineFlagFinalPoint  = 0x00008000
	moveSplineFlagFinalTarget = 0x00010000
	moveSplineFlagFinalAngle  = 0x00020000

	unitFieldBytes0    = 0x0017
	unitFieldHealth    = 0x0018
	unitFieldPower1    = 0x0019
	unitFieldMaxHealth = 0x0020
	unitFieldMaxPower1 = 0x0021
	unitFieldLevel     = 0x0036
	unitPowerFieldMax  = 7
)

func (s *GameSession) InterceptObjectUpdate(_ context.Context, p *packet.Packet) error {
	s.gameSocket.SendPacket(p)

	if p.Source != packet.SourceWorldServer || s.character == nil || s.playerStateUpdatesBarrier == nil {
		return nil
	}

	if s.pendingRedirectID != "" {
		return nil
	}
	if !s.playerWorldActive {
		return nil
	}

	payload := p.Data
	var err error
	if p.Opcode == packet.SMsgCompressedUpdateObject {
		payload, err = decompressObjectUpdatePayload(p.Data)
		if err != nil {
			s.logger.Debug().Err(err).Msg("can't decompress object update packet for player state extraction")
			return nil
		}
	}

	snapshots, err := extractPlayerStateSnapshotsFromObjectUpdate(payload)
	if err != nil {
		s.logger.Debug().Err(err).Msg("can't fully parse object update packet for player state extraction")
	}

	currentMemberGUID := currentCharacterMemberGUID(s.character.GUID)

	for _, snapshot := range snapshots {
		if snapshot.MemberGUID != currentMemberGUID {
			continue
		}

		s.fillPlayerStateSnapshotSessionFields(&snapshot)
		if event := groupstatetrace.Event(s.logger, "gateway.object_update.snapshot", snapshot.MemberGUID); event != nil {
			traceSessionPlayerStateSnapshot(event, snapshot).
				Uint32("accountID", s.accountID).
				Str("opcode", p.Opcode.String()).
				Msg(groupstatetrace.Message)
		}
		s.playerStateUpdatesBarrier.Update(snapshot)
	}

	return nil
}

func (s *GameSession) fillPlayerStateSnapshotSessionFields(snapshot *service.PlayerStateSnapshot) {
	online := true
	level := s.character.Level
	classID := s.character.Class
	zoneID := s.character.Zone
	mapID := s.character.Map

	snapshot.SourceWorldserverID = s.currentWorldserverSourceID()
	if snapshot.Online == nil {
		snapshot.Online = &online
	}
	if snapshot.Level == nil {
		snapshot.Level = &level
	}
	if snapshot.Class == nil {
		snapshot.Class = &classID
	}
	if snapshot.ZoneID == nil {
		snapshot.ZoneID = &zoneID
	}
	if snapshot.MapID == nil {
		snapshot.MapID = &mapID
	}
	if snapshot.TimestampMs == 0 {
		snapshot.TimestampMs = uint64(time.Now().UnixMilli())
	}
}

func decompressObjectUpdatePayload(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("compressed object update too short: %d", len(data))
	}

	expectedSize := binary.LittleEndian.Uint32(data[:4])
	zr, err := zlib.NewReader(bytes.NewReader(data[4:]))
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	payload, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}

	if expectedSize != 0 && uint32(len(payload)) != expectedSize {
		return payload, fmt.Errorf("decompressed object update size mismatch: got %d, expected %d", len(payload), expectedSize)
	}

	return payload, nil
}

func extractPlayerStateSnapshotsFromObjectUpdate(data []byte) ([]service.PlayerStateSnapshot, error) {
	r := objectUpdateReader{data: data}
	blockCount, err := r.u32()
	if err != nil {
		return nil, err
	}

	snapshots := make([]service.PlayerStateSnapshot, 0)
	for i := uint32(0); i < blockCount; i++ {
		updateType, err := r.u8()
		if err != nil {
			return snapshots, err
		}

		switch updateType {
		case objectUpdateTypeValues:
			rawGUID, err := r.packedGUID()
			if err != nil {
				return snapshots, err
			}
			snapshot, err := r.valuesUpdate(rawGUID, true)
			if err != nil {
				return snapshots, err
			}
			if snapshot.MemberGUID != 0 {
				snapshots = append(snapshots, snapshot)
			}
		case objectUpdateTypeMovement:
			if _, err := r.packedGUID(); err != nil {
				return snapshots, err
			}
			if err := r.skipMovementUpdate(); err != nil {
				return snapshots, err
			}
		case objectUpdateTypeCreateObject, objectUpdateTypeCreateObject2:
			rawGUID, err := r.packedGUID()
			if err != nil {
				return snapshots, err
			}
			objectTypeID, err := r.u8()
			if err != nil {
				return snapshots, err
			}
			if err := r.skipMovementUpdate(); err != nil {
				return snapshots, err
			}
			snapshot, err := r.valuesUpdate(rawGUID, objectTypeID == objectTypeIDPlayer)
			if err != nil {
				return snapshots, err
			}
			if snapshot.MemberGUID != 0 {
				snapshots = append(snapshots, snapshot)
			}
		case objectUpdateTypeOutOfRangeObjects:
			count, err := r.u32()
			if err != nil {
				return snapshots, err
			}
			for j := uint32(0); j < count; j++ {
				if _, err := r.packedGUID(); err != nil {
					return snapshots, err
				}
			}
		default:
			return snapshots, fmt.Errorf("unsupported object update type %d", updateType)
		}
	}

	return snapshots, nil
}

type objectUpdateReader struct {
	data []byte
	pos  int
}

func (r *objectUpdateReader) u8() (uint8, error) {
	if r.pos+1 > len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}
	v := r.data[r.pos]
	r.pos++
	return v, nil
}

func (r *objectUpdateReader) u16() (uint16, error) {
	if r.pos+2 > len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.LittleEndian.Uint16(r.data[r.pos:])
	r.pos += 2
	return v, nil
}

func (r *objectUpdateReader) u32() (uint32, error) {
	if r.pos+4 > len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.LittleEndian.Uint32(r.data[r.pos:])
	r.pos += 4
	return v, nil
}

func (r *objectUpdateReader) skip(n int) error {
	if n < 0 || r.pos+n > len(r.data) {
		return io.ErrUnexpectedEOF
	}
	r.pos += n
	return nil
}

func (r *objectUpdateReader) packedGUID() (uint64, error) {
	mask, err := r.u8()
	if err != nil {
		return 0, err
	}

	var raw uint64
	for i := 0; i < 8; i++ {
		if mask&(1<<i) == 0 {
			continue
		}

		b, err := r.u8()
		if err != nil {
			return 0, err
		}
		raw |= uint64(b) << (8 * i)
	}

	return raw, nil
}

func (r *objectUpdateReader) skipMovementUpdate() error {
	flags, err := r.u16()
	if err != nil {
		return err
	}

	var movementFlags uint32
	var movementFlags2 uint16
	if flags&objectUpdateFlagLiving != 0 {
		movementFlags, err = r.u32()
		if err != nil {
			return err
		}
		movementFlags2, err = r.u16()
		if err != nil {
			return err
		}

		if err := r.skip(20); err != nil { // time, position, orientation
			return err
		}

		if movementFlags&movementFlagOnTransport != 0 {
			if _, err := r.packedGUID(); err != nil {
				return err
			}
			if err := r.skip(21); err != nil { // transport position, time, seat
				return err
			}
			if movementFlags2&movementFlag2InterpolatedMovement != 0 {
				if err := r.skip(4); err != nil {
					return err
				}
			}
		}

		if movementFlags&(movementFlagSwimming|movementFlagFlying) != 0 || movementFlags2&movementFlag2AlwaysAllowPitching != 0 {
			if err := r.skip(4); err != nil {
				return err
			}
		}

		if err := r.skip(4); err != nil { // fall time
			return err
		}

		if movementFlags&movementFlagFalling != 0 {
			if err := r.skip(16); err != nil {
				return err
			}
		}

		if movementFlags&movementFlagSplineElevation != 0 {
			if err := r.skip(4); err != nil {
				return err
			}
		}

		if err := r.skip(36); err != nil { // unit speeds
			return err
		}

		if movementFlags&movementFlagSplineEnabled != 0 {
			if err := r.skipSplineMovementCreate(); err != nil {
				return err
			}
		}
	} else if flags&objectUpdateFlagPosition != 0 {
		if _, err := r.packedGUID(); err != nil {
			return err
		}
		if err := r.skip(32); err != nil {
			return err
		}
	} else if flags&objectUpdateFlagStationaryPosition != 0 {
		if err := r.skip(16); err != nil {
			return err
		}
	}

	if flags&objectUpdateFlagUnknown != 0 {
		if err := r.skip(4); err != nil {
			return err
		}
	}
	if flags&objectUpdateFlagLowGUID != 0 {
		if err := r.skip(4); err != nil {
			return err
		}
	}
	if flags&objectUpdateFlagHasTarget != 0 {
		if _, err := r.packedGUID(); err != nil {
			return err
		}
	}
	if flags&objectUpdateFlagTransport != 0 {
		if err := r.skip(4); err != nil {
			return err
		}
	}
	if flags&objectUpdateFlagVehicle != 0 {
		if err := r.skip(8); err != nil {
			return err
		}
	}
	if flags&objectUpdateFlagRotation != 0 {
		if err := r.skip(8); err != nil {
			return err
		}
	}

	return nil
}

func (r *objectUpdateReader) skipSplineMovementCreate() error {
	splineFlags, err := r.u32()
	if err != nil {
		return err
	}

	switch {
	case splineFlags&moveSplineFlagFinalAngle != 0:
		if err := r.skip(4); err != nil {
			return err
		}
	case splineFlags&moveSplineFlagFinalTarget != 0:
		if err := r.skip(8); err != nil {
			return err
		}
	case splineFlags&moveSplineFlagFinalPoint != 0:
		if err := r.skip(12); err != nil {
			return err
		}
	}

	if err := r.skip(28); err != nil { // timePassed, duration, id, duration mods, vertical acceleration, effect start
		return err
	}

	nodes, err := r.u32()
	if err != nil {
		return err
	}

	if nodes > uint32((len(r.data)-r.pos)/12) {
		return io.ErrUnexpectedEOF
	}

	if err := r.skip(int(nodes) * 12); err != nil {
		return err
	}

	if err := r.skip(1); err != nil { // spline mode
		return err
	}

	return r.skip(12) // final destination
}

func (r *objectUpdateReader) valuesUpdate(rawGUID uint64, canExtract bool) (service.PlayerStateSnapshot, error) {
	maskBlockCount, err := r.u8()
	if err != nil {
		return service.PlayerStateSnapshot{}, err
	}

	updateMask := make([]uint32, maskBlockCount)
	for i := range updateMask {
		updateMask[i], err = r.u32()
		if err != nil {
			return service.PlayerStateSnapshot{}, err
		}
	}

	snapshot := service.PlayerStateSnapshot{}
	if canExtract {
		snapshot.MemberGUID = playerDBGUIDFromObjectUpdateGUID(rawGUID)
	}

	for blockIdx, block := range updateMask {
		for bit := 0; bit < 32; bit++ {
			if block&(1<<bit) == 0 {
				continue
			}

			value, err := r.u32()
			if err != nil {
				return service.PlayerStateSnapshot{}, err
			}

			if snapshot.MemberGUID == 0 {
				continue
			}

			field := uint32(blockIdx*32 + bit)
			applyPlayerStateField(&snapshot, field, value)
		}
	}

	if snapshot.MemberGUID == 0 {
		return service.PlayerStateSnapshot{}, nil
	}

	return snapshot, nil
}

func applyPlayerStateField(snapshot *service.PlayerStateSnapshot, field, value uint32) {
	switch {
	case field == unitFieldBytes0:
		classID := uint8((value >> 8) & 0xff)
		powerType := uint8((value >> 24) & 0xff)
		snapshot.Class = &classID
		snapshot.PowerType = &powerType
	case field == unitFieldHealth:
		snapshot.Health = &value
		dead := value == 0
		snapshot.Dead = &dead
	case field >= unitFieldPower1 && field < unitFieldPower1+unitPowerFieldMax:
		powerType := uint8(field - unitFieldPower1)
		if snapshot.PowerType != nil && *snapshot.PowerType != powerType {
			return
		}
		snapshot.PowerType = &powerType
		snapshot.Power = &value
	case field == unitFieldMaxHealth:
		snapshot.MaxHealth = &value
	case field >= unitFieldMaxPower1 && field < unitFieldMaxPower1+unitPowerFieldMax:
		powerType := uint8(field - unitFieldMaxPower1)
		if snapshot.PowerType != nil && *snapshot.PowerType != powerType {
			return
		}
		snapshot.PowerType = &powerType
		snapshot.MaxPower = &value
	case field == unitFieldLevel:
		level := uint8(value)
		snapshot.Level = &level
	}
}

func playerDBGUIDFromObjectUpdateGUID(raw uint64) uint64 {
	g := guid.New(raw)
	if raw == 0 || g.GetHigh() != guid.Player {
		return 0
	}

	if raw>>32 == 0 {
		return raw
	}

	return raw & 0xffffffff
}

func currentCharacterMemberGUID(characterGUID uint64) uint64 {
	currentMemberGUID := playerDBGUIDFromObjectUpdateGUID(characterGUID)
	if currentMemberGUID == 0 {
		return characterGUID
	}

	return currentMemberGUID
}
