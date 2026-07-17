package packet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewDestroyObjectPacketUsesUnpackedGUID(t *testing.T) {
	const objectGUID = uint64(0xF130001234ABCDEF)
	p := NewDestroyObjectPacket(objectGUID, false)
	require.Equal(t, SMsgDestroyObject, p.Opcode)
	require.Len(t, p.Data, 9)
	r := p.Reader()
	require.Equal(t, objectGUID, r.Uint64())
	require.Equal(t, uint8(0), r.Uint8())
}
