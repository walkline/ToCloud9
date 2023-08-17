package session

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	ebroadMock "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster/mocks"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/sockets"
	mocks "github.com/walkline/ToCloud9/apps/game-load-balancer/sockets/socketmock"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	charMocks "github.com/walkline/ToCloud9/gen/characters/pb/mocks"
	pbMail "github.com/walkline/ToCloud9/gen/mail/pb"
	mailMocks "github.com/walkline/ToCloud9/gen/mail/pb/mocks"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	regMock "github.com/walkline/ToCloud9/gen/servers-registry/pb/mocks"
	"github.com/walkline/ToCloud9/gen/worldserver/pb"
	wsMocks "github.com/walkline/ToCloud9/gen/worldserver/pb/mocks"
	"github.com/walkline/ToCloud9/shared/events"
	lbProducerMock "github.com/walkline/ToCloud9/shared/events/mocks"
)

func TestGameSessionHandlePacketsWorldPacketsRoute(t *testing.T) {
	worldReadChan := make(chan *packet.Packet)
	gameWriteChan := make(chan *packet.Packet, 100)

	worldSocket := &mocks.Socket{}
	worldSocket.On("ReadChannel").Return((<-chan *packet.Packet)(worldReadChan))

	gameSocket := &mocks.Socket{}
	gameSocket.On("WriteChannel").Return((chan<- *packet.Packet)(gameWriteChan))
	gameSocket.On("ReadChannel").Return((<-chan *packet.Packet)(nil))
	gameSocket.On("SendPacket", mock.MatchedBy(func(p *packet.Packet) bool {
		gameWriteChan <- p
		return true
	})).Return()

	session := NewGameSession(
		context.Background(),
		&log.Logger,
		gameSocket,
		1,
		&packet.Packet{},
		GameSessionParams{},
	)

	session.worldSocket = worldSocket

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()

		worldReadChan <- packet.NewWriter(packet.SMsgNewWorld).ToPacket()
		worldReadChan <- packet.NewWriter(packet.SMsgNewWorld).ToPacket()
		worldReadChan <- packet.NewWriter(packet.SMsgNewWorld).ToPacket()
		worldReadChan <- packet.NewWriter(packet.SMsgNewWorld).ToPacket()
	}()

	session.HandlePackets(ctx)

	assert.Len(t, gameWriteChan, 4)
	for len(gameWriteChan) > 0 {
		v := <-gameWriteChan
		assert.Equal(t, v.Opcode, packet.SMsgNewWorld)
		assert.Equal(t, v.Size, uint32(0))
	}
}

func TestGameSessionHandlePacketsGamePacketsRoute(t *testing.T) {
	gameReadChan := make(chan *packet.Packet)
	worldWriteChan := make(chan *packet.Packet, 100)

	gameSocket := &mocks.Socket{}
	gameSocket.On("ReadChannel").Return((<-chan *packet.Packet)(gameReadChan))

	worldSocket := &mocks.Socket{}
	worldSocket.On("WriteChannel").Return((chan<- *packet.Packet)(worldWriteChan))
	worldSocket.On("ReadChannel").Return((<-chan *packet.Packet)(nil))

	session := NewGameSession(
		context.Background(),
		&log.Logger,
		gameSocket,
		1,
		&packet.Packet{},
		GameSessionParams{},
	)

	session.worldSocket = worldSocket

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		defer close(gameReadChan)

		gameReadChan <- packet.NewWriter(packet.CMsgPing).ToPacket()
		gameReadChan <- packet.NewWriter(packet.CMsgPing).ToPacket()
		gameReadChan <- packet.NewWriter(packet.CMsgPing).ToPacket()
		gameReadChan <- packet.NewWriter(packet.CMsgPing).ToPacket()
	}()

	session.HandlePackets(ctx)

	assert.Len(t, worldWriteChan, 4)
	for len(worldWriteChan) > 0 {
		v := <-worldWriteChan
		assert.Equal(t, v.Opcode, packet.CMsgPing)
		assert.Equal(t, v.Size, uint32(0))
	}
}

func TestGameSessionHandlePacketsSafeFuncs(t *testing.T) {
	safeFuncs := make(chan func(*GameSession))

	gameSocket := &mocks.Socket{}
	gameSocket.On("ReadChannel").Return((<-chan *packet.Packet)(nil))

	session := NewGameSession(
		context.Background(),
		&log.Logger,
		gameSocket,
		1,
		&packet.Packet{},
		GameSessionParams{},
	)

	session.sessionSafeFuChan = safeFuncs

	result := []byte{}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()

		safeFuncs <- func(session *GameSession) {
			result = append(result, 1)
		}

		safeFuncs <- func(session *GameSession) {
			result = append(result, 2)
		}
	}()

	session.HandlePackets(ctx)

	assert.Equal(t, []byte{1, 2}, result)
}

