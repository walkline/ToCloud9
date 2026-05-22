package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/service"
	"github.com/walkline/ToCloud9/apps/gateway/sockets"
	"github.com/walkline/ToCloud9/apps/gateway/sockets/worldsocket"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
	pbGroup "github.com/walkline/ToCloud9/gen/group/pb"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	pbMail "github.com/walkline/ToCloud9/gen/mail/pb"
	pbMatchmaking "github.com/walkline/ToCloud9/gen/matchmaking/pb"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	pbGameServ "github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/gameserver/conn"
)

var (
	worldConnectErrInstanceNotFound  = errors.New("no available world instances")
	worldConnectErrCharacterNotFound = errors.New("character not found")
	worldConnectErrAuthTimeout       = errors.New("world auth response timeout")
	worldConnectErrAuthSocketClosed  = errors.New("world socket closed before auth response")
	worldConnectErrSocketDialFailed  = errors.New("world socket dial failed")
)

const worldAuthResponseOK = 12

var (
	worldAuthAttemptTimeout     = 5 * time.Second
	worldAuthSessionReadyDelay  = 300 * time.Millisecond
	worldserverConnectRetryWait = 200 * time.Millisecond
	worldserverConnectRetryMax  = 2 * time.Second
	slowWorldLoginVerifyAfter   = time.Second
)

const defaultPacketProcessingTimeout = 5 * time.Second

func ConfigureWorldserverConnectRetry(wait, maxWait time.Duration) {
	if wait > 0 {
		worldserverConnectRetryWait = wait
	}
	if maxWait > 0 {
		worldserverConnectRetryMax = maxWait
	}
	if worldserverConnectRetryMax > 0 && worldserverConnectRetryWait > worldserverConnectRetryMax {
		worldserverConnectRetryMax = worldserverConnectRetryWait
	}
}

// GameSession represents session of the player, holds world and game sockets, routes and handles packets.
type GameSession struct {
	ctx    context.Context
	cancel context.CancelFunc
	logger *zerolog.Logger

	gameSocket  sockets.Socket
	worldSocket sockets.Socket

	eventsChan        <-chan eBroadcaster.Event
	sessionSafeFuChan chan func(*GameSession)

	charServiceClient             pbChar.CharactersServiceClient
	serversRegistryClient         pbServ.ServersRegistryServiceClient
	chatServiceClient             pbChat.ChatServiceClient
	guildServiceClient            pbGuild.GuildServiceClient
	mailServiceClient             pbMail.MailServiceClient
	groupServiceClient            pbGroup.GroupServiceClient
	gameServerGRPCClient          pbGameServ.WorldServerServiceClient
	matchmakingServiceClient      pbMatchmaking.MatchmakingServiceClient
	eventsProducer                events.GatewayProducer
	eventsBroadcaster             eBroadcaster.Broadcaster
	chatChannelsEventsBroadcaster *eBroadcaster.ChatChannelsService
	sessionRegistry               GameSessionRegistry
	charsUpdsBarrier              *service.CharactersUpdatesBarrier
	playerStateUpdatesBarrier     *service.PlayerStateUpdatesBarrier
	realmNamesService             *service.RealmNamesService
	gameServerGRPCConnMgr         conn.GameServerGRPCConnMgr

	groupUpdateCounter uint32

	packetProcessTimeout       time.Duration
	worldAuthAttemptTimeout    time.Duration
	worldAuthSessionReadyDelay time.Duration

	authPacket *packet.Packet

	pingToWorldServerStarted time.Time

	accountID                  uint32
	character                  *LoggedInCharacter
	playerWorldActive          bool
	characterLoggedInPublished bool
	worldserverID              string
	lastLfgProposalSuccessID   uint32
	lfgDungeonActive           bool

	teleportingToNewMap       *uint32
	pendingMapTransferRouting *mapTransferRouting
	activeMapTransferRouting  *mapTransferRouting
	currentMapTransferRouting *mapTransferRouting
	pendingGuildCreate        *pendingGuildCreateState
	pendingRedirectID         string
	pendingRedirectAt         time.Time
	playerAuraState           map[uint8]service.PlayerAuraSnapshot
	observedPlayerAuraStates  map[uint64]map[uint8]service.PlayerAuraSnapshot
	renderedGroupMemberAuras  map[groupMemberAuraRenderKey]map[uint8]events.GroupMemberAuraState

	worldLoginTimingMu       sync.Mutex
	pendingWorldLoginStarted time.Time
	pendingWorldLoginGUID    uint64
	pendingWorldLoginAddress string

	packetSendingControl PacketSendingControl

	channelMembership          *ChannelMembership
	worldserverChannelBuffer   []WorldserverChannelInfo
	worldserverChannelBufferMu sync.Mutex
	worldserverChannelTimer    *time.Timer

	// showGameserverConnChangeToClient when enabled sends chat system message
	// to the player with information about connection change.
	showGameserverConnChangeToClient bool
}

type lfgDungeonTransportState struct {
	dungeonEntry uint32
	mapID        uint32
	difficulty   uint32
	routing      *mapTransferRouting
}

