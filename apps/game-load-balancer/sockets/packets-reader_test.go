package sockets

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
	"github.com/walkline/ToCloud9/shared/slices"
)

func TestPacketsReader(t *testing.T) {
	r := bytes.NewReader(
		slices.AppendBytes(
			writerToBytes(packet.NewWriter(packet.CMsgPlayerLogin).Uint64(1)),
			writerToBytes(packet.NewWriter(packet.CMsgAuthSession).Uint64(1).Uint64(2).Uint64(3).Uint64(4)),
		),
	)

	reader := NewPacketsReader(r, 4, packet.SourceUnknown)
	result := []packet.Packet{}
	for reader.Next() {
		result = append(result, *reader.Packet())
	}

	assert.Equal(t, io.EOF, reader.Error())

	assert.Equal(t, 2, len(result))

	assert.Equal(t, packet.CMsgPlayerLogin, result[0].Opcode)
	assert.Equal(t, packet.CMsgAuthSession, result[1].Opcode)

	assert.Equal(t, uint32(8), result[0].Size)
	assert.Equal(t, uint32(8*4), result[1].Size)
}

func writerToBytes(p *packet.Writer) []byte {
	header := make([]byte, 6, len(p.Payload.Bytes())+6)
	binary.BigEndian.PutUint16(header[0:2], uint16(len(p.Payload.Bytes())+4))
	binary.LittleEndian.PutUint16(header[2:6], uint16(p.Opcode))

	return append(header, p.Payload.Bytes()...)
}