func TestGameSessionHandlePacketsGamePacketsHandler(t *testing.T) {
	dumpHandleMap := dumpHandleMap(HandleMap)
	defer func() {
		HandleMap = dumpHandleMap
	}()

	gameReadChan := make(chan *packet.Packet)

	gameSocket := &mocks.Socket{}
	gameSocket.On("ReadChannel").Return((<-chan *packet.Packet)(gameReadChan))

	session := NewGameSession(
		context.Background(),
		&log.Logger,
		gameSocket,
		1,
		&packet.Packet{},
		GameSessionParams{},
	)

	pingHandled := false
	loginHandled := false

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		defer close(gameReadChan)

		HandleMap[packet.CMsgPing] = NewHandler("test1", func(s *GameSession, c context.Context, p *packet.Packet) error {
			defer func() { pingHandled = true }()

			assert.Equal(t, packet.CMsgPing, p.Opcode)
			assert.Equal(t, uint64(42), p.Reader().Uint64())

			return nil
		})

		HandleMap[packet.CMsgPlayerLogin] = NewHandler("test2", func(s *GameSession, c context.Context, p *packet.Packet) error {
			defer func() { loginHandled = true }()

			assert.Equal(t, packet.CMsgPlayerLogin, p.Opcode)

			return nil
		})

		gameReadChan <- packet.NewWriter(packet.CMsgPing).Uint64(42).ToPacket()
		gameReadChan <- packet.NewWriter(packet.CMsgPlayerLogin).ToPacket()
	}()

	session.HandlePackets(ctx)

	assert.True(t, pingHandled)
	assert.True(t, loginHandled)
}

func TestGameSessionHandlePacketsGamePacketsHandlerTimeout(t *testing.T) {
	timeoutTime := time.Millisecond * 100

	dumpedHandleMap := dumpHandleMap(HandleMap)
	defer func() {
		HandleMap = dumpedHandleMap
	}()

	gameReadChan := make(chan *packet.Packet)

	gameSocket := &mocks.Socket{}
	gameSocket.On("ReadChannel").Return((<-chan *packet.Packet)(gameReadChan))

	session := NewGameSession(
		context.Background(),
		&log.Logger,
		gameSocket,
		1,
		&packet.Packet{},
		GameSessionParams{
			PacketProcessTimeout: timeoutTime,
		},
	)

	pingHandled := false
	loginHandled := false

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		defer close(gameReadChan)

		HandleMap[packet.CMsgPing] = NewHandler("test1", func(s *GameSession, c context.Context, p *packet.Packet) error {
			defer func() { pingHandled = true }()

			<-c.Done()

			assert.Equal(t, packet.CMsgPing, p.Opcode)
			assert.Equal(t, uint64(42), p.Reader().Uint64())

			return nil
		})

		HandleMap[packet.CMsgPlayerLogin] = NewHandler("test2", func(s *GameSession, c context.Context, p *packet.Packet) error {
			defer func() { loginHandled = true }()

			assert.Equal(t, packet.CMsgPlayerLogin, p.Opcode)

			return nil
		})

		gameReadChan <- packet.NewWriter(packet.CMsgPing).Uint64(42).ToPacket()
		gameReadChan <- packet.NewWriter(packet.CMsgPlayerLogin).ToPacket()
	}()

	processingStartedTime := time.Now()
	session.HandlePackets(ctx)

	assert.True(t, pingHandled)
	assert.True(t, loginHandled)
	assert.Greater(t, time.Since(processingStartedTime), timeoutTime)
}

func TestGameSessionHandlePacketsEventsHandler(t *testing.T) {
	dumpEventsHandleMap := dumpEventsHandleMap(EventsHandleMap)
	defer func() {
		EventsHandleMap = dumpEventsHandleMap
	}()

	eventsChan := make(chan eBroadcaster.Event)

	gameSocket := &mocks.Socket{}
	gameSocket.On("ReadChannel").Return((<-chan *packet.Packet)(nil))

	session := NewGameSession(
		context.Background(),
		&log.Logger,
		gameSocket,
		1,
		&packet.Packet{},
		GameSessionParams{},
	)

	session.eventsChan = eventsChan

	eventHandled := false

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		defer close(eventsChan)

		EventsHandleMap[eBroadcaster.EventTypeIncomingWhisper] = NewEventHandler(
			"test",
			func(session *GameSession, ctx context.Context, event *eBroadcaster.Event) error {
				defer func() { eventHandled = true }()
				return nil
			},
		)

		eventsChan <- eBroadcaster.Event{
			Type:    eBroadcaster.EventTypeIncomingWhisper,
			Payload: &eBroadcaster.IncomingWhisperPayload{},
		}
	}()

	session.HandlePackets(ctx)

	assert.True(t, eventHandled)
}