type worldPreLoginHook func(sockets.Socket) error

type GameSessionParams struct {
	CharServiceClient                pbChar.CharactersServiceClient
	ServersRegistryClient            pbServ.ServersRegistryServiceClient
	ChatServiceClient                pbChat.ChatServiceClient
	GuildsServiceClient              pbGuild.GuildServiceClient
	MailServiceClient                pbMail.MailServiceClient
	MatchmakingServiceClient         pbMatchmaking.MatchmakingServiceClient
	GroupServiceClient               pbGroup.GroupServiceClient
	EventsProducer                   events.GatewayProducer
	CharsUpdsBarrier                 *service.CharactersUpdatesBarrier
	RealmNamesService                *service.RealmNamesService
	EventsBroadcaster                eBroadcaster.Broadcaster
	ChatChannelsEventBroadcaster     *eBroadcaster.ChatChannelsService
	SessionRegistry                  GameSessionRegistry
	GameServerGRPCConnMgr            conn.GameServerGRPCConnMgr
	PacketProcessTimeout             time.Duration
	WorldAuthAttemptTimeout          time.Duration
	WorldAuthSessionReadyDelay       time.Duration
	ShowGameserverConnChangeToClient bool
	PlayerStateUpdatesBarrier        *service.PlayerStateUpdatesBarrier
}

func NewGameSession(
	ctx context.Context, logger *zerolog.Logger,
	gameSocket sockets.Socket, accountID uint32,
	authPacket *packet.Packet, params GameSessionParams,
) *GameSession {
	sessionCtx, sessionCancel := context.WithCancel(ctx)

	packetProcessTimeout := params.PacketProcessTimeout
	if packetProcessTimeout == 0 {
		packetProcessTimeout = defaultPacketProcessingTimeout
	}
	worldAuthReadyDelay := params.WorldAuthSessionReadyDelay
	if worldAuthReadyDelay == 0 {
		worldAuthReadyDelay = worldAuthSessionReadyDelay
	}
	worldAuthTimeout := params.WorldAuthAttemptTimeout
	if worldAuthTimeout == 0 {
		worldAuthTimeout = worldAuthAttemptTimeout
	}

	s := &GameSession{
		ctx:        sessionCtx,
		cancel:     sessionCancel,
		logger:     logger,
		gameSocket: gameSocket,
		authPacket: authPacket,
		accountID:  accountID,

		charServiceClient:                params.CharServiceClient,
		serversRegistryClient:            params.ServersRegistryClient,
		chatServiceClient:                params.ChatServiceClient,
		guildServiceClient:               params.GuildsServiceClient,
		mailServiceClient:                params.MailServiceClient,
		matchmakingServiceClient:         params.MatchmakingServiceClient,
		groupServiceClient:               params.GroupServiceClient,
		eventsProducer:                   params.EventsProducer,
		eventsBroadcaster:                params.EventsBroadcaster,
		chatChannelsEventsBroadcaster:    params.ChatChannelsEventBroadcaster,
		sessionRegistry:                  params.SessionRegistry,
		charsUpdsBarrier:                 params.CharsUpdsBarrier,
		playerStateUpdatesBarrier:        params.PlayerStateUpdatesBarrier,
		realmNamesService:                params.RealmNamesService,
		gameServerGRPCConnMgr:            params.GameServerGRPCConnMgr,
		showGameserverConnChangeToClient: params.ShowGameserverConnChangeToClient,

		sessionSafeFuChan:          make(chan func(*GameSession), 100),
		packetProcessTimeout:       packetProcessTimeout,
		worldAuthAttemptTimeout:    worldAuthTimeout,
		worldAuthSessionReadyDelay: worldAuthReadyDelay,
		channelMembership:          NewChannelMembership(0, params.ChatChannelsEventBroadcaster),
		worldserverChannelBuffer:   make([]WorldserverChannelInfo, 0),
	}
	if s.sessionRegistry != nil {
		s.sessionRegistry.RegisterAccount(s)
	}
	return s
}

// HandlePackets handles game and world packets, as well as general events (like messages).
// Has infinite loop that can be broken with ctx or by closing gameSocket read channel.
func (s *GameSession) HandlePackets(ctx context.Context) {
	c, cancel := context.WithCancel(s.ctx)
	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				cancel()
			case <-c.Done():
			}
		}()
	}
	defer cancel()
	defer s.cancelSession()
	defer s.logger.Debug().Msg("Stopped to handle packets")
	defer s.unregisterFromSessionRegistry()

	defer func() {
		if s.character != nil {
			s.onLoggedOut()
		}
	}()

	handleEvent := func(event eBroadcaster.Event) {
		handler, found := EventsHandleMap[event.Type]
		if !found {
			return
		}

		pCtx, pCancel := context.WithTimeout(c, s.packetProcessTimeout)
		defer pCancel()

		if err := handler.Handle(pCtx, s, &event); err != nil {
			s.logger.Error().Err(err).Msgf("can't handle event with name %s", handler.name)
		}
	}

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

			// Check if this opcode should be dropped (blacklisted)
			if OpcodeBlacklist[p.Opcode] {
				s.logger.Debug().Msgf("Dropped blacklisted opcode from worldserver: %d", p.Opcode)
				break
			}

			s.observeWorldLoginVerify(p)

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
			handleEvent(event)

		case event := <-s.channelMembership.GetEventsStream():
			handleEvent(event)

		case <-c.Done():
			return
		}
	}
}

