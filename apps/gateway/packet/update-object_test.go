package packet

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCharGUID uint64 = 0x60554

func valuesPart(w *Writer, fields map[int]uint32) {
	maxIdx := 0
	for idx := range fields {
		if idx > maxIdx {
			maxIdx = idx
		}
	}

	maskBlocks := maxIdx/32 + 1
	masks := make([]uint32, maskBlocks)
	for idx := range fields {
		masks[idx/32] |= 1 << (idx % 32)
	}

	w.Uint8(uint8(maskBlocks))
	for _, m := range masks {
		w.Uint32(m)
	}

	// values follow in ascending field index order
	for i := 0; i <= maxIdx; i++ {
		if v, ok := fields[i]; ok {
			w.Uint32(v)
		}
	}
}

func selfValuesBlockPacket(fields map[int]uint32) []byte {
	w := NewWriter(SMsgUpdateObject)
	w.Uint32(1) // block count
	w.Uint8(updateTypeValues)
	w.GUID(testCharGUID)
	valuesPart(w, fields)
	return w.ToPacket().Data
}

func TestParseUpdateObjectStatsValuesBlock(t *testing.T) {
	data := selfValuesBlockPacket(map[int]uint32{
		unitFieldBytes0:        0x03 << 24, // power type: energy
		unitFieldHealth:        1234,
		unitFieldMaxHealth:     2000,
		unitFieldPower1 + 3:    55,
		unitFieldMaxPower1 + 3: 100,
		unitFieldLevel:         12,
	})

	upd, err := ParseUpdateObjectStatsForGUID(data, testCharGUID)
	require.NoError(t, err)

	require.NotNil(t, upd.PowerType)
	assert.Equal(t, uint8(3), *upd.PowerType)
	require.NotNil(t, upd.CurHP)
	assert.Equal(t, uint32(1234), *upd.CurHP)
	require.NotNil(t, upd.MaxHP)
	assert.Equal(t, uint32(2000), *upd.MaxHP)
	require.NotNil(t, upd.Powers[3])
	assert.Equal(t, uint32(55), *upd.Powers[3])
	require.NotNil(t, upd.MaxPowers[3])
	assert.Equal(t, uint32(100), *upd.MaxPowers[3])
	require.NotNil(t, upd.Level)
	assert.Equal(t, uint32(12), *upd.Level)
}

// TestParseUpdateObjectStatsReadsCorrectMaxPowerSlot pins the max-power field index to
// UNIT_FIELD_MAXPOWER1 (OBJECT_END+0x1B). The field map uses literal 3.3.5a indices as an
// independent oracle, so a wrong constant (e.g. reading MAXPOWER7's slot) is caught here.
func TestParseUpdateObjectStatsReadsCorrectMaxPowerSlot(t *testing.T) {
	const (
		fieldBytes0    = 0x6 + 0x11
		fieldPower1    = 0x6 + 0x13 // current mana
		fieldMaxPower1 = 0x6 + 0x1B // max mana
		fieldMaxPower7 = 0x6 + 0x21 // decoy, must not be read as max mana
	)
	data := selfValuesBlockPacket(map[int]uint32{
		fieldBytes0:    0x00 << 24, // power type: mana (0)
		fieldPower1:    50,
		fieldMaxPower1: 200,
		fieldMaxPower7: 999,
	})

	upd, err := ParseUpdateObjectStatsForGUID(data, testCharGUID)
	require.NoError(t, err)
	require.NotNil(t, upd.Powers[0])
	assert.Equal(t, uint32(50), *upd.Powers[0])
	require.NotNil(t, upd.MaxPowers[0])
	assert.Equal(t, uint32(200), *upd.MaxPowers[0])
}

func TestParseUpdateObjectStatsIgnoresOtherGUIDs(t *testing.T) {
	w := NewWriter(SMsgUpdateObject)
	w.Uint32(1)
	w.Uint8(updateTypeValues)
	w.GUID(testCharGUID + 1)
	valuesPart(w, map[int]uint32{unitFieldHealth: 777})

	upd, err := ParseUpdateObjectStatsForGUID(w.ToPacket().Data, testCharGUID)
	require.NoError(t, err)
	assert.True(t, upd.IsEmpty())
}

