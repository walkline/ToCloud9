package packet

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
)

// Unit update field indices for client build 3.3.5a (see AzerothCore UpdateFields.h).
const (
	objectEnd = 0x6

	unitFieldBytes0    = objectEnd + 0x11
	unitFieldHealth    = objectEnd + 0x12
	unitFieldPower1    = objectEnd + 0x13
	unitFieldMaxHealth = objectEnd + 0x1A
	unitFieldMaxPower1 = objectEnd + 0x1B
	unitFieldLevel     = objectEnd + 0x30
	unitFieldFlags     = objectEnd + 0x35

	powersCount = 7
)

// Update types (see AzerothCore UpdateData.h).
const (
	updateTypeValues            = 0
	updateTypeMovement          = 1
	updateTypeCreateObject      = 2
	updateTypeCreateObject2     = 3
	updateTypeOutOfRangeObjects = 4
	updateTypeNearObjects       = 5
)

// Update flags (see AzerothCore UpdateData.h).
const (
	updateFlagTransport          = 0x0002
	updateFlagHasTarget          = 0x0004
	updateFlagUnknown            = 0x0008
	updateFlagLowGUID            = 0x0010
	updateFlagLiving             = 0x0020
	updateFlagStationaryPosition = 0x0040
	updateFlagVehicle            = 0x0080
	updateFlagPosition           = 0x0100
	updateFlagRotation           = 0x0200
)

// Movement flags (see AzerothCore UnitDefines.h).
const (
	moveFlagOnTransport     = 0x00000200
	moveFlagFalling         = 0x00001000
	moveFlagSwimming        = 0x00200000
	moveFlagFlying          = 0x02000000
	moveFlagSplineElevation = 0x04000000
	moveFlagSplineEnabled   = 0x08000000

	moveFlag2AlwaysAllowPitching  = 0x0020
	moveFlag2InterpolatedMovement = 0x0400
)

// Spline flags (see AzerothCore MoveSplineFlag.h).
const (
	splineFlagFinalPoint  = 0x00008000
	splineFlagFinalTarget = 0x00010000
	splineFlagFinalAngle  = 0x00020000
)

// UnitStatsUpdate holds stats-related unit fields extracted from an update object packet.
type UnitStatsUpdate struct {
	Level     *uint32
	UnitFlags *uint32
	CurHP     *uint32
	MaxHP     *uint32
	PowerType *uint8
	Powers    [powersCount]*uint32
	MaxPowers [powersCount]*uint32
}

// IsEmpty returns true if no tracked field was present in the packet.
func (u *UnitStatsUpdate) IsEmpty() bool {
	if u.Level != nil || u.UnitFlags != nil || u.CurHP != nil || u.MaxHP != nil || u.PowerType != nil {
		return false
	}
	for i := 0; i < powersCount; i++ {
		if u.Powers[i] != nil || u.MaxPowers[i] != nil {
			return false
		}
	}
	return true
}

// ParseUpdateObjectStatsForGUID extracts stats-related field updates for the given
// character guid from an SMSG_UPDATE_OBJECT payload. The payload is only read, never modified.
func ParseUpdateObjectStatsForGUID(data []byte, charGUID uint64) (UnitStatsUpdate, error) {
	upd := UnitStatsUpdate{}
	r := NewReaderWithData(data)

	blockCount := r.Uint32()
	for i := uint32(0); i < blockCount; i++ {
		if r.Error() != nil {
			return upd, r.Error()
		}

		updateType := r.Uint8()
		switch updateType {
		case updateTypeValues:
			guid := r.ReadGUID()
			parseValuesBlock(r, guid == charGUID, &upd)
		case updateTypeMovement:
			_ = r.ReadGUID()
			skipMovementBlock(r)
		case updateTypeCreateObject, updateTypeCreateObject2:
			guid := r.ReadGUID()
			_ = r.Uint8() // object type id
			skipMovementBlock(r)
			parseValuesBlock(r, guid == charGUID, &upd)
		case updateTypeOutOfRangeObjects, updateTypeNearObjects:
			count := r.Uint32()
			for j := uint32(0); j < count && r.Error() == nil; j++ {
				_ = r.ReadGUID()
			}
		default:
			return upd, fmt.Errorf("unknown update type %d in block %d", updateType, i)
		}
	}

	return upd, r.Error()
}

// DecompressUpdateObject inflates an SMSG_COMPRESSED_UPDATE_OBJECT payload
// (uint32 uncompressed size followed by a zlib stream) into an SMSG_UPDATE_OBJECT payload.
func DecompressUpdateObject(data []byte) ([]byte, error) {
	const maxUncompressedSize = 1 << 22

	if len(data) < 4 {
		return nil, fmt.Errorf("compressed update object payload too short: %d", len(data))
	}

	uncompressedSize := binary.LittleEndian.Uint32(data[:4])
	if uncompressedSize > maxUncompressedSize {
		return nil, fmt.Errorf("compressed update object claims too big size: %d", uncompressedSize)
	}

	zr, err := zlib.NewReader(bytes.NewReader(data[4:]))
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	out := make([]byte, uncompressedSize)
	if _, err = io.ReadFull(zr, out); err != nil {
		return nil, err
	}

	return out, nil
}