func (s *GameSession) cancelSession() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *GameSession) CanRestartConnectionAuthSession() bool {
	return s != nil && s.character == nil && s.worldSocket == nil
}

func (s *GameSession) StopForConnectionAuthSession() {
	s.cancelSession()
}

func (s *GameSession) Login(ctx context.Context, p *packet.Packet) error {
	// Reset sending control for new login.
	s.packetSendingControl = PacketSendingControl{}
	s.playerWorldActive = false
	s.characterLoggedInPublished = false
	s.worldserverID = ""
	s.pendingRedirectID = ""
	s.pendingRedirectAt = time.Time{}
	s.resetPlayerAuraState()

	char, socket, worldserverID, err := s.connectToGameServer(ctx, p.Reader().Uint64(), nil, nil)
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

	s.character = &LoggedInCharacter{
		GUID:                    char.GUID,
		Name:                    char.Name,
		Race:                    uint8(char.Race),
		Class:                   uint8(char.Class),
		Gender:                  uint8(char.Gender),
		Skin:                    uint8(char.Skin),
		Face:                    uint8(char.Face),
		HairStyle:               uint8(char.HairStyle),
		HairColor:               uint8(char.HairColor),
		FacialStyle:             uint8(char.FacialStyle),
		Level:                   uint8(char.Level),
		Zone:                    char.Zone,
		Map:                     char.Map,
		PositionX:               char.PositionX,
		PositionY:               char.PositionY,
		PositionZ:               char.PositionZ,
		GuildID:                 char.GuildID,
		PlayerFlags:             char.PlayerFlags,
		AtLogin:                 char.AtLogin,
		PetEntry:                char.PetEntry,
		PetModelID:              char.PetModelID,
		PetLevel:                char.PetLevel,
		Banned:                  char.Banned,
		AccountID:               char.AccountID,
		ExtraFlags:              char.ExtraFlags,
		GroupMangedByGameServer: false,
	}
	s.worldSocket = socket
	s.worldserverID = worldserverID
	if s.sessionRegistry != nil {
		s.sessionRegistry.RegisterCharacter(s, s.character.GUID)
	}

	s.eventsChan = s.eventsBroadcaster.RegisterCharacter(char.GUID)

	if s.character.GuildID != 0 {
		if err = s.GuildLoginCommand(ctx); err != nil {
			s.logger.Err(err).Msg("can't process guild login command")
		}
	}

	if err = s.HandleQueryNextMailTime(ctx, p); err != nil {
		return err
	}

	if err = s.LoadGroupForPlayer(ctx); err != nil {
		return err
	}

	s.channelMembership = NewChannelMembership(char.GUID, s.chatChannelsEventsBroadcaster)

	return err
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

	resp := packet.NewWriterWithSize(packet.SMsgAccountDataTimes, 4+1+4+8*4)
	resp.Uint32(uint32(time.Now().Unix()))
	resp.Uint8(1)
	resp.Uint32(globalAccountDataMask)
	for i := uint32(0); i < 8; i++ {
		if globalAccountDataMask&(uint32(1)<<i) > 0 {
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

func (s *GameSession) connectToGameServer(ctx context.Context, characterGUID uint64, mapID *uint32, preLoginHook worldPreLoginHook) (*pbChar.LogInCharacter, sockets.Socket, string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.packetProcessTimeout)
		defer cancel()
	}

	charLookupStarted := time.Now()
	r, err := s.charServiceClient.CharactersToLoginByGUID(ctx, &pbChar.CharactersToLoginByGUIDRequest{
		Api:           root.SupportedCharServiceVer,
		CharacterGUID: characterGUID,
		RealmID:       root.RealmID,
		AccountID:     s.accountID,
	})

	if err != nil {
		return nil, nil, "", fmt.Errorf("can't get characters to login, err: %w", err)
	}

	if r.Character == nil {
		return nil, nil, "", fmt.Errorf("char id: %q, err: %w", characterGUID, worldConnectErrCharacterNotFound)
	}
	if r.Character.AccountID != s.accountID {
		s.logger.Error().
			Uint32("sessionAccount", s.accountID).
			Uint32("characterAccount", r.Character.AccountID).
			Uint64("character", characterGUID).
			Msg("Blocked cross-account character login")
		return nil, nil, "", fmt.Errorf("char id: %q, err: %w", characterGUID, worldConnectErrCharacterNotFound)
	}
	s.logger.Debug().
		Uint64("character", characterGUID).
		Dur("duration", time.Since(charLookupStarted)).
		Msg("Resolved character for worldserver login")

	mapIDToLogin := r.Character.Map
	if mapID != nil {
		mapIDToLogin = *mapID
	}
	persistentLfgRoute, persistentLfgBlocked, err := s.lfgPersistentRouteForMap(ctx, characterGUID, mapIDToLogin)
	if err != nil {
		return nil, nil, "", fmt.Errorf("can't resolve persisted LFG dungeon route, err: %w", err)
	}
	if persistentLfgBlocked {
		return r.Character, nil, "", fmt.Errorf("%w, persisted LFG route for mapID %v has no bound instance", worldConnectErrInstanceNotFound, mapIDToLogin)
	}

	var lastErr error
	for attempt := 1; ctx.Err() == nil; attempt++ {
		registryLookupStarted := time.Now()
		loginCharacterGUID := characterGUID
		transferRouting := (*mapTransferRouting)(nil)
		lookupRealmID := root.RealmID
		lookupCrossRealm := false
		var serversResult *pbServ.AvailableGameServersForMapAndRealmResponse
		var err error
		if persistentLfgRoute != nil {
			transferRouting = lfgRouteMapTransferRouting(persistentLfgRoute)
			loginCharacterGUID = mapTransferLoginPlayerGUID(characterGUID, transferRouting)
			lookupRealmID = persistentLfgRoute.GetOwnerRealmID()
			lookupCrossRealm = persistentLfgRoute.GetIsCrossRealm()
			serversResult, err = s.serversRegistryClient.AvailableGameServersForMapAndRealm(ctx, &pbServ.AvailableGameServersForMapAndRealmRequest{
				Api:          root.SupportedServerRegistryVer,
				RealmID:      lookupRealmID,
				MapID:        mapIDToLogin,
				IsCrossRealm: lookupCrossRealm,
			})
		} else {
			serversResult, err = s.serversRegistryClient.AvailableGameServersForMapAndRealm(ctx, &pbServ.AvailableGameServersForMapAndRealmRequest{
				Api:     root.SupportedServerRegistryVer,
				RealmID: root.RealmID,
				MapID:   mapIDToLogin,
			})
		}

		if err != nil {
			return nil, nil, "", fmt.Errorf("can't get available game servers for map, err: %w", err)
		}
		if len(serversResult.GameServers) == 0 && persistentLfgRoute == nil {
			crossrealmResult, err := s.serversRegistryClient.AvailableGameServersForMapAndRealm(ctx, &pbServ.AvailableGameServersForMapAndRealmRequest{
				Api:          root.SupportedServerRegistryVer,
				RealmID:      0,
				MapID:        mapIDToLogin,
				IsCrossRealm: true,
			})
			if err != nil {
				return nil, nil, "", fmt.Errorf("can't get available crossrealm game servers for map, err: %w", err)
			}
			if len(crossrealmResult.GetGameServers()) > 0 {
				serversResult = crossrealmResult
				transferRouting = &mapTransferRouting{realmID: 0, isCrossRealm: true}
				loginCharacterGUID = mapTransferLoginPlayerGUID(characterGUID, transferRouting)
				lookupRealmID = 0
				lookupCrossRealm = true
				s.logger.Warn().
					Uint64("character", characterGUID).
					Uint64("loginCharacter", loginCharacterGUID).
					Uint32("map", mapIDToLogin).
					Uint32("homeRealm", root.RealmID).
					Msg("Falling back to crossrealm worldserver for persisted character map")
			}
		}

		if len(serversResult.GameServers) == 0 {
			lastErr = fmt.Errorf("%w, mapID %v, realmID %d", worldConnectErrInstanceNotFound, mapIDToLogin, lookupRealmID)
			retryEvent := s.logger.Debug()
			if attempt == 1 {
				retryEvent = s.logger.Warn()
			}
			retryEvent.
				Uint64("character", characterGUID).
				Uint32("map", mapIDToLogin).
				Uint32("realm", lookupRealmID).
				Bool("crossrealm", lookupCrossRealm).
				Int("attempt", attempt).
				Dur("duration", time.Since(registryLookupStarted)).
				Msg("No worldserver available for map, retrying")
			if !waitForWorldserverConnectRetry(ctx, attempt) {
				break
			}
			continue
		}

		attemptedCandidate := false
		for candidateIndex, server := range serversResult.GameServers {
			if server == nil || server.Address == "" {
				continue
			}
			attemptedCandidate = true

			s.logger.Debug().
				Uint64("character", characterGUID).
				Uint32("map", mapIDToLogin).
				Uint32("realm", lookupRealmID).
				Bool("crossrealm", lookupCrossRealm).
				Str("address", server.Address).
				Int("candidate", candidateIndex+1).
				Int("candidates", len(serversResult.GameServers)).
				Int("attempt", attempt).
				Dur("duration", time.Since(registryLookupStarted)).
				Msg("Resolved worldserver candidate for map")

			s.gameServerGRPCConnMgr.AddAddressMapping(server.Address, server.GrpcAddress)

			gameServerGRPCClient, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(server.Address)
			if err != nil {
				return nil, nil, "", fmt.Errorf("can't get game server grpc client, err: %w", err)
			}

			socket, err := s.connectToGameServerWithAddress(ctx, loginCharacterGUID, server.Address, preLoginHook)
			if err == nil {
				s.gameServerGRPCClient = gameServerGRPCClient
				s.setCurrentMapTransferRouting(transferRouting)
				return r.Character, socket, gameServerSourceID(server), nil
			}

			lastErr = err
			if !isRetryableWorldConnectError(err) {
				return r.Character, nil, "", err
			}

			retryEvent := s.logger.Debug()
			if attempt == 1 {
				retryEvent = s.logger.Warn()
			}
			retryEvent.
				Err(err).
				Uint64("character", characterGUID).
				Uint64("loginCharacter", loginCharacterGUID).
				Uint32("map", mapIDToLogin).
				Uint32("realm", lookupRealmID).
				Bool("crossrealm", lookupCrossRealm).
				Str("address", server.Address).
				Int("candidate", candidateIndex+1).
				Int("candidates", len(serversResult.GameServers)).
				Int("attempt", attempt).
				Msg("Worldserver login candidate failed")
		}
		if !attemptedCandidate {
			lastErr = fmt.Errorf("%w, mapID %v, realmID %d", worldConnectErrInstanceNotFound, mapIDToLogin, lookupRealmID)
		}

		if !waitForWorldserverConnectRetry(ctx, attempt) {
			break
		}
	}

	if lastErr != nil {
		return r.Character, nil, "", lastErr
	}

	return r.Character, nil, "", ctx.Err()
}

func gameServerSourceID(server *pbServ.Server) string {
	if server == nil {
		return ""
	}
	if server.Id != "" {
		return server.Id
	}
	return server.Address
}

func (s *GameSession) canonicalWorldserverIDForAddress(ctx context.Context, address string) string {
	if address == "" || s.serversRegistryClient == nil {
		return address
	}

	resp, err := s.serversRegistryClient.ListAllGameServers(ctx, &pbServ.ListAllGameServersRequest{
		Api: root.SupportedServerRegistryVer,
	})
	if err != nil {
		if s.logger != nil {
			s.logger.Debug().Err(err).Str("address", address).Msg("can't resolve canonical worldserver id")
		}
		return address
	}

	for _, server := range resp.GameServers {
		if server.Address == address && server.ID != "" {
			return server.ID
		}
	}

	return address
}

func (s *GameSession) connectToGameServerWithAddress(ctx context.Context, characterGUID uint64, gameserverAddress string, preLoginHook worldPreLoginHook) (sockets.Socket, error) {
	connectStarted := time.Now()
	s.logger.Debug().
		Str("address", gameserverAddress).
		Uint64("character", characterGUID).
		Msg("Connecting to the world server")

	socket, err := WorldSocketCreator(s.logger, gameserverAddress)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", worldConnectErrSocketDialFailed, err)
	}
	s.logger.Debug().
		Str("address", gameserverAddress).
		Uint64("character", characterGUID).
		Dur("duration", time.Since(connectStarted)).
		Msg("Connected TCP socket to world server")

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
	if err := s.waitForWorldAuthResponse(authCtx, socket, characterGUID, gameserverAddress, authStarted); err != nil {
		socket.Close()
		return nil, err
	}

	worldAuthReadyDelay := s.worldAuthSessionReadyDelay
	if worldAuthReadyDelay == 0 {
		worldAuthReadyDelay = worldAuthSessionReadyDelay
	}
	if err := s.waitForWorldAuthSessionReady(ctx, socket, characterGUID, gameserverAddress, worldAuthReadyDelay); err != nil {
		socket.Close()
		return nil, err
	}

	if preLoginHook != nil {
		if err := preLoginHook(socket); err != nil {
			socket.Close()
			return nil, fmt.Errorf("pre-login hook failed for character %d on %s: %w", characterGUID, gameserverAddress, err)
		}
	}

	resp := packet.NewWriterWithSize(packet.CMsgPlayerLogin, 8)
	resp.Uint64(characterGUID)
	socket.Send(resp)
	s.markWorldLoginSent(characterGUID, gameserverAddress)
	s.logger.Debug().
		Str("address", gameserverAddress).
		Uint64("character", characterGUID).
		Dur("duration", time.Since(connectStarted)).
		Msg("Sent CMsgPlayerLogin to world server")

	return socket, nil
}

