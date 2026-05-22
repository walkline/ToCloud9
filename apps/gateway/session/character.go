package session

import (
	"context"
	"fmt"
	"time"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

const (
	charDeleteSuccess            = uint8(0x47)
	charDeleteFailed             = uint8(0x48)
	charDeleteFailedArenaCaptain = uint8(0x4B)
)

func (s *GameSession) CharactersList(ctx context.Context, p *packet.Packet) error {
	if s.character != nil {
		s.onLoggedOut()
	}

	if s.worldSocket != nil {
		socket := s.worldSocket
		s.worldSocket = nil
		socket.Close()
	}

	r, err := s.charServiceClient.CharactersToLoginForAccount(ctx, &pbChar.CharactersToLoginForAccountRequest{
		Api:       root.SupportedCharServiceVer,
		AccountID: s.accountID,
		RealmID:   root.RealmID,
	})
	if err != nil {
		return err
	}

	characters := make([]*pbChar.LogInCharacter, 0, len(r.Characters))
	for _, character := range r.Characters {
		if character.AccountID != s.accountID {
			s.logger.Error().
				Uint32("sessionAccount", s.accountID).
				Uint32("characterAccount", character.AccountID).
				Uint64("character", character.GUID).
				Msg("Blocked cross-account character from character list")
			continue
		}

		characters = append(characters, character)
	}

	resp := packet.NewWriterWithSize(packet.SMsgCharEnum, 0)
	resp.Uint8(uint8(len(characters)))
	for _, character := range characters {
		resp.Uint64(character.GUID)
		resp.String(character.Name)
		resp.Uint8(uint8(character.Race))
		resp.Uint8(uint8(character.Class))
		resp.Uint8(uint8(character.Gender))

		resp.Uint8(uint8(character.Skin))
		resp.Uint8(uint8(character.Face))
		resp.Uint8(uint8(character.HairStyle))
		resp.Uint8(uint8(character.HairColor))
		resp.Uint8(uint8(character.FacialStyle))

		resp.Uint8(uint8(character.Level))
		resp.Uint32(character.Zone)
		resp.Uint32(character.Map)

		resp.Float32(character.PositionX)
		resp.Float32(character.PositionY)
		resp.Float32(character.PositionZ)

		resp.Uint32(character.GuildID)

		// TODO: provide correct value
		resp.Uint32(33554432) // character flags

		resp.Uint32(0) // CHAR_CUSTOMIZE_FLAG_NONE

		// TODO: provide correct value
		resp.Uint8(0) // First login

		resp.Uint32(character.PetModelID)
		resp.Uint32(character.PetLevel)
		resp.Uint32(0) // petFamily

		for _, equipment := range character.Equipments {
			resp.Uint32(equipment.DisplayInfoID)
			resp.Uint8(uint8(equipment.InventoryType))
			resp.Uint32(equipment.EnchantmentID)
		}
	}

	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) CreateCharacter(ctx context.Context, p *packet.Packet) error {
	sendCreateFailed := func() {
		const createFailedCode = uint8(0x31)
		resp := packet.NewWriterWithSize(packet.SMsgCharCreate, 1)
		resp.Uint8(createFailedCode)
		s.gameSocket.Send(resp)
	}

	resp, err := s.sendCharacterMutationToWorld(ctx, p, packet.SMsgCharCreate)
	if err != nil {
		sendCreateFailed()
		return err
	}

	s.gameSocket.SendPacket(resp)
	return nil
}

func (s *GameSession) DeleteCharacter(ctx context.Context, p *packet.Packet) error {
	sendDelFailed := func(code uint8) {
		resp := packet.NewWriterWithSize(packet.SMsgCharDelete, 1)
		resp.Uint8(code)
		s.gameSocket.Send(resp)
	}

	deleteGUID, parseErr := characterDeleteGUID(p)
	if s.charServiceClient != nil {
		if parseErr != nil || deleteGUID == 0 {
			sendDelFailed(charDeleteFailed)
			if parseErr != nil {
				return parseErr
			}
			return fmt.Errorf("character delete packet has empty guid")
		}

		validateResp, err := s.charServiceClient.ValidateArenaTeamCharacterDelete(ctx, &pbChar.ValidateArenaTeamCharacterDeleteRequest{
			Api:        root.SupportedCharServiceVer,
			RealmID:    root.RealmID,
			PlayerGUID: deleteGUID,
		})
		if err != nil {
			sendDelFailed(charDeleteFailed)
			return err
		}
		if validateResp == nil {
			sendDelFailed(charDeleteFailed)
			return nil
		}
		switch validateResp.GetStatus() {
		case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK:
		case pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_LEADER_LEAVE:
			sendDelFailed(charDeleteFailedArenaCaptain)
			return nil
		default:
			sendDelFailed(charDeleteFailed)
			return nil
		}
	}

	resp, err := s.sendCharacterMutationToWorld(ctx, p, packet.SMsgCharDelete)
	if err != nil {
		sendDelFailed(charDeleteFailed)
		return err
	}

	if s.charServiceClient != nil && characterDeleteSucceeded(resp) {
		cleanupResp, cleanupErr := s.charServiceClient.RemovePlayerFromArenaTeams(ctx, &pbChar.RemovePlayerFromArenaTeamsRequest{
			Api:        root.SupportedCharServiceVer,
			RealmID:    root.RealmID,
			PlayerGUID: deleteGUID,
		})
		if cleanupErr != nil {
			s.logger.Error().Err(cleanupErr).Uint64("playerGUID", deleteGUID).Msg("Failed to remove deleted character from arena teams")
		} else if cleanupResp == nil {
			s.logger.Error().Uint64("playerGUID", deleteGUID).Msg("Charserver returned nil deleted character arena team cleanup response")
		} else if cleanupResp.GetStatus() != pbChar.ArenaTeamMutationStatus_ARENA_TEAM_MUTATION_OK {
			s.logger.Error().Uint64("playerGUID", deleteGUID).Stringer("status", cleanupResp.GetStatus()).Msg("Charserver rejected deleted character arena team cleanup")
		}
	}

	s.gameSocket.SendPacket(resp)
	return nil
}

func characterDeleteGUID(p *packet.Packet) (uint64, error) {
	if p == nil {
		return 0, fmt.Errorf("nil character delete packet")
	}
	reader := p.Reader()
	guid := reader.Uint64()
	return guid, reader.Error()
}

func characterDeleteSucceeded(p *packet.Packet) bool {
	if p == nil || p.Opcode != packet.SMsgCharDelete {
		return false
	}
	reader := p.Reader()
	code := reader.Uint8()
	return reader.Error() == nil && code == charDeleteSuccess
}

func (s *GameSession) sendCharacterMutationToWorld(ctx context.Context, p *packet.Packet, responseOpcode packet.Opcode) (*packet.Packet, error) {
	if ctx == nil {
		ctx = s.ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		timeout := s.packetProcessTimeout
		if timeout == 0 {
			timeout = defaultPacketProcessingTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	serverResult, err := s.serversRegistryClient.RandomGameServerForRealm(ctx, &pbServ.RandomGameServerForRealmRequest{
		Api:     root.SupportedServerRegistryVer,
		RealmID: root.RealmID,
	})
	if err != nil {
		return nil, err
	}

	if serverResult.GameServer == nil {
		return nil, fmt.Errorf("no available game servers to handle 0x%X packet", uint16(p.Opcode))
	}

	socket, err := WorldSocketCreator(s.logger, serverResult.GameServer.Address)
	if err != nil {
		return nil, fmt.Errorf("can't connect to the world server, err: %w", err)
	}
	defer socket.Close()

	go socket.ListenAndProcess(s.ctx)

	authStarted := time.Now()
	socket.SendPacket(s.authPacket)
	authTimeout := s.worldAuthAttemptTimeout
	if authTimeout == 0 {
		authTimeout = worldAuthAttemptTimeout
	}
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < authTimeout {
			authTimeout = remaining
		}
	}
	authCtx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()
	if err := s.waitForWorldAuthResponse(authCtx, socket, 0, serverResult.GameServer.Address, authStarted); err != nil {
		return nil, err
	}

	worldAuthReadyDelay := s.worldAuthSessionReadyDelay
	if worldAuthReadyDelay == 0 {
		worldAuthReadyDelay = worldAuthSessionReadyDelay
	}
	if err := s.waitForWorldAuthSessionReady(ctx, socket, 0, serverResult.GameServer.Address, worldAuthReadyDelay); err != nil {
		return nil, err
	}

	socket.SendPacket(p)

	for {
		select {
		case resp, open := <-socket.ReadChannel():
			if !open {
				return nil, fmt.Errorf("world socket closed before %s response, gameserver: %s", responseOpcode.String(), serverResult.GameServer.Address)
			}
			if resp.Opcode == responseOpcode {
				return resp, nil
			}
			s.logger.Debug().
				Str("opcode", resp.Opcode.String()).
				Str("expectedOpcode", responseOpcode.String()).
				Str("gameserver", serverResult.GameServer.Address).
				Msg("Discarding internal character mutation packet from worldserver")
		case <-ctx.Done():
			return nil, fmt.Errorf("character mutation timeout waiting for %s, gameserver: %s: %w", responseOpcode.String(), serverResult.GameServer.Address, ctx.Err())
		}
	}
}
