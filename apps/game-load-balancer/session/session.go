package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	root "github.com/walkline/ToCloud9/apps/game-load-balancer"
	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/service"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/sockets"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/sockets/worldsocket"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
	pbGroup "github.com/walkline/ToCloud9/gen/group/pb"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	pbMail "github.com/walkline/ToCloud9/gen/mail/pb"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	pbGameServ "github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/events"
)

var (
	worldConnectErrInstanceNotFound  = errors.New("no available world instances")
	worldConnectErrCharacterNotFound = errors.New("character not found")
)

// GameSession represents session of the player, holds world and game sockets, routes and handles packets.
type GameSession struct {
	ctx    context.Context
	logger *zerolog.Logger

	gameSocket  sockets.Socket
	worldSocket sockets.Socket

	eventsChan        <-chan eBroadcaster.Event
	sessionSafeFuChan chan func(*GameSession)

	charServiceClient     pbChar.CharactersServiceClient
	serversRegistryClient pbServ.ServersRegistryServiceClient
	chatServiceClient     pbChat.ChatServiceClient
	guildServiceClient    pbGuild.GuildServiceClient
	mailServiceClient     pbMail.MailServiceClient
	groupServiceClient    pbGroup.GroupServiceClient
	gameServerGRPCClient  pbGameServ.WorldServerServiceClient
	eventsProducer        events.LoadBalancerProducer
	eventsBroadcaster     eBroadcaster.Broadcaster
	charsUpdsBarrier      *service.CharactersUpdatesBarrier
	gameServerGRPCConnMgr service.GameServerGRPCConnMgr

	groupUpdateCounter uint32

	packetProcessTimeout time.Duration

	authPacket *packet.Packet

	pingToWorldServerStarted time.Time

	accountID uint32
	character *pbChar.LogInCharacter

	teleportingToNewMap *uint32
}

type GameSessionParams struct {
	CharServiceClient     pbChar.CharactersServiceClient
	ServersRegistryClient pbServ.ServersRegistryServiceClient
	ChatServiceClient     pbChat.ChatServiceClient
	GuildsServiceClient   pbGuild.GuildServiceClient
	MailServiceClient     pbMail.MailServiceClient
	GroupServiceClient    pbGroup.GroupServiceClient
	EventsProducer        events.LoadBalancerProducer
	CharsUpdsBarrier      *service.CharactersUpdatesBarrier
	EventsBroadcaster     eBroadcaster.Broadcaster
	GameServerGRPCConnMgr service.GameServerGRPCConnMgr
	PacketProcessTimeout  time.Duration
}

func NewGameSession(
	ctx context.Context, logger *zerolog.Logger,
	gameSocket sockets.Socket, accountID uint32,
	authPacket *packet.Packet, params GameSessionParams,
) *GameSession {
	const defaultPacketProcessingTimeout = time.Second * 5
	packetProcessTimeout := params.PacketProcessTimeout
	if packetProcessTimeout == 0 {
		packetProcessTimeout = defaultPacketProcessingTimeout
	}

	s := &GameSession{
		ctx:                   ctx,
		logger:                logger,
		gameSocket:            gameSocket,
		authPacket:            authPacket,
		accountID:             accountID,
		charServiceClient:     params.CharServiceClient,
		serversRegistryClient: params.ServersRegistryClient,
		chatServiceClient:     params.ChatServiceClient,
		guildServiceClient:    params.GuildsServiceClient,
		mailServiceClient:     params.MailServiceClient,
		groupServiceClient:    params.GroupServiceClient,
		eventsProducer:        params.EventsProducer,
		eventsBroadcaster:     params.EventsBroadcaster,
		charsUpdsBarrier:      params.CharsUpdsBarrier,
		gameServerGRPCConnMgr: params.GameServerGRPCConnMgr,
		sessionSafeFuChan:     make(chan func(*GameSession), 100),
		packetProcessTimeout:  packetProcessTimeout,
	}
	return s
}

