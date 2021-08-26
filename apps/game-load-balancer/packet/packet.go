package packet

import (
	"bytes"
	"encoding/binary"
)

type GameServerOpcode uint16

const (
	CMsgCharCreate           = 0x036
	CMsgCharEnum             = 0x037
	CMsgCharDelete           = 0x038
	SMsgCharCreate           = 0x03A
	SMsgCharEnum             = 0x03B
	SMsgCharDelete           = 0x03C
	CMsgPlayerLogin          = 0x03D
	SMsgNewWorld             = 0x03E
	SMsgCharacterLoginFailed = 0x041
	SMsgLogoutComplete       = 0x04D
	CMsgMessageChat          = 0x095
	SMsgMessageChat          = 0x096
	SMsgTutorialFlags        = 0x0FD
	CMsgPing                 = 0x1DC
	CMsgPong                 = 0x1DD
	SMsgAuthChallenge        = 0x1EC
	CMsgAuthSession          = 0x1ED
	SMsgAuthResponse         = 0x1EE
	SMsgAccountDataTimes     = 0x209
	SMsgAddonInfo            = 0x2EF
	SMsgRealmSplit           = 0x38B
	CMsgRealmSplit           = 0x38C

	CMsgReadyForAccountDataTimes = 0x4FF
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
	Opcode uint16
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
