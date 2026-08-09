package session

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

func (s *GameSession) CharactersList(ctx context.Context, p *packet.Packet) error {
	if s.worldSocket != nil {
		socket := s.worldSocket
		s.worldSocket = nil
		socket.Close()
	}

	if s.character != nil {
		s.onLoggedOut()
	}

	r, err := s.charServiceClient.CharactersToLoginForAccount(ctx, &pbChar.CharactersToLoginForAccountRequest{
		Api:       root.SupportedCharServiceVer,
		AccountID: s.accountID,
		RealmID:   root.RealmID,
	})
	if err != nil {
		return err
	}

	resp := packet.NewWriterWithSize(packet.SMsgCharEnum, 0)
	resp.Uint8(uint8(len(r.Characters)))
	for _, character := range r.Characters {
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

	serverResult, err := s.serversRegistryClient.RandomGameServerForRealm(ctx, &pbServ.RandomGameServerForRealmRequest{
		Api:     root.SupportedServerRegistryVer,
		RealmID: root.RealmID,
	})
	if err != nil {
		sendCreateFailed()
		return err
	}

	if serverResult.GameServer == nil {
		sendCreateFailed()
		return fmt.Errorf("no available game servers to handle 0x%X packet", uint16(p.Opcode))
	}

	socket, err := WorldSocketCreator(s.logger, serverResult.GameServer.Address)
	if err != nil {
		sendCreateFailed()
		return fmt.Errorf("can't connect to the world server, err: %w", err)
	}

	go socket.ListenAndProcess(s.ctx)
	newCtx, cancel := context.WithTimeout(s.ctx, time.Second*20)
	defer cancel()

	waitDone := make(chan struct{})
	go func() {
		defer func() { waitDone <- struct{}{} }()

		for {
			select {
			case p, open := <-socket.ReadChannel():
				if !open {
					return
				}
				s.gameSocket.WriteChannel() <- p
				if p.Opcode == packet.SMsgCharCreate {
					socket.Close()
					return
				}

			case <-newCtx.Done():
				if s.worldSocket != nil {
					s.worldSocket.Close()
				}
				return
			}
		}
	}()

	socket.SendPacket(s.authPacket)

	// we need to give some time to add session on the world side
	time.Sleep(time.Millisecond * 300)

	socket.SendPacket(p)

	<-waitDone

	select {
	case <-newCtx.Done():
		sendCreateFailed()
		return fmt.Errorf("character creation timeouted, gameserver: %s", serverResult.GameServer.Address)
	default:
	}

	return nil
}

// Character delete result codes (3.3.5 ResponseCodes).
const (
	charDeleteFailed            = uint8(0x48) // CHAR_DELETE_FAILED
	charDeleteFailedGuildLeader = uint8(0x4A) // CHAR_DELETE_FAILED_GUILD_LEADER
)

func (s *GameSession) DeleteCharacter(ctx context.Context, p *packet.Packet) error {
	sendDelResult := func(code uint8) {
		resp := packet.NewWriterWithSize(packet.SMsgCharDelete, 1)
		resp.Uint8(code)
		s.gameSocket.Send(resp)
	}

	charGUID := p.Reader().Uint64()

	// Gateway owns guild leader check + membership cleanup (world may lack sGuildMgr).
	_, err := s.guildServiceClient.Leave(ctx, &pbGuild.LeaveParams{
		Api:     root.Ver,
		RealmID: root.RealmID,
		Leaver:  charGUID,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.FailedPrecondition:
			// Guild master cannot be deleted while still leading the guild.
			sendDelResult(charDeleteFailedGuildLeader)
			return nil
		case codes.NotFound:
			// Character is not in a guild — continue with world delete.
		default:
			sendDelResult(charDeleteFailed)
			return fmt.Errorf("guild cleanup before character delete failed for guid %d: %w", charGUID, err)
		}
	}

	serverResult, err := s.serversRegistryClient.RandomGameServerForRealm(ctx, &pbServ.RandomGameServerForRealmRequest{
		Api:     root.SupportedServerRegistryVer,
		RealmID: root.RealmID,
	})
	if err != nil {
		sendDelResult(charDeleteFailed)
		return err
	}

	if serverResult.GameServer == nil {
		sendDelResult(charDeleteFailed)
		return fmt.Errorf("no available game servers to handle 0x%X packet", uint16(p.Opcode))
	}

	socket, err := WorldSocketCreator(s.logger, serverResult.GameServer.Address)
	if err != nil {
		sendDelResult(charDeleteFailed)
		return fmt.Errorf("can't connect to the world server, err: %w", err)
	}

	go socket.ListenAndProcess(s.ctx)
	newCtx, cancel := context.WithTimeout(s.ctx, time.Second*20)
	defer cancel()

	waitDone := make(chan struct{})
	go func() {
		defer func() { waitDone <- struct{}{} }()

		for {
			select {
			case p, open := <-socket.ReadChannel():
				if !open {
					return
				}
				s.gameSocket.WriteChannel() <- p
				if p.Opcode == packet.SMsgCharDelete {
					socket.Close()
					return
				}

			case <-newCtx.Done():
				if s.worldSocket != nil {
					s.worldSocket.Close()
				}
				return
			}
		}
	}()

	socket.SendPacket(s.authPacket)

	// we need to give some time to add session on the world side
	time.Sleep(time.Millisecond * 300)

	socket.SendPacket(p)

	<-waitDone

	select {
	case <-newCtx.Done():
		sendDelResult(charDeleteFailed)
		return fmt.Errorf("character deletion timeouted, gameserver: %s", serverResult.GameServer.Address)
	default:
	}

	// Let's wait some moment because delete command may take some time on worldserver side.
	time.Sleep(time.Second * 1)

	return nil
}

// HandleNameQuery answers player name queries at the gateway, but only while
// the player is entering the world (login or redirect). During that window
// the game server drops STATUS_LOGGEDIN opcodes, so name queries triggered by
// group packets sent during the login sequence would get no response at all,
// leaving permanent "Unknown" frames. Outside of the window the query is
// forwarded to the game server as usual.
func (s *GameSession) HandleNameQuery(ctx context.Context, p *packet.Packet) error {
	if !s.worldEntryPending {
		if s.worldSocket != nil {
			s.worldSocket.SendPacket(p)
		}
		return nil
	}

	charGUID := p.Reader().Uint64()

	data, err := s.lookupCharacterNameData(ctx, charGUID)
	if err != nil || data == nil {
		// Unknown character or characters service unavailable, let the game
		// server answer once the player is in world.
		if s.worldSocket != nil {
			s.worldSocket.SendPacket(p)
		}
		return err
	}

	s.gameSocket.Send(s.buildNameQueryResponse(ctx, charGUID, data))

	return nil
}