// HandlePackets handles game and world packets, as well as general events (like messages).
// Has infinite loop that can be broken with ctx or by closing gameSocket read channel.
func (s *GameSession) HandlePackets(ctx context.Context) {
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	defer s.logger.Debug().Msg("Stopped to handle packets")

	defer func() {
		if s.character != nil {
			s.onLoggedOut()
		}
	}()

	var worldReadChan <-chan *packet.Packet
	var err error
	for {
		if s.worldSocket != nil {
			worldReadChan = s.worldSocket.ReadChannel()
		} else {
			worldReadChan = nil
		}
		select {
		case f := <-s.sessionSafeFuChan:
			f(s)
		case p, ok := <-s.gameSocket.ReadChannel():
			if !ok {
				return
			}
			handler, found := HandleMap[p.Opcode]
			if !found {
				if s.worldSocket != nil {
					s.worldSocket.WriteChannel() <- p
				}
				break
			}

			pCtx, pCancel := context.WithTimeout(c, s.packetProcessTimeout)
			if err = handler.Handle(pCtx, s, p); err != nil {
				s.logger.Error().Err(err).Msgf("can't handle packet with name %s", handler.name)
				if userFriendlyErr, ok := err.(*UserFriendlyError); ok {
					if s.character != nil {
						s.SendSysMessage(userFriendlyErr.UserError)
					}
				}
			}
			pCancel()

		// worldReadChan can be nil and can be forever blocked
		case p, ok := <-worldReadChan:
			if !ok {
				s.worldSocket = nil
				s.onWorldSocketClosed()
				break
			}
			handler, found := HandleMap[p.Opcode]
			if !found {
				if s.gameSocket != nil {
					s.gameSocket.WriteChannel() <- p
				}
				break
			}

			pCtx, pCancel := context.WithTimeout(c, s.packetProcessTimeout)
			if err = handler.Handle(pCtx, s, p); err != nil {
				s.logger.Error().Err(err).Msgf("can't handle packet with name %s", handler.name)
			}
			pCancel()

		case event := <-s.eventsChan:
			handler, found := EventsHandleMap[event.Type]
			if !found {
				break
			}

			pCtx, pCancel := context.WithTimeout(c, s.packetProcessTimeout)
			if err = handler.Handle(pCtx, s, &event); err != nil {
				s.logger.Error().Err(err).Msgf("can't handle event with name %s", handler.name)
			}
			pCancel()

		case <-c.Done():
			return
		}
	}
}

func (s *GameSession) Login(ctx context.Context, p *packet.Packet) error {
	char, socket, err := s.connectToGameServer(ctx, p.Reader().Uint64(), nil)
	if err != nil {
		code := packet.LoginErrorCodeLoginFailed
		switch {
		case errors.Is(err, worldConnectErrCharacterNotFound):
			code = packet.LoginErrorCodeCharNotFound
		case errors.Is(err, worldConnectErrInstanceNotFound):
			code = packet.LoginErrorCodeNoInstanceServers
		}

		resp := packet.NewWriterWithSize(packet.SMsgCharacterLoginFailed, 1)
		resp.Uint8(uint8(code))
		s.gameSocket.Send(resp)
		return fmt.Errorf("failed to connect to game server, err: %w", err)
	}

	s.character = char
	s.worldSocket = socket

	err = s.eventsProducer.CharacterLoggedIn(&events.LBEventCharacterLoggedInPayload{
		CharGUID:    char.GUID,
		CharName:    char.Name,
		CharRace:    uint8(char.Race),
		CharClass:   uint8(char.Class),
		CharGender:  uint8(char.Gender),
		CharLevel:   uint8(char.Level),
		CharZone:    char.Zone,
		CharMap:     char.Map,
		CharPosX:    char.PositionX,
		CharPosY:    char.PositionY,
		CharPosZ:    char.PositionZ,
		CharGuildID: char.GuildID,
		AccountID:   char.AccountID,
	})
	if err != nil {
		s.logger.Err(err).Msg("can't send login event")
	}

	s.eventsChan = s.eventsBroadcaster.RegisterCharacter(char.GUID)

	if s.character.GuildID != 0 {
		return s.GuildLoginCommand(ctx)
	}

	if err = s.HandleQueryNextMailTime(ctx, p); err != nil {
		return err
	}

	if err = s.LoadGroupForPlayer(ctx); err != nil {
		return err
	}

	return err
}

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

	resp := packet.NewWriterWithSize(packet.SMsgCharEnum, 2)
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

