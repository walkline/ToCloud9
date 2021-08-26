package session

import (
	"context"
	root "github.com/walkline/ToCloud9/apps/game-load-balancer"
	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
)

type ChatType uint8

const (
	ChatTypeSystem ChatType = iota
	ChatTypeSay
	ChatTypeParty
	ChatTypeRaid
	ChatTypeGuild
	ChatTypeOfficer
	ChatTypeYell
	ChatTypeWhisper
	ChatTypeWhisperForeign
	ChatTypeWhisperInform
)

func (s *GameSession) SendSysMessage(msg string) {
	resp := packet.NewWriterWithSize(packet.SMsgMessageChat, 0)
	resp.Uint8(uint8(ChatTypeSystem)) // chatType
	resp.Uint32(0)                    // language
	resp.Uint64(0)                    // sender
	resp.Uint32(0)                    // some flags
	resp.Uint64(0)                    // receiver
	resp.Uint32(uint32(len(msg) + 1))
	resp.String(msg)
	resp.Uint8(0) // chat tag
	s.gameSocket.Send(resp)
}

func (s *GameSession) HandleChatMessage(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	msgType := r.Uint32()
	lang := r.Uint32()
	to := ""
	msg := ""
	switch msgType {
	case uint32(ChatTypeWhisper):
		to = r.String()
		msg = r.String()
		res, err := s.chatServiceClient.SendWhisperMessage(ctx, &pbChat.SendWhisperMessageRequest{
			Api:          root.Ver,
			RealmID:      root.RealmID,
			SenderGUID:   s.character.GUID,
			SenderName:   s.character.Name,
			SenderRace:   s.character.Race,
			Language:     lang,
			ReceiverName: to,
			Msg:          msg,
		})

		// TODO: handle response

		if err != nil {
			return err
		}

		resp := packet.NewWriterWithSize(packet.SMsgMessageChat, 0)
		resp.Uint8(uint8(ChatTypeWhisperInform))
		resp.Uint32(lang)
		resp.Uint64(res.ReceiverGUID)
		resp.Uint32(0) // some flags
		resp.Uint64(res.ReceiverGUID)
		resp.Uint32(uint32(len(msg) + 1))
		resp.String(msg)
		resp.Uint8(0) // chat tag
		s.gameSocket.Send(resp)

	default:
		if s.worldSocket != nil {
			s.worldSocket.WriteChannel() <- p
		}
	}

	return nil
}

func (s *GameSession) HandleEventIncomingWhisperMessage(ctx context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*eBroadcaster.IncomingWhisperPayload)

	resp := packet.NewWriterWithSize(packet.SMsgMessageChat, 0)
	resp.Uint8(uint8(ChatTypeWhisper))
	resp.Uint32(eventData.Language)
	resp.Uint64(eventData.SenderGUID)
	resp.Uint32(0) // some flags
	resp.Uint64(eventData.SenderGUID)
	resp.Uint32(uint32(len(eventData.Msg) + 1))
	resp.String(eventData.Msg)
	resp.Uint8(0) // chat tag
	s.gameSocket.Send(resp)

	return nil
}
