package session

import (
	"context"
	"fmt"
	"time"

	root "github.com/walkline/ToCloud9/apps/game-load-balancer"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

var HandleMap = map[packet.Opcode]HandlersQueue{
	packet.CMsgCharCreate:               NewHandler("CMsgCharCreate", ForwardPacketToRandomGameServer(packet.SMsgCharCreate)),
	packet.CMsgPlayerLogin:              NewHandler("CMsgPlayerLogin", (*GameSession).Login),
	packet.CMsgCharDelete:               NewHandler("CMsgCharDelete", ForwardPacketToRandomGameServer(packet.SMsgCharDelete)),
	packet.CMsgCharEnum:                 NewHandler("CMsgCharEnum", (*GameSession).CharactersList),
	packet.CMsgRealmSplit:               NewHandler("CMsgRealmSplit", (*GameSession).RealmSplit),
	packet.CMsgReadyForAccountDataTimes: NewHandler("CMsgReadyForAccountDataTimes", (*GameSession).ReadyForAccountDataTimes),
	packet.CMsgMessageChat:              NewHandler("CMsgMessageChat", (*GameSession).HandleChatMessage),
	packet.CMsgGuildQuery:               NewHandler("CMsgGuildQuery", (*GameSession).HandleGuildQuery),
	packet.CMsgWho:                      NewHandler("CMsgWho", (*GameSession).HandleWho),
	packet.CMsgGuildInvite:              NewHandler("CMsgGuildInvite", (*GameSession).HandleGuildInvite),
	packet.CMsgGuildInviteAccept:        NewHandler("CMsgGuildInviteAccept", (*GameSession).HandleGuildInviteAccept),
	packet.CMsgGuildRoster:              NewHandler("CMsgGuildRoster", (*GameSession).HandleGuildRoster),
	packet.CMsgGuildLeave:               NewHandler("CMsgGuildLeave", (*GameSession).HandleGuildLeave),
	packet.CMsgGuildRemove:              NewHandler("CMsgGuildRemove", (*GameSession).HandleGuildKick),
	packet.CMsgGuildMOTD:                NewHandler("CMsgGuildSMTD", (*GameSession).HandleGuildSetMessageOfTheDay),
	packet.CMsgGuildSetPublicNote:       NewHandler("CMsgGuildSetPublicNote", (*GameSession).HandleGuildSetPublicNote),
	packet.CMsgGuildSetOfficerNote:      NewHandler("CMsgGuildSetOfficerNote", (*GameSession).HandleGuildSetOfficerNote),
	packet.CMsgGuildInfoText:            NewHandler("CMsgGuildInfoText", (*GameSession).HandleGuildSetInfoText),
	packet.CMsgGuildRank:                NewHandler("CMsgGuildRank", (*GameSession).HandleGuildRankUpdate),
	packet.CMsgGuildAddRank:             NewHandler("CMsgGuildAddRank", (*GameSession).HandleGuildRankAdd),
	packet.CMsgGuildDelRank:             NewHandler("CMsgGuildDelRank", (*GameSession).HandleGuildRankDelete),
	packet.CMsgGuildPromote:             NewHandler("CMsgGuildPromote", (*GameSession).HandleGuildPromote),
	packet.CMsgGuildDemote:              NewHandler("CMsgGuildDemote", (*GameSession).HandleGuildDemote),
	packet.CMsgSendMail:                 NewHandler("CMsgSendMail", (*GameSession).HandleSendMail),
	packet.CMsgGetMailList:              NewHandler("CMsgGetMailList", (*GameSession).HandleGetMailList),
	packet.CMsgMailMarkAsRead:           NewHandler("CMsgMailMarkAsRead", (*GameSession).HandleMailMarksAsRead),
	packet.CMsgMailTakeMoney:            NewHandler("CMsgMailTakeMoney", (*GameSession).HandleMailTakeMoney),
	packet.CMsgMailTakeItem:             NewHandler("CMsgMailTakeItem", (*GameSession).HandleMailTakeItem),
	packet.CMsgMailDelete:               NewHandler("CMsgMailDelete", (*GameSession).HandleDeleteMail),
	packet.MsgQueryNextMailTime:         NewHandler("MsgQueryNextMailTime", (*GameSession).HandleQueryNextMailTime),
	packet.SMsgInitWorldStates:          NewHandler("SMsgInitWorldStates", (*GameSession).InterceptInitWorldStates),
	packet.SMsgLevelUpInfo:              NewHandler("SMsgLevelUpInfo", (*GameSession).InterceptLevelUpInfo),
	packet.CMsgPing:                     NewHandler("CMsgPing", (*GameSession).HandlePing),
	packet.SMsgPong:                     NewHandler("SMsgPong", (*GameSession).InterceptPong),
	packet.SMsgNewWorld:                 NewHandler("SMsgNewWorld", (*GameSession).InterceptNewWorld),
	packet.MsgGuildPermissions:          NewHandler("MsgGuildPermissions", (*GameSession).HandleGuildPermissions),
	packet.MsgGuildBankMoneyWithdrawn:   NewHandler("MsgGuildBankMoneyWithdrawn", (*GameSession).HandleGuildBankMoneyWithdrawn),
	packet.MsgMoveWorldPortAck:          NewHandler("MsgMoveWorldPortAck", (*GameSession).InterceptMoveWorldPortAck),
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

func ForwardPacketToRandomGameServer(waitOpcodeToClose packet.Opcode) Handler {
	return func(s *GameSession, ctx context.Context, p *packet.Packet) error {
		serverResult, err := s.serversRegistryClient.RandomGameServerForRealm(ctx, &pbServ.RandomGameServerForRealmRequest{
			Api:     root.SupportedServerRegistryVer,
			RealmID: root.RealmID,
		})
		if err != nil {
			return err
		}

		if serverResult.GameServer == nil {
			return fmt.Errorf("no available game servers to handle 0x%X packet", uint16(p.Opcode))
		}

		socket, err := WorldSocketCreator(s.logger, serverResult.GameServer.Address)
		if err != nil {
			return fmt.Errorf("can't connect to the world server, err: %w", err)
		}

		go socket.ListenAndProcess(s.ctx)
		newCtx, cancel := context.WithTimeout(s.ctx, time.Minute)
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

		if waitOpcodeToClose != 0 {
			<-waitDone
		} else {
			socket.Close()
		}

		return nil
	}
}
