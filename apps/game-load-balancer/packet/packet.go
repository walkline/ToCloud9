package packet

import (
	"bytes"
	"encoding/binary"
)

// LoginErrorCode is the code of an error that occurred during login request
type LoginErrorCode uint8

const (
	// LoginErrorCodeLoginFailed is error msg - "Login Failed."
	LoginErrorCodeLoginFailed LoginErrorCode = iota

	// LoginErrorCodeWorldServerIsDown is error msg - "World server is down."
	LoginErrorCodeWorldServerIsDown

	// LoginErrorCodeCharAlreadyExists is error msg - "A character with that name already exists."
	LoginErrorCodeCharAlreadyExists

	// LoginErrorCodeNoInstanceServers is error msg - "No instance servers are available."
	LoginErrorCodeNoInstanceServers

	// LoginErrorCodeDisabled is error msg - "Login for that race, class, or character is currently disabled."
	LoginErrorCodeDisabled

	// LoginErrorCodeCharNotFound is error msg - "Character not found."
	LoginErrorCodeCharNotFound

	// LoginErrorCodeCharUpdateInProgress is error msg - "You cannot log in until the character update process you recently initiated is complete."
	LoginErrorCodeCharUpdateInProgress

	// LoginErrorCodeCharLockedBilling is error msg - "Character locked. Contact Billing for more information."
	LoginErrorCodeCharLockedBilling

	// LoginErrorCodeWarcraftRemote is error msg - "You cannot log in while using World Of Warcraft Remote."
	LoginErrorCodeWarcraftRemote
)

type Packet struct {
	Opcode Opcode
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
