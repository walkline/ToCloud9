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

// layerPlayerRedirect performs a regular client world transition while moving
// a character between gameservers that host different copies of the same map.
// Battleground redirects intentionally keep their separate protocol.
func (s *GameSession) layerPlayerRedirect(ctx context.Context, characterGUID uint64, address, layerAlias string) error {
	if s.worldSocket == nil || s.character == nil {
		return nil
	}

	oldSocket := s.worldSocket
	oldSocket.Send(packet.NewWriterWithSize(packet.TC9CMsgPrepareForRedirect, 0))
	if err := waitForWorldServerRedirect(ctx, oldSocket); err != nil {
		return fmt.Errorf("prepare layer redirect for account %d: %w", s.accountID, err)
	}

	oldSocket.Close()
	s.worldSocket = nil
	s.worldEntryPending = true

	newSocket, err := s.connectToGameServerWithAddress(ctx, characterGUID, address, nil)
	if err != nil {
		return fmt.Errorf("connect to layer gameserver %s: %w", address, err)
	}

	readyPacket, pendingPackets, err := waitForLayerWorldReady(ctx, newSocket)
	if err != nil {
		newSocket.Close()
		return fmt.Errorf("wait for layer gameserver %s: %w", address, err)
	}

	// The destination worldserver logs the character into the same map, so it
	// sends SMSG_LOGIN_VERIFY_WORLD rather than a map-changing SMSG_NEW_WORLD.
	// Use its post-save position and facing for the synthetic transfer.
	if readyPacket.Opcode == packet.SMsgNewWorld {
		s.gameSocket.SendPacket(readyPacket)
	} else {
		ready := readyPacket.Reader()
		mapID := ready.Uint32()
		x := ready.Float32()
		y := ready.Float32()
		z := ready.Float32()
		o := ready.Float32()
		if err := ready.Error(); err != nil {
			newSocket.Close()
			return fmt.Errorf("read layer destination position: %w", err)
		}

		// SMSG_TRANSFER_PENDING is what makes the client enter its normal
		// loading-screen state. It is required here even though the map ID is
		// unchanged, because the backing worldserver is changing.
		transferPending := packet.NewWriterWithSize(packet.SMsgTransferPending, 0)
		transferPending.Uint32(mapID)
		s.gameSocket.Send(transferPending)

		newWorld := packet.NewWriterWithSize(packet.SMsgNewWorld, 0)
		newWorld.Uint32(mapID)
		newWorld.Float32(x)
		newWorld.Float32(y)
		newWorld.Float32(z)
		newWorld.Float32(o)
		s.gameSocket.Send(newWorld)
	}
	s.layerPendingPackets = pendingPackets
	s.layerWorldAckPending = true
	s.worldSocket = newSocket
	if s.showGameserverConnChangeToClient {
		s.SendSysMessage(fmt.Sprintf("You have been moved to %s layer.", layerAlias))
	}

	return nil
}

func waitForWorldServerRedirect(ctx context.Context, socket sockets.Socket) error {
	confirmationContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for {
		select {
		case <-confirmationContext.Done():
			return confirmationContext.Err()
		case response, open := <-socket.ReadChannel():
			if !open {
				// A closed source socket also means it completed the handoff.
				return nil
			}
			if response.Opcode == packet.TC9SMsgReadyForRedirect {
				if response.Reader().Uint8() != 0 {
					return fmt.Errorf("worldserver rejected redirect")
				}
				return nil
			}
		}
	}
}

func waitForLayerWorldReady(ctx context.Context, socket sockets.Socket) (*packet.Packet, []*packet.Packet, error) {
	pendingPackets := make([]*packet.Packet, 0, 8)
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case response, open := <-socket.ReadChannel():
			if !open {
				return nil, nil, fmt.Errorf("world socket closed")
			}
			switch response.Opcode {
			case packet.SMsgLoginVerifyWorld, packet.SMsgNewWorld:
				return response, pendingPackets, nil
			default:
				// AzerothCore can emit ordinary character initialization
				// packets before SMSG_LOGIN_VERIFY_WORLD. Preserve them and
				// forward them after starting the client's world transition.
				pendingPackets = append(pendingPackets, response)
			}
		}
	}
}