// maxStatsFieldsBlock is the last mask block that can contain tracked fields.
const maxStatsFieldsBlock = unitFieldFlags / 32

// parseValuesBlock reads a values part of a block, keeping fields of interest when isTarget is set.
func parseValuesBlock(r *Reader, isTarget bool, upd *UnitStatsUpdate) {
	maskBlocks := int(r.Uint8())

	if !isTarget {
		valuesCount := 0
		for i := 0; i < maskBlocks && r.Error() == nil; i++ {
			valuesCount += bits.OnesCount32(r.Uint32())
		}
		r.Skip(valuesCount * 4)
		return
	}

	masks := make([]uint32, maskBlocks)
	for i := 0; i < maskBlocks && r.Error() == nil; i++ {
		masks[i] = r.Uint32()
	}

	for block := 0; block < maskBlocks && r.Error() == nil; block++ {
		m := masks[block]

		if block > maxStatsFieldsBlock {
			r.Skip(bits.OnesCount32(m) * 4)
			continue
		}

		for m != 0 && r.Error() == nil {
			bit := bits.TrailingZeros32(m)
			m &= m - 1

			idx := block*32 + bit

			switch {
			case idx == unitFieldBytes0:
				// bytes: race, class, gender, power type
				pt := uint8(r.Uint32() >> 24)
				upd.PowerType = &pt
			case idx == unitFieldHealth:
				v := r.Uint32()
				upd.CurHP = &v
			case idx == unitFieldMaxHealth:
				v := r.Uint32()
				upd.MaxHP = &v
			case idx == unitFieldLevel:
				v := r.Uint32()
				upd.Level = &v
			case idx == unitFieldFlags:
				v := r.Uint32()
				upd.UnitFlags = &v
			case idx >= unitFieldPower1 && idx < unitFieldPower1+powersCount:
				v := r.Uint32()
				upd.Powers[idx-unitFieldPower1] = &v
			case idx >= unitFieldMaxPower1 && idx < unitFieldMaxPower1+powersCount:
				v := r.Uint32()
				upd.MaxPowers[idx-unitFieldMaxPower1] = &v
			default:
				r.Skip(4)
			}
		}
	}
}

// skipMovementBlock advances the reader past a movement part of a block
// (see AzerothCore Object::BuildMovementUpdate).
func skipMovementBlock(r *Reader) {
	flags := r.Uint16()

	if flags&updateFlagLiving != 0 {
		moveFlags := r.Uint32()
		moveFlags2 := uint32(r.Uint16())
		r.Skip(4 + 4*4) // time, x, y, z, orientation

		if moveFlags&moveFlagOnTransport != 0 {
			_ = r.ReadGUID()
			r.Skip(4*4 + 4 + 1) // transport offsets, transport time, transport seat
			if moveFlags2&moveFlag2InterpolatedMovement != 0 {
				r.Skip(4) // transport time2
			}
		}

		if moveFlags&(moveFlagSwimming|moveFlagFlying) != 0 || moveFlags2&moveFlag2AlwaysAllowPitching != 0 {
			r.Skip(4) // pitch
		}

		r.Skip(4) // fall time

		if moveFlags&moveFlagFalling != 0 {
			r.Skip(4 * 4) // fall velocity, sin, cos, xy speed
		}

		if moveFlags&moveFlagSplineElevation != 0 {
			r.Skip(4) // spline elevation
		}

		r.Skip(9 * 4) // speeds

		if moveFlags&moveFlagSplineEnabled != 0 {
			skipSplineData(r)
		}
	} else if flags&updateFlagPosition != 0 {
		_ = r.ReadGUID() // transport guid or 0
		// x, y, z, transport offsets (or position again), orientation, corpse orientation
		r.Skip(8 * 4)
	} else if flags&updateFlagStationaryPosition != 0 {
		r.Skip(4 * 4) // x, y, z, orientation
	}

	if flags&updateFlagUnknown != 0 {
		r.Skip(4)
	}

	if flags&updateFlagLowGUID != 0 {
		r.Skip(4)
	}

	if flags&updateFlagHasTarget != 0 {
		_ = r.ReadGUID()
	}

	if flags&updateFlagTransport != 0 {
		r.Skip(4) // transport time
	}

	if flags&updateFlagVehicle != 0 {
		r.Skip(4 + 4) // vehicle id, orientation
	}

	if flags&updateFlagRotation != 0 {
		r.Skip(8) // packed rotation
	}
}

// skipSplineData advances the reader past spline data (see AzerothCore PacketBuilder::WriteCreate).
func skipSplineData(r *Reader) {
	splineFlags := r.Uint32()

	if splineFlags&splineFlagFinalAngle != 0 {
		r.Skip(4) // final angle
	} else if splineFlags&splineFlagFinalTarget != 0 {
		r.Skip(8) // final target guid
	} else if splineFlags&splineFlagFinalPoint != 0 {
		r.Skip(3 * 4) // final point
	}

	// time passed, duration, id, duration mod, duration mod next,
	// vertical acceleration, effect start time
	r.Skip(7 * 4)

	nodes := r.Uint32()
	r.Skip(int(nodes) * 3 * 4) // path points

	r.Skip(1 + 3*4) // spline mode, final destination
}
