package gamesocket

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	ebroadMock "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster/mocks"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	"github.com/walkline/ToCloud9/apps/gateway/repo"
	accRepoMock "github.com/walkline/ToCloud9/apps/gateway/repo/mocks"
	"github.com/walkline/ToCloud9/apps/gateway/session"
	"github.com/walkline/ToCloud9/apps/gateway/sockets"
	"github.com/walkline/ToCloud9/apps/gateway/sockets/connmock"
	"github.com/walkline/ToCloud9/apps/gateway/sockets/worldsocket"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	charMocks "github.com/walkline/ToCloud9/gen/characters/pb/mocks"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	regMock "github.com/walkline/ToCloud9/gen/servers-registry/pb/mocks"
	gwProducerMock "github.com/walkline/ToCloud9/shared/events/mocks"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "15:04:05.000",
	}).Level(4)
}

func TestGameSocketForwardsPacketsFromGameClient(t *testing.T) {
	q := connmock.NewDataQueue(
		true,
		0,
		// ping
		WriterToBytes(packet.NewWriter(packet.CMsgPing).Int32(1)),
	)

	s := NewGameSocket(q.Mock(), nil, session.GameSessionParams{})
	assert.NoError(t, s.ListenAndProcess(context.Background()))
	readCh := s.ReadChannel()
	assert.Equal(t, 1, len(readCh))
	p := <-readCh
	assert.Equal(t, packet.CMsgPing, p.Opcode)
}

func TestGameSocketForwardsPacketsToGameClient(t *testing.T) {
	q := connmock.NewDataQueue(
		false,
		0,
	)
	m := q.Mock()

	var bytesWritten [][]byte
	bytesMutex := sync.Mutex{}
	m.OnWrite = func(b []byte) (n int, err error) {
		bytesMutex.Lock()
		defer bytesMutex.Unlock()
		bytesWritten = append(bytesWritten, b)
		return len(b), nil
	}

	s := NewGameSocket(m, nil, session.GameSessionParams{})
	go func() {
		time.Sleep(time.Millisecond * 4)
		s.Send(packet.NewWriter(packet.CMsgPing).Uint8(42))
		time.Sleep(time.Millisecond * 2)
		m.Close()
	}()
	assert.NoError(t, s.ListenAndProcess(context.Background()))

	bytesMutex.Lock()
	defer bytesMutex.Unlock()

	assert.Len(t, bytesWritten, 2) // the first msg is handshake
	assert.Equal(t, bytesWritten[1], WriterToBytes(packet.NewWriter(packet.CMsgPing).Uint8(42)))
}