func TestGameSessionLogin(t *testing.T) {
	dumpWorldSocketCreator := WorldSocketCreator
	defer func() { WorldSocketCreator = dumpWorldSocketCreator }()

	const (
		charID = uint64(42)
	)

	charMock := &charMocks.CharactersServiceClient{}
	charMock.On("CharactersToLoginByGUID", mock.Anything, mock.Anything).Return(&pbChar.CharactersToLoginByGUIDResponse{
		Character: &pbChar.LogInCharacter{GUID: charID, Map: 1},
	}, nil)

	servRegistryMock := &regMock.ServersRegistryServiceClient{}
	servRegistryMock.On("AvailableGameServersForMapAndRealm", mock.Anything, mock.Anything).Return(&pbServ.AvailableGameServersForMapAndRealmResponse{
		GameServers: []*pbServ.Server{
			{
				Address: "127.0.0.1:8000",
				RealmID: 1,
			},
		},
	}, nil)

	mailServiceMock := &mailMocks.MailServiceClient{}
	mailServiceMock.On("MailsForPlayer", mock.Anything, mock.Anything).Return(&pbMail.MailsForPlayerResponse{}, nil)

	producer := &lbProducerMock.LoadBalancerProducer{}
	producer.On("CharacterLoggedIn", mock.MatchedBy(func(p *events.LBEventCharacterLoggedInPayload) bool {
		return p.CharGUID == charID
	})).Return(nil)

	broadcaster := &ebroadMock.Broadcaster{}
	broadcaster.On("RegisterCharacter", mock.MatchedBy(func(id uint64) bool {
		return id == charID
	})).Return((<-chan eBroadcaster.Event)(make(chan eBroadcaster.Event)))

	worldSocket := &mocks.Socket{}
	worldSocket.On("SendPacket", mock.Anything).Return()
	worldSocket.On("Send", mock.MatchedBy(func(wr *packet.Writer) bool {
		return wr.Opcode == packet.CMsgAuthSession || wr.Opcode == packet.CMsgPlayerLogin || wr.Opcode == packet.MsgQueryNextMailTime
	})).Return()
	worldSocket.On("ReadChannel").Return((<-chan *packet.Packet)(make(chan *packet.Packet, 100)))
	worldSocket.On("ListenAndProcess", mock.Anything).Return(nil)

	gameSocket := &mocks.Socket{}
	gameSocket.On("SendPacket", mock.Anything).Return()
	gameSocket.On("Send", mock.MatchedBy(func(wr *packet.Writer) bool {
		return wr.Opcode == packet.MsgQueryNextMailTime
	})).Return()

	session := NewGameSession(
		context.Background(),
		&log.Logger,
		gameSocket,
		1,
		packet.NewWriter(packet.CMsgAuthSession).ToPacket(),
		GameSessionParams{
			ServersRegistryClient: servRegistryMock,
			CharServiceClient:     charMock,
			EventsProducer:        producer,
			EventsBroadcaster:     broadcaster,
			MailServiceClient:     mailServiceMock,
			GameServerGRPCConnMgr: &GameGRPCConnMgrMock{
				connToReturn: &wsMocks.WorldServerServiceClient{},
			},
		},
	)

	WorldSocketCreator = func(logger *zerolog.Logger, addr string) (sockets.Socket, error) {
		return worldSocket, nil
	}

	err := session.Login(context.TODO(), packet.NewWriter(packet.CMsgPlayerLogin).Uint64(charID).ToPacket())
	assert.Nil(t, err)
	assert.Equal(t, charID, session.character.GUID)
	assert.Equal(t, worldSocket, session.worldSocket)
}

func dumpHandleMap(m map[packet.Opcode]HandlersQueue) map[packet.Opcode]HandlersQueue {
	dumpHandleMap := map[packet.Opcode]HandlersQueue{}
	for k, v := range m {
		dumpHandleMap[k] = v
	}
	return dumpHandleMap
}

func dumpEventsHandleMap(m map[eBroadcaster.EventType]EventsHandlersQueue) map[eBroadcaster.EventType]EventsHandlersQueue {
	dumpHandleMap := map[eBroadcaster.EventType]EventsHandlersQueue{}
	for k, v := range m {
		dumpHandleMap[k] = v
	}
	return dumpHandleMap
}

type GameGRPCConnMgrMock struct {
	connToReturn pb.WorldServerServiceClient
	err          error
}

func (m GameGRPCConnMgrMock) AddAddressMapping(gameServerAddress, grpcServerAddress string) {

}

func (m GameGRPCConnMgrMock) GRPCConnByGameServerAddress(address string) (pb.WorldServerServiceClient, error) {
	return m.connToReturn, m.err
}
