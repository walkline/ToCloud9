package session

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
	pbGroup "github.com/walkline/ToCloud9/gen/group/pb"
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
	ChatTypeChannel     = 0x11
	ChatTypeRaidLeader  = 0x27
	ChatTypePartyLeader = 0x33
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

	s.logger.Debug().
		Uint32("msgType", msgType).
		Uint32("language", lang).
		Msg("HandleChatMessage received")

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
			SenderRace:   uint32(s.character.Race),
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
	case ChatTypeParty, ChatTypePartyLeader, ChatTypeRaid, ChatTypeRaidLeader:
		msg = r.String()

		handled, err := s.handleCommandMsgIfNeeded(ctx, msg)
		if err != nil {
			return err
		}

		if handled {
			return nil
		}

		_, err = s.groupServiceClient.SendMessage(ctx, &pbGroup.SendGroupMessageParams{
			Api:         root.Ver,
			RealmID:     root.RealmID,
			SenderGUID:  s.character.GUID,
			Language:    lang,
			Message:     msg,
			MessageType: msgType,
		})

		if err != nil {
			return err
		}

		resp := packet.NewWriterWithSize(packet.SMsgMessageChat, 0)
		resp.Uint8(uint8(msgType))
		resp.Uint32(lang)
		resp.Uint64(s.character.GUID)
		resp.Uint32(0) // some flags
		resp.Uint64(s.character.GUID)
		resp.Uint32(uint32(len(msg) + 1))
		resp.String(msg)
		resp.Uint8(0) // chat tag
		s.gameSocket.Send(resp)

	case ChatTypeChannel:
		channelName := r.String()
		msg = r.String()

		handled, err := s.handleCommandMsgIfNeeded(ctx, msg)
		if err != nil {
			return err
		}

		if handled {
			return nil
		}

		// Send channel message through chat service
		return s.SendChannelMessageToChat(ctx, channelName, msg, lang)

	case ChatTypeSay, ChatTypeYell:
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
		s.logger.Debug().
			Uint32("msgType", msgType).
			Uint32("language", lang).
			Msg("HandleChatMessage - default case (msgType decimal), forwarding to worldserver")
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
	if msg == ".layer" || strings.HasPrefix(msg, ".layer ") {
		s.logger.Info().Uint32("accountID", s.accountID).Str("command", msg).Msg("Intercepted layer command")
		return true, s.handleLayerCommand(ctx, strings.Fields(msg))
	}
	const TC9CommandPrefix = ".tc9 "
	if !strings.HasPrefix(msg, TC9CommandPrefix) {
		return false, nil
	}
	gmLevel, err := s.accountGMLevel(ctx)
	if err != nil {
		return true, err
	}
	if gmLevel == 0 {
		s.SendSysMessage("You do not have permission to view server diagnostics.")
		return true, nil
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
	case "gateways", "gw":
		if len(args) < 2 {
			s.SendSysMessage("not enough args")
			return true, nil
		}

		switch strings.ToLower(args[1]) {
		case "list", "ls":
			return true, s.handleCommandMsgListGateways(ctx)
		default:
			s.SendSysMessage("unk subcommand")
		}

	default:
		s.SendSysMessage("unk command")
	}
	return true, nil
}

func (s *GameSession) handleLayerCommand(ctx context.Context, args []string) error {
	gmLevel, err := s.accountGMLevel(ctx)
	if err != nil {
		return err
	}
	if len(args) == 1 {
		if s.character == nil {
			return nil
		}
		resp, err := s.serversRegistryClient.GetLayerStats(ctx, &pbServ.GetLayerStatsRequest{Api: root.SupportedServerRegistryVer, RealmID: root.RealmID, MapID: s.character.Map, PlayerGUID: s.character.GUID})
		if err != nil {
			return err
		}
		currentLayerID := resp.CurrentLayerID
		if currentLayerID == 0 {
			currentLayerID = s.currentLayerID
		}
		if gmLevel == 0 {
			s.SendSysMessage(fmt.Sprintf("You are currently on layer %d.", currentLayerID))
			if resp.SwitchCooldownRemainingSeconds > 0 {
				s.SendSysMessage(fmt.Sprintf("You can switch layers again in %d seconds.", resp.SwitchCooldownRemainingSeconds))
			}
			return nil
		}
		mapConfig, err := s.serversRegistryClient.GetMapLayerConfiguration(ctx, &pbServ.GetMapLayerConfigurationRequest{Api: root.SupportedServerRegistryVer, RealmID: root.RealmID})
		if err != nil {
			return err
		}
		s.SendSysMessage(fmt.Sprintf("Layering: enabled=%t maxPlayers=%d target=%d%% overflow=%d%% layers=%d-%d cooldown=%ds hourlyLimit=%d", resp.Enabled, resp.MaxPopulation, resp.TargetPopulationPercent, resp.OverflowMarginPercent, resp.MinLayers, resp.MaxLayers, resp.SwitchCooldownSeconds, resp.MaxSwitchesPerHour))
		for _, item := range mapConfig.Maps {
			marker := ""
			if s.character != nil && item.MapID == s.character.Map {
				marker = fmt.Sprintf(" (current map, you are on layer %d)", s.currentLayerID)
			}
			s.SendSysMessage(fmt.Sprintf("Map %d: configured layers=%d%s", item.MapID, item.LayerCount, marker))
		}
		if len(resp.Layers) == 0 {
			s.SendSysMessage("No layers have registered ready cores.")
		}
		for _, layer := range resp.Layers {
			state := "active"
			if layer.Draining {
				state = "draining"
			}
			marker := ""
			if layer.LayerID == s.currentLayerID {
				marker = " (you)"
			}
			s.SendSysMessage(fmt.Sprintf("Layer %d: players=%d/%d readyCores=%d state=%s%s", layer.LayerID, layer.CurrentPlayers, resp.MaxPopulation, layer.ReadyCores, state, marker))
		}
		return nil
	}
	if gmLevel == 0 {
		s.SendSysMessage("You do not have permission to switch layers.")
		return nil
	}
	if len(args) < 3 || strings.ToLower(args[1]) != "switch" {
		s.SendSysMessage("Usage: .layer | .layer switch <number> [playername]")
		return nil
	}
	layer64, err := strconv.ParseUint(args[2], 10, 32)
	if err != nil || layer64 == 0 {
		s.SendSysMessage("Layer number must be a positive integer.")
		return nil
	}
	if s.character == nil {
		return nil
	}
	guid, mapID, targetName := s.character.GUID, s.character.Map, s.character.Name
	if len(args) >= 4 {
		character, err := s.charServiceClient.CharacterOnlineByName(ctx, &pbChar.CharacterOnlineByNameRequest{Api: root.Ver, RealmID: root.RealmID, CharacterName: args[3]})
		if err != nil {
			return err
		}
		if character.Character == nil {
			s.SendSysMessage("Player not found or offline.")
			return nil
		}
		guid, mapID, targetName = character.Character.CharGUID, character.Character.CharMap, character.Character.CharName
	}
	resp, err := s.serversRegistryClient.ForcePlayerLayer(ctx, &pbServ.ForcePlayerLayerRequest{Api: root.SupportedServerRegistryVer, RealmID: root.RealmID, PlayerGUID: guid, LayerID: uint32(layer64), MapID: mapID})
	if err != nil {
		return err
	}
	switch resp.Status {
	case pbServ.ForcePlayerLayerResponse_OK:
		s.SendSysMessage(fmt.Sprintf("Forced %s to layer %d; the redirect will begin shortly.", targetName, layer64))
	case pbServ.ForcePlayerLayerResponse_PLAYER_OFFLINE:
		s.SendSysMessage("Player is not tracked as online by the layering service.")
	case pbServ.ForcePlayerLayerResponse_LAYER_NOT_FOUND:
		s.SendSysMessage("That layer does not exist.")
	case pbServ.ForcePlayerLayerResponse_NO_COMPATIBLE_CORE:
		s.SendSysMessage("That layer has no ready core for the player's current map.")
	}
	return nil
}