// remove '_' to run benchmark
func _TestBenchmarkPlayersCount(t *testing.T) {
	useEncryption = false

	session.WorldSocketCreator = func(logger *zerolog.Logger, addr string) (sockets.Socket, error) {
		q := connmock.NewDataQueue(
			false,
			time.Millisecond*20,
			[]byte{0, 50, 221, 0, 217, 54, 189, 41, 48, 241, 0, 16, 11, 155, 68, 156, 142, 141, 197, 27, 71, 166, 65, 56, 0, 0, 0, 0, 0, 0, 0, 0, 49, 7, 0, 0, 1, 0, 0, 0, 152, 185, 154, 68, 244, 111, 141, 197, 200, 202, 165, 65},
		)

		q.SetLoopedPart(
			[]byte{3, 123, 111, 3, 2, 4, 1, 0, 6, 0, 192, 87, 1, 0, 56, 74, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 1, 0, 6, 1, 16, 33, 1, 0, 166, 64, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 105, 0, 0, 0, 1, 0, 0, 1, 3, 0, 0, 0, 2, 0, 0, 1, 3, 0, 0, 0, 4, 0, 0, 1, 3, 0, 0, 0, 6, 0, 0, 1, 3, 0, 0, 0, 8, 0, 0, 1, 3, 0, 0, 0, 10, 0, 0, 1, 3, 0, 0, 0, 12, 0, 0, 1, 3, 0, 0, 0, 14, 0, 0, 1, 3, 0, 0, 0, 16, 0, 0, 1, 3, 0, 0, 0, 18, 0, 0, 1, 3, 0, 0, 0, 20, 0, 0, 1, 3, 0, 0, 0, 22, 0, 0, 1, 3, 0, 0, 0, 24, 0, 0, 1, 3, 0, 0, 0, 26, 0, 0, 1, 3, 0, 0, 0, 28, 0, 0, 1, 3, 0, 0, 0, 30, 0, 0, 1, 3, 0, 0, 0, 32, 0, 0, 1, 3, 0, 0, 0, 34, 0, 0, 1, 3, 0, 0, 0, 36, 0, 0, 1, 3, 0, 0, 0, 38, 0, 0, 1, 3, 0, 0, 0, 40, 0, 0, 1, 3, 0, 0, 0, 136, 0, 0, 1, 3, 0, 0, 0, 137, 0, 0, 1, 3, 0, 0, 0, 140, 0, 0, 1, 3, 0, 0, 0, 146, 0, 0, 1, 3, 0, 0, 0, 163, 0, 0, 1, 3, 0, 0, 0, 164, 0, 0, 1, 3, 0, 0, 0, 165, 0, 0, 1, 3, 0, 0, 0, 170, 0, 0, 1, 254, 3, 0, 0, 171, 0, 0, 1, 254, 3, 0, 0, 203, 0, 0, 1, 2, 0, 0, 0, 206, 0, 0, 1, 2, 0, 0, 0, 207, 0, 0, 1, 2, 0, 0, 0, 208, 0, 0, 1, 2, 0, 0, 0, 209, 0, 0, 1, 2, 0, 0, 0, 214, 0, 0, 1, 2, 0, 0, 0, 216, 0, 0, 1, 2, 0, 0, 0, 220, 0, 0, 1, 2, 0, 0, 0, 245, 0, 0, 1, 6, 0, 0, 0, 251, 0, 0, 1, 2, 0, 0, 0, 253, 0, 0, 1, 2, 0, 0, 0, 255, 0, 0, 1, 2, 0, 0, 0, 16, 1, 0, 1, 3, 0, 0, 0, 17, 1, 0, 1, 3, 0, 0, 0, 18, 1, 0, 1, 3, 0, 0, 0, 20, 1, 0, 1, 3, 0, 0, 0, 29, 1, 0, 1, 2, 0, 0, 0, 30, 1, 0, 1, 2, 0, 0, 0, 31, 1, 0, 1, 2, 0, 0, 0, 32, 1, 0, 1, 2, 0, 0, 0, 46, 0, 0, 2, 2, 0, 0, 0, 159, 0, 0, 2, 2, 0, 0, 0, 223, 0, 0, 2, 2, 0, 0, 0, 224, 0, 0, 2, 2, 0, 0, 0, 227, 0, 0, 2, 2, 0, 0, 0, 237, 0, 0, 2, 2, 0, 0, 0, 238, 0, 0, 2, 2, 0, 0, 0, 239, 0, 0, 2, 2, 0, 0, 0, 240, 0, 0, 2, 2, 0, 0, 0, 243, 0, 0, 2, 2, 0, 0, 0, 244, 0, 0, 2, 2, 0, 0, 0, 246, 0, 0, 2, 2, 0, 0, 0, 247, 0, 0, 2, 2, 0, 0, 0, 248, 0, 0, 2, 2, 0, 0, 0, 250, 0, 0, 2, 2, 0, 0, 0, 1, 1, 0, 2, 2, 0, 0, 0, 23, 1, 0, 2, 2, 0, 0, 0, 24, 1, 0, 2, 2, 0, 0, 0, 37, 1, 0, 2, 2, 0, 0, 0, 38, 1, 0, 2, 2, 0, 0, 0, 178, 0, 0, 5, 1, 4, 0, 0, 179, 0, 0, 5, 1, 4, 0, 0, 180, 0, 0, 5, 1, 4, 0, 0, 181, 0, 0, 5, 1, 4, 0, 0, 182, 0, 0, 5, 254, 3, 0, 0, 183, 0, 0, 5, 254, 3, 0, 0, 184, 0, 0, 5, 1, 4, 0, 0, 185, 0, 0, 5, 1, 4, 0, 0, 186, 0, 0, 5, 1, 4, 0, 0, 187, 0, 0, 5, 1, 4, 0, 0, 188, 0, 0, 5, 1, 4, 0, 0, 189, 0, 0, 5, 1, 4, 0, 0, 190, 0, 0, 5, 1, 4, 0, 0, 191, 0, 0, 5, 1, 4, 0, 0, 192, 0, 0, 5, 1, 4, 0, 0, 201, 0, 0, 5, 254, 3, 0, 0, 205, 0, 0, 5, 2, 0, 0, 0, 210, 0, 0, 5, 2, 0, 0, 0, 211, 0, 0, 5, 2, 0, 0, 0, 212, 0, 0, 5, 2, 0, 0, 0, 213, 0, 0, 5, 2, 0, 0, 0, 215, 0, 0, 5, 2, 0, 0, 0, 217, 0, 0, 5, 2, 0, 0, 0, 219, 0, 0, 5, 2, 0, 0, 0, 221, 0, 0, 5, 2, 0, 0, 0, 226, 0, 0, 5, 2, 0, 0, 0, 241, 0, 0, 5, 2, 0, 0, 0, 242, 0, 0, 5, 2, 0, 0, 0, 249, 0, 0, 5, 6, 0, 0, 0, 252, 0, 0, 5, 2, 0, 0, 0, 254, 0, 0, 5, 2, 0, 0, 0, 0, 1, 0, 5, 2, 0, 0, 0, 2, 1, 0, 6, 3, 0, 0, 0, 3, 1, 0, 6, 3, 0, 0, 0, 6, 1, 0, 6, 2, 0, 0, 0},
			[]byte{0, 50, 221, 0, 217, 54, 189, 41, 48, 241, 0, 16, 11, 155, 68, 156, 142, 141, 197, 27, 71, 166, 65, 56, 0, 0, 0, 0, 0, 0, 0, 0, 49, 7, 0, 0, 1, 0, 0, 0, 152, 185, 154, 68, 244, 111, 141, 197, 200, 202, 165, 65},
			[]byte{0, 50, 221, 0, 217, 52, 189, 41, 48, 241, 0, 40, 240, 156, 68, 184, 139, 141, 197, 125, 93, 166, 65, 22, 6, 0, 0, 0, 0, 0, 0, 0, 238, 4, 0, 0, 1, 0, 0, 0, 120, 249, 156, 68, 156, 114, 141, 197, 143, 221, 166, 65},
		)

		m := q.Mock()

		return worldsocket.NewWorldSocketWithConnection(logger, m)
	}

	gorBefore := runtime.NumGoroutine()
	start := time.Now()
	wg := sync.WaitGroup{}
	for i := 0; i < 15000; i++ {
		wg.Add(1)
		go func() {
			simulateOnePlayer(t, 300)
			wg.Done()
		}()
	}
	wg.Wait()

	assert.Less(t, int64(time.Since(start)), int64(time.Second*25))
	time.Sleep(time.Millisecond * 300)
	fmt.Println("Goroutines delta:", runtime.NumGoroutine()-gorBefore)
}