func (s *GameSession) RealmSplit(ctx context.Context, p *packet.Packet) error {
	reader := p.Reader()
	unk := reader.Uint32()
	splitDate := "01/01/01"
	resp := packet.NewWriterWithSize(packet.SMsgRealmSplit, uint32(4+4+len(splitDate)+1))
	resp.Uint32(unk)
	resp.Uint32(0)
	resp.String(splitDate)
	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) ReadyForAccountDataTimes(ctx context.Context, p *packet.Packet) error {
	accountData, err := s.charServiceClient.AccountDataForAccount(ctx, &pbChar.AccountDataForAccountRequest{
		Api:       root.SupportedCharServiceVer,
		AccountID: s.accountID,
		RealmID:   root.RealmID,
	})
	if err != nil {
		return err
	}

	globalCacheMask := uint32(0x15)
	resp := packet.NewWriterWithSize(packet.SMsgAccountDataTimes, 4+1+4+8*4)
	resp.Uint32(uint32(time.Now().Unix()))
	resp.Uint8(1)
	resp.Uint32(globalCacheMask)
	for i := uint32(0); i < 8; i++ {
		if globalCacheMask&(uint32(1)<<i) > 0 {
			found := false
			for _, data := range accountData.AccountData {
				if data.Type == i {
					resp.Uint32(uint32(data.Time))
					found = true
					break
				}
			}
			if !found {
				resp.Uint32(0)
			}
		}
	}

	s.gameSocket.Send(resp)
	return nil
}

func (s *GameSession) HandlePing(ctx context.Context, p *packet.Packet) error {
	s.pingToWorldServerStarted = time.Now()
	if s.worldSocket != nil {
		s.worldSocket.WriteChannel() <- p
	} else {
		resp := packet.NewWriterWithSize(packet.SMsgPong, 4)
		resp.Uint32(p.Reader().Uint32())
		s.gameSocket.Send(resp)
	}

	return nil
}

func (s *GameSession) InterceptPong(ctx context.Context, p *packet.Packet) error {
	s.logger.Info().
		Uint32("account", s.accountID).
		Str("latency", time.Since(s.pingToWorldServerStarted).String()).
		Msg("Latency with world server")

	s.gameSocket.WriteChannel() <- p
	return nil
}

