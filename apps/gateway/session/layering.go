package session

import (
	"context"
	"fmt"
	"time"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/sockets"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

func (s *GameSession) selectLayerGameServer(ctx context.Context, groupID uint32, preferredAlias string) (*pbServ.Server, error) {
	if s.character == nil {
		return nil, nil
	}
	response, err := s.serversRegistryClient.AvailableGameServersForMapAndRealm(ctx, &pbServ.AvailableGameServersForMapAndRealmRequest{
		Api: root.SupportedServerRegistryVer, RealmID: root.RealmID, MapID: s.character.Map,
		GroupID: groupID, PreferredGameServerAlias: preferredAlias,
	})
	if err != nil || len(response.GameServers) == 0 {
		return nil, err
	}
	return response.GameServers[0], nil
}

func (s *GameSession) applyGroupLayer(ctx context.Context, groupID uint32) error {
	server, err := s.selectLayerGameServer(ctx, groupID, "")
	if err != nil || server == nil {
		return err
	}
	if server.ID == s.currentGameServerID {
		s.currentGameServerAlias = server.Alias
		return nil
	}
	s.SendSysMessage(fmt.Sprintf("Switching to %s.", server.Alias))
	if err := s.redirectToSelectedLayer(ctx, server); err != nil {
		return err
	}
	return nil
}

func (s *GameSession) redirectToSelectedLayer(ctx context.Context, server *pbServ.Server) error {
	if server == nil || s.character == nil {
		return nil
	}
	s.gameServerGRPCConnMgr.AddAddressMapping(server.Address, server.GrpcAddress)
	client, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(server.Address)
	if err != nil {
		return fmt.Errorf("connect to layer gameserver gRPC: %w", err)
	}
	if err := s.layerPlayerRedirect(ctx, s.character.GUID, server.Address, server.Alias); err != nil {
		return err
	}
	s.gameServerGRPCClient = client
	s.currentGameServerID = server.ID
	s.currentGameServerAlias = server.Alias
	return nil
}

func (s *GameSession) layerPlayerRedirect(ctx context.Context, characterGUID uint64, address, layerAlias string) error {
	if s.worldSocket == nil || s.character == nil {
		return nil
	}

	oldSocket := s.worldSocket
	prepareRedirect := packet.NewWriterWithSize(packet.TC9CMsgPrepareForRedirect, 0)
	if s.seamlessLayerSwitch {
		prepareRedirect = packet.NewWriterWithSize(packet.TC9CMsgPrepareForRedirect, 2).
			Uint8(packet.TC9RedirectVersionedRequest).
			Uint8(packet.TC9RedirectOptionSeamless)
	}
	oldSocket.Send(prepareRedirect)
	var onSourcePacket func(*packet.Packet)
	if s.seamlessLayerSwitch {
		onSourcePacket = s.forwardLayerTransitionPacket
	}
	acceptedOptions, err := waitForWorldServerRedirect(ctx, oldSocket, onSourcePacket)
	if err != nil {
		return fmt.Errorf("prepare layer redirect for account %d: %w", s.accountID, err)
	}
	seamlessRedirect := s.seamlessLayerSwitch &&
		acceptedOptions&packet.TC9RedirectOptionSeamless != 0

	oldSocket.Close()
	s.worldSocket = nil
	s.worldEntryPending = true

	newSocket, err := s.connectToGameServerWithAddress(ctx, characterGUID, address, nil)
	if err != nil {
		return fmt.Errorf("connect to layer gameserver %s: %w", address, err)
	}

	if !seamlessRedirect {
		newWorld := packet.NewWriterWithSize(packet.SMsgNewWorld, 0)
		newWorld.Uint32(s.character.Map)
		newWorld.Float32(s.character.PositionX)
		newWorld.Float32(s.character.PositionY)
		newWorld.Float32(s.character.PositionZ)
		newWorld.Float32(s.character.PositionO)
		s.gameSocket.Send(newWorld)
	}

	firstPacket, err := waitForFirstWorldPacket(ctx, newSocket)
	if err != nil {
		newSocket.Close()
		return fmt.Errorf("wait for layer gameserver %s: %w", address, err)
	}

	s.worldSocket = newSocket
	// Don't use HandleMap here: chat handlers -> layer redirect would init-cycle.
	if !OpcodeBlacklist[firstPacket.Opcode] && s.gameSocket != nil {
		s.gameSocket.WriteChannel() <- firstPacket
	}

	if s.showGameserverConnChangeToClient {
		s.SendSysMessage(fmt.Sprintf("You have been moved to %s layer.", layerAlias))
	}

	return nil
}

// forwardLayerTransitionPacket keeps the client's current world stream alive
// while the source core prepares a seamless redirect. In particular, an
// AzerothCore phase change emits the native out-of-range updates that make the
// old layer fade out. The gateway deliberately treats these packets as opaque:
// it neither parses nor retains any visible-world state.
func (s *GameSession) forwardLayerTransitionPacket(p *packet.Packet) {
	if p == nil || s.gameSocket == nil || OpcodeBlacklist[p.Opcode] {
		return
	}
	s.gameSocket.WriteChannel() <- p
}

func waitForWorldServerRedirect(ctx context.Context, socket sockets.Socket, onPacket func(*packet.Packet)) (uint8, error) {
	confirmationContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for {
		select {
		case <-confirmationContext.Done():
			return 0, confirmationContext.Err()
		case response, open := <-socket.ReadChannel():
			if !open {
				return 0, nil
			}
			if response.Opcode == packet.TC9SMsgReadyForRedirect {
				reader := response.Reader()
				status := reader.Uint8()
				if reader.Error() != nil {
					return 0, fmt.Errorf("worldserver returned malformed redirect acknowledgement")
				}
				if status != 0 {
					return 0, fmt.Errorf("worldserver rejected redirect")
				}

				// A one-byte success is the legacy acknowledgement. The redirect is
				// still valid, but the gateway must use SMSG_NEW_WORLD because the
				// source core did not explicitly accept the seamless option.
				if reader.Left() == 0 {
					return 0, nil
				}

				version := reader.Uint8()
				acceptedOptions := reader.Uint8()
				if reader.Error() != nil || reader.Left() != 0 || version != packet.TC9RedirectVersionedRequest {
					return 0, nil
				}
				return acceptedOptions & packet.TC9RedirectSupportedOptions, nil
			}
			if onPacket != nil {
				onPacket(response)
			}
		}
	}
}

func waitForFirstWorldPacket(ctx context.Context, socket sockets.Socket) (*packet.Packet, error) {
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	select {
	case <-waitCtx.Done():
		return nil, waitCtx.Err()
	case response, open := <-socket.ReadChannel():
		if !open {
			return nil, fmt.Errorf("world socket closed")
		}
		return response, nil
	}
}
