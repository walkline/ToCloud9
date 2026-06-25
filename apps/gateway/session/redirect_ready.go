package session

import (
	"context"
	"fmt"

	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/sockets"
)

func redirectReadyStatus(p *packet.Packet) (uint8, error) {
	if p == nil {
		return 1, fmt.Errorf("TC9 redirect ready packet is nil")
	}
	if p.Opcode != packet.TC9SMsgReadyForRedirect {
		return 1, fmt.Errorf("unexpected redirect ready opcode %s", p.Opcode.String())
	}
	if p.Size == 0 {
		return 1, fmt.Errorf("TC9 redirect ready packet has no status byte")
	}

	reader := p.Reader()
	status := reader.Uint8()
	if err := reader.Error(); err != nil {
		return 1, fmt.Errorf("can't read TC9 redirect ready status: %w", err)
	}

	return status, nil
}

func (s *GameSession) waitForSourceRedirectReady(ctx context.Context, socket sockets.Socket, redirectID string, playerGUID uint64, mapID uint32, sourceAddress string, targetAddress string) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for TC9 redirect ready, redirect %s, account %d, character %d, map %d, source %s, target %s: %w", redirectID, s.accountID, playerGUID, mapID, sourceAddress, targetAddress, ctx.Err())
		case p, open := <-socket.ReadChannel():
			if !open {
				return fmt.Errorf("source world socket closed before TC9 redirect ready, redirect %s, account %d, character %d, map %d, source %s, target %s", redirectID, s.accountID, playerGUID, mapID, sourceAddress, targetAddress)
			}
			if p.Opcode != packet.TC9SMsgReadyForRedirect {
				s.logger.Debug().
					Str("redirect", redirectID).
					Uint32("account", s.accountID).
					Uint64("character", playerGUID).
					Uint32("map", mapID).
					Uint16("opcode", uint16(p.Opcode)).
					Str("source", sourceAddress).
					Str("target", targetAddress).
					Msg("Ignoring source world packet while waiting for TC9 redirect ready")
				continue
			}

			status, err := redirectReadyStatus(p)
			if err != nil {
				return fmt.Errorf("invalid TC9 redirect ready, redirect %s, account %d, character %d, map %d, source %s, target %s: %w", redirectID, s.accountID, playerGUID, mapID, sourceAddress, targetAddress, err)
			}
			if status != 0 {
				return fmt.Errorf("source worldserver rejected redirect %s with status %d, account %d, character %d, map %d, source %s, target %s", redirectID, status, s.accountID, playerGUID, mapID, sourceAddress, targetAddress)
			}
			return nil
		}
	}
}
