package session

import (
	"context"
	"fmt"
	"time"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/sockets"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

type mapTransferRouting struct {
	realmID      uint32
	isCrossRealm bool
	feature      clusterTransferFeature
	ownerAddress string
}

type clusterTransferFeature uint8

const (
	clusterTransferFeatureGeneric clusterTransferFeature = iota
	clusterTransferFeatureLFG
	clusterTransferFeatureBattleground
	clusterTransferFeatureArena
	clusterTransferFeatureWintergrasp
)

const transferAbortNotFound uint8 = 3

func (feature clusterTransferFeature) String() string {
	switch feature {
	case clusterTransferFeatureLFG:
		return "lfg"
	case clusterTransferFeatureBattleground:
		return "battleground"
	case clusterTransferFeatureArena:
		return "arena"
	case clusterTransferFeatureWintergrasp:
		return "wintergrasp"
	case clusterTransferFeatureGeneric:
		return "generic"
	default:
		return "unknown"
	}
}

type clusterNativeWorldportTransport struct {
	feature                         clusterTransferFeature
	operation                       string
	playerGUID                      uint64
	routing                         *mapTransferRouting
	reloadManagedGroupAfterTransfer bool
	start                           func(context.Context) error
}

type clusterOwnerNativeWorldportTransport struct {
	feature                         clusterTransferFeature
	operation                       string
	sessionPlayerGUID               uint64
	loginPlayerGUID                 uint64
	targetAddress                   string
	targetWorldserverID             string
	routing                         *mapTransferRouting
	forwardAfterPlacement           bool
	forwardOptions                  nativeWorldportForwardOptions
	reloadManagedGroupAfterTransfer bool
	onOwnerPlaced                   func(context.Context) error
}

type nativeWorldportForwardOptions struct {
	feature                            clusterTransferFeature
	synthesizeTransferPendingForNewMap bool
	expectedMapID                      uint32
	acceptedMapIDs                     []uint32
}

func clusterTransferPrepareRedirectPacket(feature clusterTransferFeature) *packet.Writer {
	return packet.NewWriterWithSize(packet.TC9CMsgPrepareForRedirect, 1).Uint8(uint8(feature))
}

func cloneMapTransferRouting(routing *mapTransferRouting) *mapTransferRouting {
	if routing == nil {
		return nil
	}
	cloned := *routing
	return &cloned
}

func mapTransferRoutingUsesCrossrealmOwner(routing *mapTransferRouting) bool {
	return routing != nil && routing.isCrossRealm
}

func shouldBlockLocalFallbackForMapTransfer(routing *mapTransferRouting) bool {
	return mapTransferRoutingUsesCrossrealmOwner(routing)
}

func shouldKeepMapTransferOnCurrentOwner(routing *mapTransferRouting, currentAddress string) bool {
	return routing != nil && routing.ownerAddress != "" && routing.ownerAddress == currentAddress
}

func (s *GameSession) sendTransferAborted(mapID uint32, reason uint8) {
	if s == nil || s.gameSocket == nil {
		return
	}
	s.gameSocket.SendPacket(packet.NewWriterWithSize(packet.SMsgTransferAborted, 5).Uint32(mapID).Uint8(reason).ToPacket())
}

func mapTransferLoginPlayerGUID(playerGUID uint64, routing *mapTransferRouting) uint64 {
	if mapTransferRoutingUsesCrossrealmOwner(routing) {
		return guid.PlayerGUIDForRealm(0, root.RealmID, playerGUID)
	}
	return playerGUID
}

func (s *GameSession) setPendingMapTransferRouting(routing *mapTransferRouting) {
	s.pendingMapTransferRouting = cloneMapTransferRouting(routing)
}

func (s *GameSession) clearPendingMapTransferRouting() {
	s.pendingMapTransferRouting = nil
}

func (s *GameSession) activatePendingMapTransferRouting() {
	s.activeMapTransferRouting = s.pendingMapTransferRouting
	s.pendingMapTransferRouting = nil
}

func (s *GameSession) clearActiveMapTransferRouting() {
	s.activeMapTransferRouting = nil
}

func (s *GameSession) setCurrentMapTransferRouting(routing *mapTransferRouting) {
	s.currentMapTransferRouting = cloneMapTransferRouting(routing)
	s.clearPendingMapTransferRouting()
}

func (s *GameSession) startClusterNativeWorldportTransport(ctx context.Context, transport clusterNativeWorldportTransport) error {
	if s == nil {
		return fmt.Errorf("can't start %s: session is nil", transport.operation)
	}
	if transport.start == nil {
		return fmt.Errorf("can't start %s for player %d: native transport start function is nil", transport.operation, transport.playerGUID)
	}

	routing := cloneMapTransferRouting(transport.routing)
	if routing != nil && routing.feature == clusterTransferFeatureGeneric {
		routing.feature = transport.feature
	}
	s.setPendingMapTransferRouting(routing)

	if err := transport.start(ctx); err != nil {
		s.clearPendingMapTransferRouting()
		return fmt.Errorf("%s failed for player %d: %w", transport.operation, transport.playerGUID, err)
	}

	if transport.reloadManagedGroupAfterTransfer && s.character != nil {
		s.character.GroupMangedByGameServer = true
	}

	return nil
}

func (s *GameSession) startClusterOwnerNativeWorldportTransport(ctx context.Context, transport clusterOwnerNativeWorldportTransport) error {
	if s == nil {
		return fmt.Errorf("can't start %s: session is nil", transport.operation)
	}
	if transport.sessionPlayerGUID == 0 {
		return fmt.Errorf("can't start %s: session player guid is empty", transport.operation)
	}
	if transport.loginPlayerGUID == 0 {
		transport.loginPlayerGUID = transport.sessionPlayerGUID
	}

	redirectedToOwner := false
	if transport.targetAddress != "" {
		if s.worldSocket == nil {
			return fmt.Errorf("can't start %s for player %d: world socket is nil", transport.operation, transport.sessionPlayerGUID)
		}
		if s.worldSocket.Address() != transport.targetAddress {
			if err := s.redirectPlayerToGameServerAddressForTransfer(ctx, transport.feature, transport.loginPlayerGUID, transport.targetAddress, transport.targetWorldserverID); err != nil {
				s.clearPendingMapTransferRouting()
				return fmt.Errorf("%s redirect to worldserver %q failed for player %d: %w", transport.operation, transport.targetWorldserverID, transport.sessionPlayerGUID, err)
			}
			redirectedToOwner = true
		} else if transport.targetWorldserverID != "" {
			s.worldserverID = transport.targetWorldserverID
		}
	} else if transport.targetWorldserverID != "" {
		s.worldserverID = transport.targetWorldserverID
	}

	routing := cloneMapTransferRouting(transport.routing)
	if routing != nil && routing.feature == clusterTransferFeatureGeneric {
		routing.feature = transport.feature
	}
	if routing != nil && transport.targetAddress != "" {
		routing.ownerAddress = transport.targetAddress
	}
	s.armClusterOwnerNativeWorldport(routing)
	if s.logger != nil && routing != nil {
		s.logger.Debug().
			Uint64("character", transport.sessionPlayerGUID).
			Uint32("realmID", routing.realmID).
			Bool("isCrossrealm", routing.isCrossRealm).
			Str("feature", routing.feature.String()).
			Str("ownerAddress", routing.ownerAddress).
			Msg("TC9 armed owner native worldport routing")
	}

	if transport.onOwnerPlaced != nil {
		if err := transport.onOwnerPlaced(ctx); err != nil {
			return fmt.Errorf("%s owner placement hook failed for player %d: %w", transport.operation, transport.sessionPlayerGUID, err)
		}
	}
	if transport.reloadManagedGroupAfterTransfer && s.character != nil {
		s.character.GroupMangedByGameServer = true
	}

	if transport.forwardAfterPlacement || redirectedToOwner {
		transport.forwardOptions.feature = transport.feature
		return s.forwardNextNativeWorldport(ctx, transport.forwardOptions)
	}
	return nil
}

func (s *GameSession) armClusterOwnerNativeWorldport(routing *mapTransferRouting) {
	s.setCurrentMapTransferRouting(routing)
	s.setPendingMapTransferRouting(routing)
}

func (s *GameSession) clearPendingRedirectState() {
	s.pendingRedirectID = ""
	s.pendingRedirectAt = time.Time{}
}

func (s *GameSession) forwardNextNativeWorldport(ctx context.Context, options nativeWorldportForwardOptions) error {
	sentTransferPending := false
	return s.processWorldPacketsInPlace(ctx, func(p *packet.Packet) (bool, error) {
		switch p.Opcode {
		case packet.SMsgTransferPending:
			s.clearPendingRedirectState()
			sentTransferPending = true
			s.gameSocket.SendPacket(p)
			return false, nil
		case packet.SMsgNewWorld:
			s.clearPendingRedirectState()
			mapID := p.Reader().Uint32()
			if !nativeWorldportAcceptsMapID(options, mapID) {
				if s.logger != nil {
					s.logger.Debug().
						Uint16("opcode", uint16(p.Opcode)).
						Uint32("account", s.accountID).
						Uint32("map", mapID).
						Uint32("expectedMap", options.expectedMapID).
						Interface("acceptedMaps", options.acceptedMapIDs).
						Str("feature", options.feature.String()).
						Msg("TC9 dropping intermediate native worldport for unexpected map")
				}
				return false, nil
			}
			if options.synthesizeTransferPendingForNewMap && !sentTransferPending && s.character != nil && mapID != s.character.Map {
				resp := packet.NewWriterWithSize(packet.SMsgTransferPending, 0)
				resp.Uint32(mapID)
				s.gameSocket.Send(resp)
			}
			return true, s.forwardNewWorldPacket(ctx, p, mapID)
		case packet.SMsgTransferAborted, packet.SMsgLFGTeleportDenied:
			s.clearPendingRedirectState()
			s.clearPendingMapTransferRouting()
			if s.teleportingToNewMap == nil {
				s.clearActiveMapTransferRouting()
			}
			s.gameSocket.SendPacket(p)
			return true, nil
		default:
			if s.logger != nil {
				s.logger.Debug().
					Uint16("opcode", uint16(p.Opcode)).
					Uint32("account", s.accountID).
					Str("feature", options.feature.String()).
					Msg("TC9 dropping intermediate native worldport packet")
			}
			return false, nil
		}
	})
}

func nativeWorldportAcceptsMapID(options nativeWorldportForwardOptions, mapID uint32) bool {
	if options.expectedMapID == 0 && len(options.acceptedMapIDs) == 0 {
		return true
	}
	if options.expectedMapID != 0 && mapID == options.expectedMapID {
		return true
	}
	for _, acceptedMapID := range options.acceptedMapIDs {
		if mapID == acceptedMapID {
			return true
		}
	}
	return false
}

func (s *GameSession) redirectPlayerToGameServerAddress(ctx context.Context, playerGuid uint64, desiredGameServerAddress string, desiredWorldserverID string) error {
	return s.redirectPlayerToGameServerAddressForTransfer(ctx, clusterTransferFeatureGeneric, playerGuid, desiredGameServerAddress, desiredWorldserverID)
}

func (s *GameSession) redirectPlayerToGameServerAddressForTransfer(ctx context.Context, feature clusterTransferFeature, playerGuid uint64, desiredGameServerAddress string, desiredWorldserverID string) error {
	oldServerAddress := s.worldSocket.Address()
	redirectID := fmt.Sprintf("%d-%d-%d", s.accountID, playerGuid, time.Now().UnixNano())
	redirectStarted := time.Now()
	s.pendingRedirectID = redirectID
	s.pendingRedirectAt = redirectStarted
	clearPendingRedirect := func() {
		if s.pendingRedirectID == redirectID {
			s.pendingRedirectID = ""
			s.pendingRedirectAt = time.Time{}
		}
	}

	saveAndClosePacket := clusterTransferPrepareRedirectPacket(feature)
	s.worldSocket.Send(saveAndClosePacket)

	confirmationContext, cancel := context.WithTimeout(ctx, s.packetProcessTimeout)
	defer cancel()
	if err := s.waitForSourceRedirectReady(confirmationContext, s.worldSocket, redirectID, playerGuid, 0, oldServerAddress, desiredGameServerAddress); err != nil {
		clearPendingRedirect()
		return fmt.Errorf("failed to redirect player with account %d: %w", s.accountID, err)
	}

	s.worldSocket.Close()
	s.worldSocket = nil

	newSocket, err := s.connectToGameServerWithAddressRetry(ctx, playerGuid, desiredGameServerAddress, nil)
	if err != nil {
		clearPendingRedirect()
		return fmt.Errorf("connectToGameServerWithAddress failed: %w, address: %s", err, desiredGameServerAddress)
	}

	if err := s.waitForTargetWorldLoginVerify(ctx, newSocket, redirectID, playerGuid, desiredGameServerAddress); err != nil {
		clearPendingRedirect()
		newSocket.Close()
		return err
	}

	s.worldSocket = newSocket
	if desiredWorldserverID != "" {
		s.worldserverID = desiredWorldserverID
	} else {
		s.worldserverID = s.canonicalWorldserverIDForAddress(ctx, desiredGameServerAddress)
	}

	if s.showGameserverConnChangeToClient {
		s.SendSysMessage(fmt.Sprintf("You have been redirected from %s to %s gameserver.", oldServerAddress, desiredGameServerAddress))
	}

	return nil
}

func (s *GameSession) waitForTargetWorldLoginVerify(ctx context.Context, socket sockets.Socket, redirectID string, playerGUID uint64, targetAddress string) error {
	for {
		select {
		case p, open := <-socket.ReadChannel():
			if !open {
				return fmt.Errorf("target world socket closed before login verify, redirect %s, account %d, character %d, target %s", redirectID, s.accountID, playerGUID, targetAddress)
			}

			switch p.Opcode {
			case packet.SMsgLoginVerifyWorld:
				s.observeWorldLoginVerify(p)
				return nil
			case packet.SMsgCharacterLoginFailed:
				status := uint8(packet.LoginErrorCodeLoginFailed)
				if p.Size > 0 {
					reader := p.Reader()
					status = reader.Uint8()
					if err := reader.Error(); err != nil {
						return fmt.Errorf("target worldserver sent malformed login failed packet, redirect %s, account %d, character %d, target %s: %w", redirectID, s.accountID, playerGUID, targetAddress, err)
					}
				}
				return fmt.Errorf("target worldserver rejected login with status %d, redirect %s, account %d, character %d, target %s", status, redirectID, s.accountID, playerGUID, targetAddress)
			default:
				if s.logger != nil {
					s.logger.Debug().
						Str("redirect", redirectID).
						Uint32("account", s.accountID).
						Uint64("character", playerGUID).
						Uint16("opcode", uint16(p.Opcode)).
						Str("target", targetAddress).
						Msg("Dropping target world packet while waiting for login verify")
				}
			}
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for target login verify, redirect %s, account %d, character %d, target %s: %w", redirectID, s.accountID, playerGUID, targetAddress, ctx.Err())
		}
	}
}
