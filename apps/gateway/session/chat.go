package session

import (
	"context"
	"fmt"
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
	ChatTypeRaidWarning = 0x28
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

func (s *GameSession) SendPlayerNotFoundNotice(name string) {
	resp := packet.NewWriterWithSize(packet.SMsgChatPlayerNotFound, uint32(len(name)+1))
	resp.String(name)
	s.gameSocket.Send(resp)
}

func (s *GameSession) SendPlayerAmbiguousNotice(name string) {
	resp := packet.NewWriterWithSize(packet.SMsgChatPlayerAmbiguous, uint32(len(name)+1))
	resp.String(name)
	s.gameSocket.Send(resp)
}

func (s *GameSession) sendAzerothCorePlayerChat(chatType ChatType, language uint32, senderRealmID uint32, senderGUID uint64, senderName string, receiverGUID uint64, channelName string, msg string, chatTag uint8) {
	senderObjectGUID := playerObjectGUIDForRealm(senderRealmID, senderGUID)
	gmMessage := chatTag&chatTagGM != 0
	opcode := packet.SMsgMessageChat
	if gmMessage {
		opcode = packet.SMsgGmMessageChat
	}

	resp := packet.NewWriterWithSize(opcode, 0)
	resp.Uint8(uint8(chatType))
	resp.Uint32(language)
	resp.Uint64(senderObjectGUID)
	resp.Uint32(0)

	switch chatType {
	case ChatTypeWhisperForeign:
		resp.Uint32(uint32(len(senderName) + 1))
		resp.String(senderName)
		resp.Uint64(receiverGUID)
	default:
		if gmMessage {
			resp.Uint32(uint32(len(senderName) + 1))
			resp.String(senderName)
		}
		if chatType == ChatTypeChannel {
			resp.String(channelName)
		}
		resp.Uint64(receiverGUID)
	}

	resp.Uint32(uint32(len(msg) + 1))
	resp.String(msg)
	resp.Uint8(chatTag)
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
		receiverRealmID, receiverName := s.whisperTargetNameRealm(ctx, to)
		gatewayValidatedGameplayCrossrealmWhisper := false
		if receiverRealmID != 0 && receiverRealmID != root.RealmID {
			var err error
			gatewayValidatedGameplayCrossrealmWhisper, err = s.gameplayCrossrealmWhisperAllowed(ctx, receiverRealmID, receiverName)
			if err != nil {
				return err
			}
			allowed := gatewayValidatedGameplayCrossrealmWhisper
			if !allowed {
				allowed, err = s.explicitCrossrealmWhisperAllowed(ctx, receiverRealmID, receiverName)
			}
			if err != nil {
				return err
			}
			if !allowed {
				s.SendPlayerNotFoundNotice(to)
				return nil
			}
		}

		res, err := s.chatServiceClient.SendWhisperMessage(ctx, &pbChat.SendWhisperMessageRequest{
			Api:             root.Ver,
			RealmID:         root.RealmID,
			SenderGUID:      s.character.GUID,
			SenderAccountID: s.senderAccountID(),
			SenderName:      s.character.Name,
			SenderRace:      uint32(s.character.Race),
			SenderClass:     uint32(s.character.Class),
			SenderGender:    uint32(s.character.Gender),
			Language:        lang,
			ReceiverRealmID: receiverRealmID,
			ReceiverName:    receiverName,
			Msg:             msg,
			GatewayValidatedGameplayCrossrealmWhisper: gatewayValidatedGameplayCrossrealmWhisper,
			SenderChatTag: uint32(s.currentChatTag()),
		})

		if err != nil {
			return err
		}
		if res.GetStatus() == pbChat.SendWhisperMessageResponse_CharacterAmbiguous {
			s.SendPlayerAmbiguousNotice(to)
			return nil
		}
		if res.GetStatus() == pbChat.SendWhisperMessageResponse_CharacterNotFound || res.GetReceiverGUID() == 0 {
			s.SendPlayerNotFoundNotice(to)
			return nil
		}
		if res.GetStatus() != pbChat.SendWhisperMessageResponse_Ok {
			return fmt.Errorf("can't send whisper to %s: %s", to, res.GetStatus().String())
		}

		if res.GetReceiverRealmID() != 0 && res.GetReceiverRealmID() != root.RealmID {
			receiverDisplayName := res.GetReceiverName()
			if receiverDisplayName == "" {
				receiverDisplayName = receiverName
			}
			s.sendNameQueryResponse(ctx, res.GetReceiverGUID(), receiverDisplayName, res.GetReceiverRace(), res.GetReceiverClass(), res.GetReceiverGender())
		}

		s.sendAzerothCorePlayerChat(ChatTypeWhisperInform, lang, res.ReceiverRealmID, res.ReceiverGUID, res.ReceiverName, res.ReceiverGUID, "", msg, 0)
	case ChatTypeGuild, ChatTypeOfficer:
		msg = r.String()
		chatTag := s.currentChatTag()

		handled, err := s.handleCommandMsgIfNeeded(ctx, msg)
		if err != nil {
			return err
		}

		if handled {
			return nil
		}

		if s.forwardAzerothCommandMsgIfNeeded(msg, p) {
			return nil
		}

		_, err = s.guildServiceClient.SendGuildMessage(ctx, &pbGuild.SendGuildMessageParams{
			Api:              root.Ver,
			RealmID:          s.guildHomeRealmID(),
			SenderGUID:       s.character.GUID,
			Language:         lang,
			Message:          msg,
			IsOfficerMessage: ChatType(msgType) == ChatTypeOfficer,
			SenderChatTag:    uint32(chatTag),
		})

		if err != nil {
			return err
		}

		s.sendAzerothCorePlayerChat(ChatType(msgType), lang, root.RealmID, s.character.GUID, s.character.Name, 0, "", msg, chatTag)
	case ChatTypeParty, ChatTypePartyLeader, ChatTypeRaid, ChatTypeRaidLeader, ChatTypeRaidWarning:
		msg = r.String()
		chatTag := s.currentChatTag()

		handled, err := s.handleCommandMsgIfNeeded(ctx, msg)
		if err != nil {
			return err
		}

		if handled {
			return nil
		}

		if s.forwardAzerothCommandMsgIfNeeded(msg, p) {
			return nil
		}

		_, err = s.groupServiceClient.SendMessage(ctx, &pbGroup.SendGroupMessageParams{
			Api:           root.Ver,
			RealmID:       root.RealmID,
			SenderGUID:    s.character.GUID,
			Language:      lang,
			Message:       msg,
			MessageType:   msgType,
			SenderChatTag: uint32(chatTag),
		})

		if err != nil {
			return err
		}

		s.sendAzerothCorePlayerChat(ChatType(msgType), lang, root.RealmID, s.character.GUID, s.character.Name, 0, "", msg, chatTag)

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

		if s.forwardAzerothCommandMsgIfNeeded(msg, p) {
			return nil
		}

		if !s.isGatewayManagedChannel(channelName) && s.worldSocket != nil {
			s.worldSocket.WriteChannel() <- p
			return nil
		}

		// Send channel message through chat service
		return s.SendChannelMessageToChat(ctx, channelName, msg, lang)

	case ChatTypeSay:
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
	senderGUID := playerObjectGUIDForRealm(eventData.SenderRealmID, eventData.SenderGUID)

	if eventData.SenderRealmID != 0 && eventData.SenderRealmID != root.RealmID {
		s.sendNameQueryResponse(ctx, senderGUID, eventData.SenderName, uint32(eventData.SenderRace), uint32(eventData.SenderClass), uint32(eventData.SenderGender))
		s.sendAzerothCorePlayerChat(ChatTypeWhisperForeign, eventData.Language, eventData.SenderRealmID, eventData.SenderGUID, eventData.SenderName, senderGUID, "", eventData.Msg, eventData.SenderChatTag)
		return nil
	}

	s.sendAzerothCorePlayerChat(ChatTypeWhisper, eventData.Language, eventData.SenderRealmID, eventData.SenderGUID, eventData.SenderName, senderGUID, "", eventData.Msg, eventData.SenderChatTag)

	return nil
}

func (s *GameSession) whisperTargetNameRealm(ctx context.Context, characterName string) (uint32, string) {
	if s.realmNamesService == nil {
		return 0, characterName
	}

	separator := strings.LastIndex(characterName, "-")
	if separator <= 0 || separator == len(characterName)-1 {
		return 0, characterName
	}

	name := characterName[:separator]
	realmName := characterName[separator+1:]
	realmID, err := s.realmNamesService.IDByName(ctx, realmName)
	if err != nil {
		return 0, characterName
	}

	return realmID, name
}

func (s *GameSession) senderAccountID() uint32 {
	if s.accountID != 0 {
		return s.accountID
	}
	if s.character != nil {
		return s.character.AccountID
	}
	return 0
}

func (s *GameSession) explicitCrossrealmWhisperAllowed(ctx context.Context, receiverRealmID uint32, receiverName string) (bool, error) {
	senderAccountID := s.senderAccountID()
	if s.charServiceClient == nil || senderAccountID == 0 {
		return false, nil
	}

	charRes, err := s.charServiceClient.CharacterByName(ctx, &pbChar.CharacterByNameRequest{
		Api:           root.Ver,
		RealmID:       receiverRealmID,
		CharacterName: receiverName,
	})
	if err != nil {
		return false, fmt.Errorf("failed to lookup crossrealm whisper target: %w", err)
	}
	if charRes.GetCharacter() == nil || charRes.GetCharacter().GetAccountID() == 0 {
		return false, nil
	}

	friendRes, err := s.charServiceClient.AreRealIDFriends(ctx, &pbChar.AreRealIDFriendsRequest{
		Api:             root.Ver,
		AccountID:       senderAccountID,
		FriendAccountID: charRes.GetCharacter().GetAccountID(),
	})
	if err != nil {
		return false, fmt.Errorf("failed to validate real id whisper relation: %w", err)
	}

	return friendRes.GetAccepted(), nil
}

func (s *GameSession) gameplayCrossrealmWhisperAllowed(ctx context.Context, receiverRealmID uint32, receiverName string) (bool, error) {
	if s == nil || s.character == nil || s.charServiceClient == nil || s.character.Map == 0 {
		return false, nil
	}
	if !s.hasGameplayCrossrealmWhisperContext() {
		return false, nil
	}

	targetRes, err := s.charServiceClient.CharacterOnlineByName(ctx, &pbChar.CharacterOnlineByNameRequest{
		Api:           root.Ver,
		RealmID:       receiverRealmID,
		CharacterName: receiverName,
	})
	if err != nil {
		return false, fmt.Errorf("failed to lookup online crossrealm gameplay whisper target: %w", err)
	}

	target := targetRes.GetCharacter()
	if target == nil || target.GetRealmID() != receiverRealmID || target.GetCharMap() != s.character.Map {
		return false, nil
	}

	s.logger.Debug().
		Uint32("senderRealmID", root.RealmID).
		Uint32("receiverRealmID", receiverRealmID).
		Uint32("mapID", s.character.Map).
		Str("receiverName", receiverName).
		Msg("Allowed explicit crossrealm whisper in shared gameplay context")
	return true, nil
}

func (s *GameSession) hasGameplayCrossrealmWhisperContext() bool {
	return gameplayCrossrealmWhisperRouting(s.currentMapTransferRouting) ||
		gameplayCrossrealmWhisperRouting(s.activeMapTransferRouting) ||
		gameplayCrossrealmWhisperRouting(s.pendingMapTransferRouting)
}

func gameplayCrossrealmWhisperRouting(routing *mapTransferRouting) bool {
	if routing == nil || !routing.isCrossRealm || routing.realmID != 0 {
		return false
	}

	switch routing.feature {
	case clusterTransferFeatureLFG, clusterTransferFeatureBattleground, clusterTransferFeatureArena, clusterTransferFeatureWintergrasp:
		return true
	default:
		return false
	}
}

func isAzerothCommandMessage(msg string) bool {
	if len(msg) < 2 {
		return false
	}
	if msg[0] != '.' && msg[0] != '!' {
		return false
	}
	return msg[1] != msg[0]
}

func (s *GameSession) forwardAzerothCommandMsgIfNeeded(msg string, p *packet.Packet) bool {
	if !isAzerothCommandMessage(msg) {
		return false
	}
	s.trackLocalGMChatCommand(msg)
	if s.worldSocket != nil {
		s.worldSocket.WriteChannel() <- p
	}
	return true
}

func (s *GameSession) trackLocalGMChatCommand(msg string) {
	if s == nil || s.character == nil {
		return
	}

	args := strings.Fields(strings.ToLower(msg))
	if len(args) != 3 || args[0] != ".gm" || args[1] != "chat" {
		return
	}

	switch args[2] {
	case "on":
		s.character.ExtraFlags |= playerExtraFlagGMChat
	case "off":
		s.character.ExtraFlags &^= playerExtraFlagGMChat
	}
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
