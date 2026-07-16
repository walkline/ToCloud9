package session

import (
	"context"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
)

func (s *GameSession) HandleMovement(ctx context.Context, p *packet.Packet) error {
	defer func() {
		if p.Source == packet.SourceGameClient && s.worldSocket != nil {
			s.worldSocket.SendPacket(p)
			return
		}

		if p.Source == packet.SourceWorldServer && s.gameSocket != nil {
			s.gameSocket.SendPacket(p)
			return
		}
	}()

	if p.Source == packet.SourceWorldServer {
		return nil
	}

	r := p.Reader()
	if r.ReadGUID() != s.character.GUID {
		return nil
	}

	flags := r.Uint32()
	const (
		movementFlagForward    = uint32(0x00000001)
		movementFlagFalling    = uint32(0x00001000)
		movementFlagFallingFar = uint32(0x00002000)
	)
	s.layerSafety.movingForward = flags&movementFlagForward != 0
	s.layerSafety.falling = flags&(movementFlagFalling|movementFlagFallingFar) != 0
	_ = r.Uint16() // flags2
	_ = r.Uint32() // time

	s.character.PositionX, s.character.PositionY, s.character.PositionZ, s.character.PositionO = r.Float32(), r.Float32(), r.Float32(), r.Float32()

	return nil
}
