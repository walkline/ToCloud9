package session

import (
	"context"
	"fmt"
	"strings"

	root "github.com/walkline/ToCloud9/apps/game-load-balancer"
	eBroadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
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
	switch ChatType(msgType) {
	case ChatTypeWhisper:
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
	case ChatTypeGuild:
		msg = r.String()

		handled, err := s.handleCommandMsgIfNeeded(ctx, msg)
		if err != nil {
			return err
		}

		if handled {
			return nil
		}

		_, err = s.guildServiceClient.SendGuildMessage(ctx, &pbGuild.SendGuildMessageParams{
			Api:              root.Ver,
			RealmID:          root.RealmID,
			SenderGUID:       s.character.GUID,
			Language:         lang,
			Message:          msg,
			IsOfficerMessage: false,
		})

		if err != nil {
			return err
		}

		resp := packet.NewWriterWithSize(packet.SMsgMessageChat, 0)
		resp.Uint8(uint8(ChatTypeGuild))
		resp.Uint32(lang)
		resp.Uint64(s.character.GUID)
		resp.Uint32(0) // some flags
		resp.Uint64(s.character.GUID)
		resp.Uint32(uint32(len(msg) + 1))
		resp.String(msg)
		resp.Uint8(0) // chat tag
		s.gameSocket.Send(resp)

	case ChatTypeSay, ChatTypeParty, ChatTypeRaid:
		msg = r.String()

		handled, err := s.handleCommandMsgIfNeeded(ctx, msg)
		if err != nil {
			return err
		}

		if handled {
			return nil
		}

		if s.worldSocket != nil {
			s.worldSocket.WriteChannel() <- p
		}
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

// TODO: rewrite commands handler with some better and more manageable constructions.
func (s *GameSession) handleCommandMsgIfNeeded(ctx context.Context, msg string) ( /* isHandled */ bool, error) {
	const TC9CommandPrefix = ".tc9 "
	if !strings.HasPrefix(msg, TC9CommandPrefix) {
		return false, nil
	}

	args := strings.Split(msg[len(TC9CommandPrefix):], " ")
	if len(args) == 0 {
		return true, nil
	}

	switch strings.ToLower(args[0]) {
	case "worldservers", "ws", "gameservers", "gs":
		if len(args) < 2 {
			s.SendSysMessage("not enough args")
			return true, nil
		}

		switch strings.ToLower(args[1]) {
		case "list", "ls":
			return true, s.handleCommandMsgListGameServers(ctx)
		default:
			s.SendSysMessage("unk subcommand")
		}
	case "loadbalancers", "lb", "gameloadbalancers", "glb":
		if len(args) < 2 {
			s.SendSysMessage("not enough args")
			return true, nil
		}

		switch strings.ToLower(args[1]) {
		case "list", "ls":
			return true, s.handleCommandMsgListLoadBalancers(ctx)
		default:
			s.SendSysMessage("unk subcommand")
		}

	default:
		s.SendSysMessage("unk command")
	}
	return true, nil
}

func (s *GameSession) handleCommandMsgListGameServers(ctx context.Context) error {
	resp, err := s.serversRegistryClient.ListGameServersForRealm(ctx, &pbServ.ListGameServersForRealmRequest{
		Api:     root.SupportedServerRegistryVer,
		RealmID: root.RealmID,
	})
	if err != nil {
		return err
	}

	s.SendSysMessage("List of available |cff7321EFworld servers|r:")

	for _, server := range resp.GameServers {
		mapsAvailable := "all"
		if len(server.AvailableMaps) > 0 {
			mapsAvailable = ""
			for _, availableMap := range server.AvailableMaps {
				mapsAvailable += fmt.Sprintf("%d ", availableMap)
			}
		}

		assignedMaps := ""
		if len(server.AssignedMaps) > 3 {
			for i := 0; i < 3; i++ {
				assignedMaps += fmt.Sprintf("%d ", server.AssignedMaps[i])
			}
			assignedMaps += fmt.Sprintf("and %d more", len(server.AssignedMaps)-3)
		} else {
			for i := 0; i < len(server.AssignedMaps); i++ {
				assignedMaps += fmt.Sprintf("%d ", server.AssignedMaps[i])
			}
		}

		isCurrentlyUsing := false
		if s.worldSocket != nil && s.worldSocket.Address() == server.Address {
			isCurrentlyUsing = true
		}

		s.SendSysMessage(fmt.Sprintf("> Node address: %s.", server.Address))
		s.SendSysMessage(fmt.Sprintf("  Available maps: %s.", mapsAvailable))
		s.SendSysMessage(fmt.Sprintf("  Assigned maps: %s.", assignedMaps))

		if isCurrentlyUsing {
			s.SendSysMessage("  You are |cff4CFF00connected |rto this one.")
		}

		s.SendSysMessage(" ")
	}

	return nil
}

func (s *GameSession) handleCommandMsgListLoadBalancers(ctx context.Context) error {
	resp, err := s.serversRegistryClient.ListLoadBalancersForRealm(ctx, &pbServ.ListLoadBalancersForRealmRequest{
		Api:     root.SupportedServerRegistryVer,
		RealmID: root.RealmID,
	})
	if err != nil {
		return err
	}

	s.SendSysMessage("List of available |cffF84519game-load-balancers|r:")

	for _, server := range resp.LoadBalancers {
		isCurrentlyUsing := root.RetrievedBalancerID == server.Id

		s.SendSysMessage(fmt.Sprintf("> Node healthCheckAddress: %s.", server.HealthAddress))
		s.SendSysMessage(fmt.Sprintf("  Active connections: %d.", server.ActiveConnections))
		if isCurrentlyUsing {
			s.SendSysMessage("  You are |cff4CFF00connected |rto this one.")
		}

		s.SendSysMessage(" ")
	}

	return nil
}
