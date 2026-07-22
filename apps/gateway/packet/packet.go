package packet

import (
	"bytes"
	"encoding/binary"
)

// LoginErrorCode is the code of an error that occurred during login request
type LoginErrorCode uint8

const (
	// LoginErrorCodeDuplicateCharacter indicates that the character or its
	// account already has an active world session.
	LoginErrorCodeDuplicateCharacter LoginErrorCode = 0x4F

	// LoginErrorCodeLoginFailed is error msg - "Login Failed."
	LoginErrorCodeLoginFailed LoginErrorCode = 0x51

	// LoginErrorCodeWorldServerIsDown is error msg - "World server is down."
	LoginErrorCodeWorldServerIsDown LoginErrorCode = 0x4E

	// LoginErrorCodeNoInstanceServers is error msg - "No instance servers are available."
	LoginErrorCodeNoInstanceServers LoginErrorCode = 0x50

	// LoginErrorCodeCharNotFound is error msg - "Character not found."
	LoginErrorCodeCharNotFound LoginErrorCode = 0x53
)

type Source uint8

const (
	SourceUnknown = iota
	SourceGameClient
	SourceWorldServer
)

type Packet struct {
	Opcode Opcode
	Source Source
	Size   uint32
	Data   []byte
}

func (p Packet) Reader() *Reader {
	return &Reader{
		data:  bytes.NewReader(p.Data),
		order: binary.LittleEndian,
		err:   nil,
	}
}