func (s *GameSession) connectToGameServer(ctx context.Context, characterGUID uint64, mapID *uint32) (*pbChar.LogInCharacter, sockets.Socket, error) {
	r, err := s.charServiceClient.CharactersToLoginByGUID(ctx, &pbChar.CharactersToLoginByGUIDRequest{
		Api:           root.SupportedCharServiceVer,
		CharacterGUID: characterGUID,
		RealmID:       root.RealmID,
	})

	if err != nil {
		return nil, nil, fmt.Errorf("can't get characters to login, err: %w", err)
	}

	if r.Character == nil {
		return nil, nil, fmt.Errorf("char id: %q, err: %w", characterGUID, worldConnectErrCharacterNotFound)
	}

	mapIDToLogin := r.Character.Map
	if mapID != nil {
		mapIDToLogin = *mapID
	}

	serversResult, err := s.serversRegistryClient.AvailableGameServersForMapAndRealm(s.ctx, &pbServ.AvailableGameServersForMapAndRealmRequest{
		Api:     root.SupportedCharServiceVer,
		RealmID: root.RealmID,
		MapID:   mapIDToLogin,
	})

	if err != nil {
		return nil, nil, fmt.Errorf("can't get available game servers for map, err: %w", err)
	}

	if len(serversResult.GameServers) == 0 {
		return nil, nil, fmt.Errorf("%w, mapID %v", worldConnectErrInstanceNotFound, mapIDToLogin)
	}

	s.logger.Debug().
		Str("address", serversResult.GameServers[0].Address).
		Msg("Connecting to the world server")

	s.gameServerGRPCConnMgr.AddAddressMapping(serversResult.GameServers[0].Address, serversResult.GameServers[0].GrpcAddress)

	s.gameServerGRPCClient, err = s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(serversResult.GameServers[0].Address)
	if err != nil {
		return nil, nil, fmt.Errorf("can't get game server grpc client, err: %w", err)
	}

	socket, err := WorldSocketCreator(s.logger, serversResult.GameServers[0].Address)
	if err != nil {
		return nil, nil, fmt.Errorf("can't connect to the world server, err: %w", err)
	}

	go socket.ListenAndProcess(s.ctx)

	socket.SendPacket(s.authPacket)

	// we need give some time to add session on the world side
	time.Sleep(time.Millisecond * 100)

	resp := packet.NewWriterWithSize(packet.CMsgPlayerLogin, 8)
	resp.Uint64(characterGUID)
	socket.Send(resp)
	return r.Character, socket, nil
}

func (s *GameSession) onWorldSocketClosed() {

	go func(charGUID uint64) {
		s.SendSysMessage("Lost connection with world server...")
		time.Sleep(time.Second * 2) // giving some time to recover

		s.SendSysMessage("Trying to recover...")
		time.Sleep(time.Second * 1) // giving some time to recover

		var err error
		var char *pbChar.LogInCharacter
		var socket sockets.Socket
		for i := 0; i < 3; i++ {
			char, socket, err = s.connectToGameServer(context.TODO(), charGUID, nil)
			if err != nil {
				s.logger.Error().Err(err).Msg("failed to reconnect player to the world")
			} else {
				break
			}
			time.Sleep(time.Second * 5) // giving some time to recover
		}

		if err != nil {
			s.SendSysMessage("Failed :( Returning to the characters screen.")

			time.Sleep(time.Second * 2) // giving some time for player to read the message

			resp := packet.NewWriterWithSize(packet.SMsgCharacterLoginFailed, 1)
			resp.Uint8(uint8(packet.LoginErrorCodeWorldServerIsDown))
			s.gameSocket.Send(resp)
			return
		}

		resp := packet.NewWriterWithSize(packet.SMsgNewWorld, 0)
		resp.Uint32(char.Map)
		resp.Float32(char.PositionX)
		resp.Float32(char.PositionY)
		resp.Float32(char.PositionZ)
		resp.Float32(0.0)
		s.gameSocket.Send(resp)

		s.SendSysMessage("Connection recovered! Sorry for inconvenience.")

		// we need to modify session in a safe thread (goroutine)
		s.sessionSafeFuChan <- func(session *GameSession) {
			if session.character != nil {
				session.worldSocket = socket
			}
		}
	}(s.character.GUID)
}

func (s *GameSession) onLoggedOut() {
	err := s.eventsProducer.CharacterLoggedOut(&events.LBEventCharacterLoggedOutPayload{
		CharGUID:    s.character.GUID,
		CharName:    s.character.Name,
		CharGuildID: s.character.GuildID,
		AccountID:   s.character.AccountID,
	})
	if err != nil {
		s.logger.Err(err).Msg("can't send logout event")
	}

	s.eventsBroadcaster.UnregisterCharacter(s.character.GUID)

	s.character = nil
}

var WorldSocketCreator = worldsocket.NewWorldSocketWithAddress
