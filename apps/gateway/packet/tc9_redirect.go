package packet

// The original TC9 redirect exchange is unversioned:
//
//	request:  empty
//	response: uint8 status
//
// Version 2 adds option negotiation while retaining status as the first
// response byte, so a new gateway can detect an older core and fall back:
//
//	request:  uint8 version, uint8 requestedOptions
//	response: uint8 status, uint8 version, uint8 acceptedOptions
const (
	TC9RedirectVersionedRequest uint8 = 2
	TC9RedirectOptionSeamless   uint8 = 1 << 0
	TC9RedirectSupportedOptions       = TC9RedirectOptionSeamless
)