func TestParseUpdateObjectStatsSkipsFieldsBeyondStatsBlocks(t *testing.T) {
	w := NewWriter(SMsgUpdateObject)
	w.Uint32(2) // block count

	// values block with fields far beyond the tracked ones (player fields area)
	w.Uint8(updateTypeValues)
	w.GUID(testCharGUID)
	valuesPart(w, map[int]uint32{
		unitFieldHealth:                 1234,
		(maxStatsFieldsBlock+2)*32 + 3:  777,
		(maxStatsFieldsBlock+3)*32 + 10: 888,
	})

	// following block must still be parsed at the right offset
	w.Uint8(updateTypeValues)
	w.GUID(testCharGUID)
	valuesPart(w, map[int]uint32{unitFieldLevel: 42})

	upd, err := ParseUpdateObjectStatsForGUID(w.ToPacket().Data, testCharGUID)
	require.NoError(t, err)
	require.NotNil(t, upd.CurHP)
	assert.Equal(t, uint32(1234), *upd.CurHP)
	require.NotNil(t, upd.Level)
	assert.Equal(t, uint32(42), *upd.Level)
}

func TestParseUpdateObjectStatsCreateBlockWithMovement(t *testing.T) {
	w := NewWriter(SMsgUpdateObject)
	w.Uint32(3) // block count

	// out of range objects block
	w.Uint8(updateTypeOutOfRangeObjects)
	w.Uint32(2)
	w.GUID(111)
	w.GUID(222)

	// create block for another unit, with movement data to skip
	w.Uint8(updateTypeCreateObject2)
	w.GUID(4444)
	w.Uint8(4) // object type id
	w.Uint16(updateFlagLiving)
	w.Uint32(moveFlagSwimming | moveFlagFalling | moveFlagSplineEnabled)
	w.Uint16(0)      // move flags 2
	w.Uint32(123456) // time
	w.Float32(1).Float32(2).Float32(3).Float32(4)
	w.Float32(0.5)                                // pitch (swimming)
	w.Uint32(100)                                 // fall time
	w.Float32(1).Float32(2).Float32(3).Float32(4) // jump data (falling)
	for i := 0; i < 9; i++ {                      // speeds
		w.Float32(7)
	}
	w.Uint32(splineFlagFinalPoint)
	w.Float32(1).Float32(2).Float32(3) // final point
	w.Uint32(10)                       // time passed
	w.Uint32(20)                       // duration
	w.Uint32(30)                       // id
	w.Float32(1).Float32(1)            // duration mods
	w.Float32(0)                       // vertical acceleration
	w.Uint32(0)                        // effect start time
	w.Uint32(2)                        // nodes count
	for i := 0; i < 6; i++ {
		w.Float32(float32(i))
	}
	w.Uint8(0)                         // spline mode
	w.Float32(9).Float32(9).Float32(9) // final destination
	valuesPart(w, map[int]uint32{unitFieldHealth: 777})

	// values block for the tracked character
	w.Uint8(updateTypeValues)
	w.GUID(testCharGUID)
	valuesPart(w, map[int]uint32{unitFieldHealth: 4321})

	upd, err := ParseUpdateObjectStatsForGUID(w.ToPacket().Data, testCharGUID)
	require.NoError(t, err)
	require.NotNil(t, upd.CurHP)
	assert.Equal(t, uint32(4321), *upd.CurHP)
	assert.Nil(t, upd.MaxHP)
}

func TestParseUpdateObjectStatsTruncatedPacket(t *testing.T) {
	data := selfValuesBlockPacket(map[int]uint32{unitFieldHealth: 1234})

	_, err := ParseUpdateObjectStatsForGUID(data[:len(data)-3], testCharGUID)
	assert.Error(t, err)
}

func TestDecompressUpdateObject(t *testing.T) {
	data := selfValuesBlockPacket(map[int]uint32{unitFieldHealth: 1234})

	var compressed bytes.Buffer
	sizeHeader := make([]byte, 4)
	binary.LittleEndian.PutUint32(sizeHeader, uint32(len(data)))
	compressed.Write(sizeHeader)
	zw := zlib.NewWriter(&compressed)
	_, err := zw.Write(data)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	decompressed, err := DecompressUpdateObject(compressed.Bytes())
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)

	upd, err := ParseUpdateObjectStatsForGUID(decompressed, testCharGUID)
	require.NoError(t, err)
	require.NotNil(t, upd.CurHP)
	assert.Equal(t, uint32(1234), *upd.CurHP)
}
