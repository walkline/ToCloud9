package sockets

import (
	"encoding/binary"
	"io"

	"github.com/walkline/ToCloud9/apps/game-load-balancer/crypto"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
)

type PacketsReader struct {
	r io.Reader

	encryption *crypto.Arc

	opcodeSize uint32

	headerBuffer []byte
	hWritePos    int

	payloadBuffer []byte
	pWritePos     int

	err error

	packet     *packet.Packet
	sourceType packet.Source
}

func NewPacketsReader(r io.Reader, opcodeSize uint32, sourceType packet.Source) *PacketsReader {
	return &PacketsReader{
		r:            r,
		opcodeSize:   opcodeSize,
		headerBuffer: make([]byte, 2+opcodeSize),
	}
}

func (p *PacketsReader) Next() bool {
	pack := packet.Packet{
		Source: p.sourceType,
	}
	for {
		if len(p.headerBuffer) > p.hWritePos {
			n, err := p.r.Read(p.headerBuffer[p.hWritePos:])
			if err != nil {
				p.err = err
				return false
			}
			p.hWritePos += n

			// header ready
			if len(p.headerBuffer) == p.hWritePos {
				if p.encryption != nil {
					p.encryption.Decrypt(p.headerBuffer)
				}

				pack.Size = uint32(binary.BigEndian.Uint16(p.headerBuffer[0:])) - p.opcodeSize
				if p.opcodeSize == 2 {
					pack.Opcode = packet.Opcode(binary.LittleEndian.Uint16(p.headerBuffer[2:]))
				} else {
					pack.Opcode = packet.Opcode(binary.LittleEndian.Uint32(p.headerBuffer[2:]))
				}
				p.payloadBuffer = make([]byte, pack.Size)
			} else {
				continue
			}
		}

		if len(p.payloadBuffer) > p.pWritePos {
			n, err := p.r.Read(p.payloadBuffer[p.pWritePos:])
			if err != nil {
				p.err = err
				return false
			}
			p.pWritePos += n
		}

		// we have full payload
		if len(p.payloadBuffer) == p.pWritePos {
			p.hWritePos = 0
			p.pWritePos = 0
			p.packet = &pack
			p.packet.Data = p.payloadBuffer[:]

			return true
		}
	}
}

func (p *PacketsReader) Error() error {
	return p.err
}

func (p *PacketsReader) Packet() *packet.Packet {
	return p.packet
}

func (p *PacketsReader) EnableEncryption(crypto *crypto.Arc) {
	p.encryption = crypto
}
