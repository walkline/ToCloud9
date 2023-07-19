package packet

import (
	"bytes"
	"encoding/binary"
)

//type GameServerOpcode uint16
//
//const (
//	CMsgCharCreate             = 0x036
//	CMsgCharEnum               = 0x037
//	CMsgCharDelete             = 0x038
//	SMsgCharCreate             = 0x03A
//	SMsgCharEnum               = 0x03B
//	SMsgCharDelete             = 0x03C
//	CMsgPlayerLogin            = 0x03D
//	SMsgNewWorld               = 0x03E
//	SMsgTransferPending        = 0x03F
//	SMsgCharacterLoginFailed   = 0x041
//	SMsgLogoutComplete         = 0x04D
//	CMsgGuildQuery             = 0x054
//	SMsgGuildQueryResponse     = 0x055
//	CMsgWho                    = 0x062
//	SMsgWho                    = 0x063
//	CMsgGuildInvite            = 0x082
//	SMsgGuildInvite            = 0x083
//	CMsgGuildInviteAccept      = 0x084
//	CMsgGuildRoster            = 0x089
//	SMsgGuildRoster            = 0x08A
//	CMsgGuildPromote           = 0x08B
//	CMsgGuildDemote            = 0x08C
//	CMsgGuildLeave             = 0x08D
//	CMsgGuildRemove            = 0x08E
//	CMsgGuildSMTD              = 0x091
//	SMsgGuildEvent             = 0x092
//	CMsgMessageChat            = 0x095
//	SMsgMessageChat            = 0x096
//	MsgMoveWorldPortAck        = 0x0DC
//	SMsgTutorialFlags          = 0x0FD
//	SMsgLevelUpInfo            = 0x1D4
//	CMsgPing                   = 0x1DC
//	SMsgPong                   = 0x1DD
//	SMsgAuthChallenge          = 0x1EC
//	CMsgAuthSession            = 0x1ED
//	SMsgAuthResponse           = 0x1EE
//	SMsgAccountDataTimes       = 0x209
//	CMsgGuildRank              = 0x231
//	CMsgGuildAddRank           = 0x232
//	CMsgGuildDelRank           = 0x233
//	CMsgGuildSetPublicNote     = 0x234
//	CMsgGuildSetOfficerNote    = 0x235
//	CMsgSendMail               = 0x238
//	SMsgSendMailResult         = 0x239
//	CMsgGetMailList            = 0x23A
//	SMsgMailListResult         = 0x23B
//	CMsgMailTakeMoney          = 0x245
//	CMsgMailTakeItem           = 0x246
//	CMsgMailMarkAsRead         = 0x247
//	CMsgMailDelete             = 0x249
//	MsgQueryNextMailTime       = 0x284
//	SMsgReceivedMail           = 0x285
//	SMsgInitWorldStates        = 0x2C2
//	SMsgAddonInfo              = 0x2EF
//	CMsgGuildInfoText          = 0x2FC
//	SMsgRealmSplit             = 0x38B
//	CMsgRealmSplit             = 0x38C
//	MsgGuildPermissions        = 0x3FD
//	MsgGuildBankMoneyWithdrawn = 0x3FE
//
//	CMsgReadyForAccountDataTimes = 0x4FF
//	TC9CMsgPrepareForRedirect    = 0x51F
//	TC9SMsgReadyForRedirect      = 0x520
//)

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