func (s *GameSession) connectToGameServerWithAddressRetry(ctx context.Context, characterGUID uint64, gameserverAddress string, preLoginHook worldPreLoginHook) (sockets.Socket, error) {
	var lastErr error
	for attempt := 1; ctx.Err() == nil; attempt++ {
		socket, err := s.connectToGameServerWithAddress(ctx, characterGUID, gameserverAddress, preLoginHook)
		if err == nil {
			return socket, nil
		}
		if !isRetryableWorldConnectError(err) {
			return nil, err
		}

		lastErr = err
		event := s.logger.Debug()
		if attempt == 1 {
			event = s.logger.Warn()
		}
		event.
			Err(err).
			Uint64("character", characterGUID).
			Str("address", gameserverAddress).
			Int("attempt", attempt).
			Msg("Worldserver redirect login attempt failed, retrying")

		if !waitForWorldserverConnectRetry(ctx, attempt) {
			break
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ctx.Err()
}

func (s *GameSession) markWorldLoginSent(characterGUID uint64, gameserverAddress string) {
	s.worldLoginTimingMu.Lock()
	defer s.worldLoginTimingMu.Unlock()

	s.pendingWorldLoginStarted = time.Now()
	s.pendingWorldLoginGUID = characterGUID
	s.pendingWorldLoginAddress = gameserverAddress
}

func (s *GameSession) observeWorldLoginVerify(p *packet.Packet) {
	if p.Opcode != packet.SMsgLoginVerifyWorld {
		return
	}

	s.markPlayerWorldActive()

	s.worldLoginTimingMu.Lock()
	started := s.pendingWorldLoginStarted
	characterGUID := s.pendingWorldLoginGUID
	address := s.pendingWorldLoginAddress
	s.pendingWorldLoginStarted = time.Time{}
	s.pendingWorldLoginGUID = 0
	s.pendingWorldLoginAddress = ""
	s.worldLoginTimingMu.Unlock()

	if started.IsZero() {
		return
	}

	duration := time.Since(started)
	event := s.logger.Debug()
	if duration >= slowWorldLoginVerifyAfter {
		event = s.logger.Warn()
	}

	event.
		Str("address", address).
		Uint64("character", characterGUID).
		Dur("duration", duration).
		Msg("Worldserver login verify reached client")
}

func (s *GameSession) waitForWorldAuthResponse(ctx context.Context, socket sockets.Socket, characterGUID uint64, gameserverAddress string, started time.Time) error {
	challengeReceived := false
	for {
		select {
		case p, open := <-socket.ReadChannel():
			if !open {
				return fmt.Errorf("%w, address: %s", worldConnectErrAuthSocketClosed, gameserverAddress)
			}

			switch p.Opcode {
			case packet.SMsgAuthChallenge:
				challengeReceived = true
				s.logger.Debug().
					Str("address", gameserverAddress).
					Uint64("character", characterGUID).
					Dur("duration", time.Since(started)).
					Msg("Received worldserver auth challenge")
			case packet.SMsgAuthResponse:
				authStatus := uint8(0)
				if p.Size > 0 {
					authStatus = p.Reader().Uint8()
					if authStatus != worldAuthResponseOK {
						return fmt.Errorf("world auth response failed with status %d, address: %s", authStatus, gameserverAddress)
					}
				}
				s.logger.Debug().
					Str("address", gameserverAddress).
					Uint64("character", characterGUID).
					Uint8("status", authStatus).
					Bool("challengeReceived", challengeReceived).
					Dur("duration", time.Since(started)).
					Msg("Received worldserver auth response")
				return nil
			default:
				return fmt.Errorf("unexpected world packet %s before auth response, address: %s", p.Opcode.String(), gameserverAddress)
			}
		case <-ctx.Done():
			return fmt.Errorf("%w from %s: %v", worldConnectErrAuthTimeout, gameserverAddress, ctx.Err())
		}
	}
}

func isRetryableWorldConnectError(err error) bool {
	return errors.Is(err, worldConnectErrInstanceNotFound) ||
		errors.Is(err, worldConnectErrAuthTimeout) ||
		errors.Is(err, worldConnectErrAuthSocketClosed) ||
		errors.Is(err, worldConnectErrSocketDialFailed)
}

func waitForWorldserverConnectRetry(ctx context.Context, attempt int) bool {
	if worldserverConnectRetryWait <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(worldserverConnectRetryDelay(attempt))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}

func worldserverConnectRetryDelay(attempt int) time.Duration {
	if worldserverConnectRetryWait <= 0 {
		return 0
	}
	if attempt < 1 {
		attempt = 1
	}

	delay := worldserverConnectRetryWait
	for i := 1; i < attempt; i++ {
		if worldserverConnectRetryMax > 0 && delay >= worldserverConnectRetryMax {
			return worldserverConnectRetryMax
		}
		delay *= 2
		if worldserverConnectRetryMax > 0 && delay > worldserverConnectRetryMax {
			return worldserverConnectRetryMax
		}
	}

	return delay
}

func (s *GameSession) waitForWorldAuthSessionReady(ctx context.Context, socket sockets.Socket, characterGUID uint64, gameserverAddress string, fallbackDelay time.Duration) error {
	if fallbackDelay <= 0 {
		if ctx.Err() != nil {
			return fmt.Errorf("%w from %s: %v", worldConnectErrAuthTimeout, gameserverAddress, ctx.Err())
		}
		return nil
	}

	timer := time.NewTimer(fallbackDelay)
	defer timer.Stop()

	for {
		select {
		case p, open := <-socket.ReadChannel():
			if !open {
				return fmt.Errorf("%w, address: %s", worldConnectErrAuthSocketClosed, gameserverAddress)
			}

			if p.Opcode != packet.TC9SMsgWorldSessionReady {
				return fmt.Errorf("unexpected world packet %s before world session ready ack, address: %s", p.Opcode.String(), gameserverAddress)
			}

			s.logger.Debug().
				Str("address", gameserverAddress).
				Uint64("character", characterGUID).
				Msg("Received TC9 world session ready ack")
			return nil
		case <-timer.C:
			s.logger.Debug().
				Str("address", gameserverAddress).
				Uint64("character", characterGUID).
				Dur("fallbackDelay", fallbackDelay).
				Msg("World session ready ack not observed before fallback delay")
			return nil
		case <-ctx.Done():
			return fmt.Errorf("%w from %s: %v", worldConnectErrAuthTimeout, gameserverAddress, ctx.Err())
		}
	}
}

func waitForWorldAuthSessionReady(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}

func (s *GameSession) processWorldPacketsInPlace(ctx context.Context, f func(*packet.Packet) (stopProcessing bool, err error)) error {
	if s.worldSocket == nil {
		return nil
	}

	for {
		select {
		case p, open := <-s.worldSocket.ReadChannel():
			if !open {
				return fmt.Errorf("world socket closed")
			}

			stop, err := f(p)
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *GameSession) onWorldSocketClosed() {
	if s.character == nil {
		return
	}
	if s.pendingRedirectID != "" {
		if s.logger != nil {
			s.logger.Debug().
				Str("redirect", s.pendingRedirectID).
				Uint32("account", s.accountID).
				Uint64("character", s.character.GUID).
				Msg("TC9 ignoring source world socket close during cross-worldserver redirect")
		}
		return
	}

	character := *s.character
	if !s.canReconnectCharacter(character.GUID) {
		return
	}

	go func(character LoggedInCharacter) {
		if !s.canReconnectCharacter(character.GUID) {
			return
		}

		s.SendSysMessage("Lost connection with world server...")
		if !s.waitForReconnectDelay(time.Second * 2) {
			return
		}

		if !s.canReconnectCharacter(character.GUID) {
			return
		}
		s.SendSysMessage("Trying to recover...")
		if !s.waitForReconnectDelay(time.Second) {
			return
		}

		var err error
		var char *pbChar.LogInCharacter
		var socket sockets.Socket
		var worldserverID string
		for i := 0; i < 3; i++ {
			if !s.canReconnectCharacter(character.GUID) {
				return
			}

			char, socket, worldserverID, err = s.connectToGameServer(s.ctx, character.GUID, nil, func(_ sockets.Socket) error {
				saveCtx, cancel := context.WithTimeout(s.ctx, s.packetProcessTimeout)
				defer cancel()
				_, err := s.charServiceClient.SavePlayerPosition(saveCtx, &pbChar.SavePlayerPositionRequest{
					Api:      root.SupportedCharServiceVer,
					RealmID:  root.RealmID,
					CharGUID: character.GUID,
					MapID:    character.Map,
					X:        character.PositionX,
					Y:        character.PositionY,
					Z:        character.PositionZ,
					O:        character.PositionO,
				})
				if err != nil {
					s.logger.Error().Err(err).Msg("can't save player position")
					return err
				}
				return nil
			})
			if err != nil {
				s.logger.Error().Err(err).Msg("failed to reconnect player to the world")
			} else {
				break
			}
			if !s.waitForReconnectDelay(time.Second * 5) {
				return
			}
		}

		if err != nil {
			if !s.canReconnectCharacter(character.GUID) {
				return
			}

			s.SendSysMessage("Failed :( Returning to the characters screen.")

			if !s.waitForReconnectDelay(time.Second * 2) {
				return
			}

			resp := packet.NewWriterWithSize(packet.SMsgCharacterLoginFailed, 1)
			resp.Uint8(uint8(packet.LoginErrorCodeWorldServerIsDown))
			s.gameSocket.Send(resp)
			return
		}
		if !s.canReconnectCharacter(character.GUID) {
			socket.Close()
			return
		}

		s.gameSocket.Send(recoveredNewWorldPacket(char, character))

		// We need to modify the session in the packet handler goroutine.
		updateSession := func(session *GameSession) {
			if !session.canReconnectCharacter(character.GUID) {
				socket.Close()
				return
			}

			if session.character != nil && session.character.GUID == character.GUID {
				session.worldSocket = socket
				session.worldserverID = worldserverID
				session.resetPlayerAuraState()
			}

			if session.showGameserverConnChangeToClient {
				session.SendSysMessage(fmt.Sprintf("Connection recovered! New gameserver: %s. Sorry for inconvenience.", socket.Address()))
			} else {
				session.SendSysMessage("Connection recovered! Sorry for inconvenience.")
			}
		}
		select {
		case s.sessionSafeFuChan <- updateSession:
		case <-s.ctx.Done():
			socket.Close()
			return
		}
	}(character)
}

func recoveredNewWorldPacket(char *pbChar.LogInCharacter, character LoggedInCharacter) *packet.Writer {
	resp := packet.NewWriterWithSize(packet.SMsgNewWorld, 0)
	resp.Uint32(char.Map)
	resp.Float32(character.PositionX)
	resp.Float32(character.PositionY)
	resp.Float32(character.PositionZ)
	resp.Float32(character.PositionO)
	return resp
}

func (s *GameSession) waitForReconnectDelay(delay time.Duration) bool {
	if delay <= 0 {
		return s.ctx == nil || s.ctx.Err() == nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return s.ctx == nil || s.ctx.Err() == nil
	case <-s.ctx.Done():
		return false
	}
}

func (s *GameSession) canReconnectCharacter(characterGUID uint64) bool {
	if s.ctx != nil && s.ctx.Err() != nil {
		return false
	}
	if s.gameSocket == nil {
		return false
	}

	return s.isActiveAccountSession() && s.isActiveCharacterSession(characterGUID)
}

func (s *GameSession) onLoggedOut() {
	if s.character == nil {
		s.playerWorldActive = false
		s.characterLoggedInPublished = false
		return
	}
	ownsCharacterSession := s.isActiveCharacterSession(s.character.GUID)
	if ownsCharacterSession && s.sessionRegistry != nil {
		s.sessionRegistry.UnregisterCharacter(s, s.character.GUID)
	}

	if ownsCharacterSession {
		s.publishOfflinePlayerStateSnapshot()
	}

	if ownsCharacterSession && s.eventsProducer != nil {
		err := s.eventsProducer.CharacterLoggedOut(&events.GWEventCharacterLoggedOutPayload{
			RealmID:     root.RealmID,
			GatewayID:   root.RetrievedGatewayID,
			CharGUID:    s.character.GUID,
			CharName:    s.character.Name,
			CharGuildID: s.character.GuildID,
			AccountID:   s.character.AccountID,
		})
		if err != nil {
			s.logger.Err(err).Msg("can't send logout event")
		}
	}

	if ownsCharacterSession && s.eventsBroadcaster != nil {
		s.eventsBroadcaster.UnregisterCharacter(s.character.GUID)
	}
	s.eventsChan = nil
	if ownsCharacterSession && s.chatChannelsEventsBroadcaster != nil {
		s.chatChannelsEventsBroadcaster.DisconnectPlayer(s.character.GUID)
	}
	if s.channelMembership != nil {
		s.channelMembership.events = nil
	}
	s.resetPlayerAuraState()

	s.character = nil
	s.playerWorldActive = false
	s.characterLoggedInPublished = false
	s.worldserverID = ""
}

func (s *GameSession) isActiveAccountSession() bool {
	if s.sessionRegistry == nil {
		return true
	}

	return s.sessionRegistry.IsAccountSession(s)
}

func (s *GameSession) isActiveCharacterSession(characterGUID uint64) bool {
	if s.sessionRegistry == nil {
		return true
	}

	return s.sessionRegistry.IsCharacterSession(s, characterGUID)
}

var WorldSocketCreator = worldsocket.NewWorldSocketWithAddress

// PacketSendingControl contains flags to track sending of some packets
// that needs to be sent only once or similar to that.
type PacketSendingControl struct {
	motdSent                    bool
	accountDataTimesGlobalSent  bool
	accountDataTimesPerCharSent bool
}

type pendingGuildCreateState struct {
	name string
}

// LoggedInCharacter represents a character that is logged in and bound to the session.
// Some values are cached values and can be not actual values from gameserver.
type LoggedInCharacter struct {
	GUID        uint64
	Name        string
	Race        uint8
	Class       uint8
	Gender      uint8
	Skin        uint8
	Face        uint8
	HairStyle   uint8
	HairColor   uint8
	FacialStyle uint8
	Level       uint8
	Zone        uint32
	Map         uint32
	PositionX   float32
	PositionY   float32
	PositionZ   float32
	PositionO   float32
	GuildID     uint32
	PlayerFlags uint32
	AtLogin     uint32
	PetEntry    uint32
	PetModelID  uint32
	PetLevel    uint32
	Banned      bool
	AccountID   uint32
	ExtraFlags  uint32

	// GroupMangedByGameServer tracks cases when player joined e.g. battleground
	// and the group is managed by game server but not group server.
	GroupMangedByGameServer bool

	ignoreNextInterceptToNewMap *uint32
	pvpQueueSlotsByType         map[uint32]uint8
	lfgPendingJoin              bool
	lfgMatchmakingActive        bool
	lastLfgStatus               events.MatchmakingLfgStatusPayload
	lfgDungeonTransport         *lfgDungeonTransportState

	// bgInviteOrderingFix handles race conditions between Invite and JoinToQueue events
	// for battleground queuing. It contains state to ensure correct event ordering:
	//   - waitingJoinToQueue: indicates if we're waiting for a JoinToQueue event
	//   - pendingInvitePacket: stores an Invite packet that arrived before JoinToQueue
	// This prevents issues where a player might receive an Invite before their
	// JoinToQueue event is processed, which results on not displaying invite on client side.
	bgInviteOrderingFix struct {
		waitingJoinToQueue  bool
		pendingInvitePacket *packet.Packet
	}
}
