package session

import (
	"context"
	"fmt"
	"time"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

type layerSwitchRequest struct {
	preferredPlayerGUID uint64
	notBefore           time.Time
}

func (s *GameSession) queueLayerSwitchToPlayer(preferredPlayerGUID uint64) {
	if !s.layeringEnabled || preferredPlayerGUID == 0 || s.character == nil {
		return
	}
	select {
	case s.layerSwitchQueue <- layerSwitchRequest{preferredPlayerGUID: preferredPlayerGUID}:
	default:
		s.SendSysMessage("Your layer switch queue is full. Please try again shortly.")
	}
}

func (s *GameSession) processNextLayerSwitch(ctx context.Context) error {
	if !s.layeringEnabled || s.character == nil || s.layerSwitchInProgress || s.teleportingToNewMap != nil {
		return nil
	}
	var request layerSwitchRequest
	select {
	case request = <-s.layerSwitchQueue:
	default:
		return nil
	}
	if time.Now().Before(request.notBefore) {
		select {
		case s.layerSwitchQueue <- request:
		default:
		}
		return nil
	}

	selection, err := s.serversRegistryClient.SelectGameServerForPlayer(ctx, &pbServ.SelectGameServerForPlayerRequest{
		Api:                      root.SupportedServerRegistryVer,
		RealmID:                  root.RealmID,
		MapID:                    s.character.Map,
		PlayerGUID:               s.character.GUID,
		PreferredPlayerGUID:      request.preferredPlayerGUID,
		Reason:                   pbServ.SelectGameServerForPlayerRequest_GROUP_JOIN,
		CurrentGameServerAddress: s.currentServerAddress,
	})
	if err != nil {
		return err
	}
	if selection.Status != pbServ.SelectGameServerForPlayerResponse_OK {
		switch selection.Status {
		case pbServ.SelectGameServerForPlayerResponse_THROTTLED:
			s.SendSysMessage(fmt.Sprintf("Layer switch is cooling down and remains queued for %d seconds.", selection.RetryAfterSeconds))
			request.notBefore = time.Now().Add(time.Duration(selection.RetryAfterSeconds) * time.Second)
			s.layerSwitchQueue <- request
		case pbServ.SelectGameServerForPlayerResponse_HOURLY_LIMIT_REACHED:
			s.SendSysMessage(fmt.Sprintf("Hourly layer switch limit reached; the move remains queued for %d seconds.", selection.RetryAfterSeconds))
			request.notBefore = time.Now().Add(time.Duration(selection.RetryAfterSeconds) * time.Second)
			s.layerSwitchQueue <- request
		default:
			s.SendSysMessage("No compatible layer is currently available.")
		}
		return nil
	}
	if selection.GameServer == nil || selection.GameServer.Address == s.currentServerAddress {
		s.currentLayerID = selection.LayerID
		return nil
	}

	// Trigger the normal client world-port acknowledgement. The existing
	// redirect handshake then saves the character and reconnects it to the
	// selected core, even though the map itself does not change.
	s.layerSwitchInProgress = true
	s.layerSwitchTarget = selection.GameServer
	mapID := s.character.Map
	s.teleportingToNewMap = &mapID

	pending := packet.NewWriterWithSize(packet.SMsgTransferPending, 4)
	pending.Uint32(mapID)
	s.gameSocket.Send(pending)

	newWorld := packet.NewWriterWithSize(packet.SMsgNewWorld, 20)
	newWorld.Uint32(mapID)
	newWorld.Float32(s.character.PositionX)
	newWorld.Float32(s.character.PositionY)
	newWorld.Float32(s.character.PositionZ)
	newWorld.Float32(s.character.PositionO)
	s.gameSocket.Send(newWorld)
	return nil
}