func (s *GameSession) handleCommandMsgListGameServers(ctx context.Context) error {
	resp, err := s.serversRegistryClient.ListAllGameServers(ctx, &pbServ.ListAllGameServersRequest{
		Api: root.SupportedServerRegistryVer,
	})
	if err != nil {
		return err
	}

	printServer := func(server *pbServ.GameServerDetailed) {
		mapsAvailable := "all"
		if len(server.AvailableMaps) > 0 {
			mapsAvailable = ""
			for _, availableMap := range server.AvailableMaps {
				mapsAvailable += fmt.Sprintf("%d ", availableMap)
			}
		}

		const maxMapsToShow = 8
		assignedMaps := ""
		if len(server.AssignedMaps) > maxMapsToShow {
			for i := 0; i < maxMapsToShow; i++ {
				assignedMaps += fmt.Sprintf("%d ", server.AssignedMaps[i])
			}
			assignedMaps += fmt.Sprintf("and %d more", len(server.AssignedMaps)-maxMapsToShow)
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
		s.SendSysMessage(fmt.Sprintf("  Active connections: %d.", server.ActiveConnections))
		s.SendSysMessage(
			fmt.Sprintf(
				"  Diff (mean, median, 95, 99, max): %dms, %dms, %dms, %dms, %dms.",
				server.Diff.Mean, server.Diff.Median, server.Diff.Percentile95,
				server.Diff.Percentile99, server.Diff.Max,
			),
		)

		if isCurrentlyUsing {
			s.SendSysMessage("  You are |cff4CFF00connected |rto this one.")
		}

		s.SendSysMessage(" ")
	}

	var crossrealms []*pbServ.GameServerDetailed
	perRealm := make(map[uint32][]*pbServ.GameServerDetailed)
	for _, server := range resp.GameServers {
		if server.IsCrossRealm {
			crossrealms = append(crossrealms, server)
			continue
		}

		perRealm[server.RealmID] = append(perRealm[server.RealmID], server)
	}

	if len(crossrealms) > 0 {
		s.SendSysMessage(fmt.Sprintf("List of available |cff4f90ffcrossrealm|r worldservers:"))
		for _, server := range crossrealms {
			printServer(server)
		}
	}

	for realm, servers := range perRealm {
		s.SendSysMessage(fmt.Sprintf("List of available worldservers for |cff4f90ffrealm %d|r:", realm))
		for _, server := range servers {
			printServer(server)
		}
	}

	return nil
}

func (s *GameSession) handleCommandMsgListGateways(ctx context.Context) error {
	resp, err := s.serversRegistryClient.ListGatewaysForRealm(ctx, &pbServ.ListGatewaysForRealmRequest{
		Api:     root.SupportedServerRegistryVer,
		RealmID: root.RealmID,
	})
	if err != nil {
		return err
	}

	s.SendSysMessage("List of available |cffF84519gateways|r:")

	for _, server := range resp.Gateways {
		isCurrentlyUsing := root.RetrievedGatewayID == server.Id

		s.SendSysMessage(fmt.Sprintf("> Node healthCheckAddress: %s.", server.HealthAddress))
		s.SendSysMessage(fmt.Sprintf("  Active connections: %d.", server.ActiveConnections))
		if isCurrentlyUsing {
			s.SendSysMessage("  You are |cff4CFF00connected |rto this one.")
		}

		s.SendSysMessage(" ")
	}

	return nil
}