func simulateOnePlayer(t *testing.T, packetsReceivedLimit int) {
	q := connmock.NewDataQueue(
		false,
		time.Millisecond*450,
		// auth packet
		[]byte{1, 25, 237, 1, 0, 0, 52, 48, 0, 0, 0, 0, 0, 0, 65, 68, 77, 73, 78, 0, 0, 0, 0, 0, 4, 104, 104, 255, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 132, 254, 97, 121, 63, 186, 119, 22, 250, 68, 240, 135, 149, 66, 60, 6, 251, 228, 248, 12, 158, 2, 0, 0, 120, 156, 117, 210, 193, 106, 195, 48, 12, 198, 113, 239, 41, 118, 233, 155, 236, 180, 180, 80, 194, 234, 203, 226, 158, 139, 98, 127, 75, 68, 108, 57, 56, 78, 183, 246, 61, 250, 190, 101, 183, 13, 148, 243, 79, 72, 240, 71, 175, 198, 152, 38, 242, 253, 78, 37, 92, 222, 253, 200, 184, 34, 65, 234, 185, 53, 47, 233, 123, 119, 50, 255, 188, 64, 72, 151, 213, 87, 206, 162, 90, 67, 165, 71, 89, 198, 60, 111, 112, 173, 17, 95, 140, 24, 44, 11, 39, 154, 181, 33, 150, 192, 50, 168, 11, 246, 20, 33, 129, 138, 70, 57, 245, 84, 79, 121, 216, 52, 135, 159, 170, 224, 1, 253, 58, 184, 156, 227, 162, 224, 209, 238, 71, 210, 11, 29, 109, 183, 150, 43, 110, 58, 198, 219, 60, 234, 178, 114, 12, 13, 201, 164, 106, 43, 203, 12, 175, 31, 108, 43, 82, 151, 253, 132, 186, 149, 199, 146, 47, 89, 149, 79, 226, 160, 130, 251, 45, 170, 223, 115, 156, 96, 73, 104, 128, 214, 219, 229, 9, 250, 19, 184, 66, 1, 221, 196, 49, 110, 49, 11, 202, 95, 123, 123, 28, 62, 158, 225, 147, 200, 141},
		WriterToBytes(packet.NewWriter(packet.CMsgPlayerLogin).Uint64(1)),
	)

	q.SetLoopedPart(
		[]byte{0, 12, 220, 1, 0, 0, 3, 0, 0, 0, 14, 0, 0, 0},
		[]byte{0, 4, 246, 4, 0, 0},
		[]byte{0, 12, 145, 3, 0, 0, 24, 0, 0, 0, 255, 13, 5, 0},
	)

	m := q.Mock()

	i := 0
	m.OnWrite = func(b []byte) (n int, err error) {
		i++
		if i == packetsReceivedLimit {
			m.Close()
		}
		return len(b), nil
	}

	charMock := &charMocks.CharactersServiceClient{}
	servRegistryMock := &regMock.ServersRegistryServiceClient{}
	accountRepoMock := &accRepoMock.AccountRepo{}
	broadcaster := &ebroadMock.Broadcaster{}
	producer := &gwProducerMock.GatewayProducer{}

	charMock.On("CharactersToLoginByGUID", mock.Anything, mock.Anything).Return(&pbChar.CharactersToLoginByGUIDResponse{
		Character: &pbChar.LogInCharacter{GUID: 1, Map: 1},
	}, nil)

	servRegistryMock.On("AvailableGameServersForMapAndRealm", mock.Anything, mock.Anything).Return(&pbServ.AvailableGameServersForMapAndRealmResponse{
		GameServers: []*pbServ.Server{
			{
				Address: "127.0.0.1:8000",
				RealmID: 1,
			},
		},
	}, nil)

	accountRepoMock.On("AccountByUserName", mock.Anything, mock.Anything).Return(&repo.Account{ID: 1, SessionKeyAuth: []byte{1, 1, 1, 1}}, nil)

	producer.On("CharacterLoggedOut", mock.Anything).Return(nil)
	producer.On("CharacterLoggedIn", mock.Anything).Return(nil)

	broadcaster.On("RegisterCharacter", mock.Anything).Return((<-chan eBroadcaster.Event)(make(chan eBroadcaster.Event)))
	broadcaster.On("UnregisterCharacter", mock.Anything)

	socket := NewGameSocket(m, accountRepoMock, session.GameSessionParams{
		CharServiceClient:     charMock,
		ServersRegistryClient: servRegistryMock,
		ChatServiceClient:     nil,
		EventsProducer:        producer,
		EventsBroadcaster:     broadcaster,
	})

	err := socket.ListenAndProcess(context.TODO())
	assert.Nil(t, err)
	assert.NotNil(t, socket.(*GameSocket).session)
}

func WriterToBytes(p *packet.Writer) []byte {
	header := make([]byte, 4, len(p.Payload.Bytes())+4)
	binary.BigEndian.PutUint16(header[0:2], uint16(len(p.Payload.Bytes())+2))
	binary.LittleEndian.PutUint16(header[2:4], uint16(p.Opcode))

	return append(header, p.Payload.Bytes()...)
}
