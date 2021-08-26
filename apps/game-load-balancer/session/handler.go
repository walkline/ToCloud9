package session

import (
	"context"
	"fmt"
	root "github.com/walkline/ToCloud9/apps/game-load-balancer"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"time"

	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
)

var HandleMap = map[uint16]HandlersQueue{
	packet.CMsgCharCreate:               NewHandler("CMsgCharCreate", ForwardPacketToRandomGameServer(packet.SMsgCharCreate)),
	packet.CMsgPlayerLogin:              NewHandler("CMsgPlayerLogin", (*GameSession).Login),
	packet.CMsgCharDelete:               NewHandler("CMsgCharCreate", ForwardPacketToRandomGameServer(packet.SMsgCharDelete)),
	packet.CMsgCharEnum:                 NewHandler("CMsgCharEnum", (*GameSession).CharactersList),
	packet.CMsgRealmSplit:               NewHandler("CMsgRealmSplit", (*GameSession).RealmSplit),
	packet.CMsgReadyForAccountDataTimes: NewHandler("CMsgReadyForAccountDataTimes", (*GameSession).ReadyForAccountDataTimes),
	packet.CMsgMessageChat:              NewHandler("CMsgMessageChat", (*GameSession).HandleChatMessage),
}

type Handler func(*GameSession, context.Context, *packet.Packet) error

func NewHandler(name string, handlers ...Handler) HandlersQueue {
	return HandlersQueue{
		name:  name,
		queue: handlers,
	}
}

type HandlersQueue struct {
	name  string
	queue []Handler
}

func (q *HandlersQueue) Handle(ctx context.Context, session *GameSession, p *packet.Packet) error {
	var err error
	for i := range q.queue {
		err = q.queue[i](session, ctx, p)
		if err != nil {
			return err
		}
	}
	return nil
}

func ForwardPacketToRandomGameServer(waitOpcodeToClose uint16) Handler {
	return func(s *GameSession, ctx context.Context, p *packet.Packet) error {
		serverResult, err := s.serversRegistryClient.RandomGameServerForRealm(ctx, &pbServ.RandomGameServerForRealmRequest{
			Api:     root.SupportedServerRegistryVer,
			RealmID: root.RealmID,
		})
		if err != nil {
			return err
		}

		if serverResult.GameServer == nil {
			return fmt.Errorf("no available game servers to handle 0x%X packet", p.Opcode)
		}

		socket, err := WorldSocketCreator(s.logger, serverResult.GameServer.Address)
		if err != nil {
			return fmt.Errorf("can't connect to the world server, err: %w", err)
		}

		//s.worldSocket = socket
		go socket.ListenAndProcess(s.ctx)
		newCtx, cancel := context.WithTimeout(s.ctx, time.Second*10)
		waitDone := make(chan struct{})
		go func() {
			defer cancel()
			defer func() { waitDone <- struct{}{} }()

			for {
				select {
				case p, open := <-socket.ReadChannel():
					if !open {
						return
					}
					s.gameSocket.WriteChannel() <- p

					if p.Opcode == waitOpcodeToClose {
						socket.Close()
						return
					}

				case <-newCtx.Done():
					s.worldSocket.Close()
					return
				}
			}
		}()

		socket.SendPacket(s.authPacket)

		// we need to give some time to add session on the world side
		time.Sleep(time.Millisecond * 100)

		socket.SendPacket(p)
		if waitOpcodeToClose != 0 {
			<-waitDone
		} else {
			socket.Close()
		}

		return nil
	}
}
