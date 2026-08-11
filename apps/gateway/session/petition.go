package session

import (
	"context"
	"fmt"
	"unicode"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	root "github.com/walkline/ToCloud9/apps/gateway"
	eBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/packet"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	pbWorld "github.com/walkline/ToCloud9/gen/worldserver/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

// SMSG_TURN_IN_PETITION_RESULTS codes.
const (
	petitionTurnOK                 = 0 // PETITION_TURN_OK
	petitionTurnAlreadyInGuild     = 2 // PETITION_TURN_ALREADY_IN_GUILD
	petitionTurnNeedMoreSignatures = 4 // PETITION_TURN_NEED_MORE_SIGNATURES
)

// SMSG_PETITION_SIGN_RESULTS codes (AC PetitionSigns).
const (
	petitionSignOK            = 0 // PETITION_SIGN_OK
	petitionSignAlreadySigned = 1 // PETITION_SIGN_ALREADY_SIGNED
)

func (s *GameSession) sendTurnInPetitionResult(result uint32) {
	w := packet.NewWriterWithSize(packet.SMsgTurnInPetitionResults, 4)
	w.Uint32(result)
	s.gameSocket.Send(w)
}

func (s *GameSession) sendPetitionSignResult(petitionGUID, playerGUID uint64, result uint32) {
	w := packet.NewWriterWithSize(packet.SMsgPetitionSignResults, 8+8+4)
	w.Uint64(petitionGUID)
	w.Uint64(playerGUID)
	w.Uint32(result)
	s.gameSocket.Send(w)
}

func (s *GameSession) sendPetitionShowSignatures(petitionGUID, ownerGUID uint64, petitionID uint32, signatoryGUIDs []uint64) {
	size := uint32(8 + 8 + 4 + 1 + len(signatoryGUIDs)*12)
	w := packet.NewWriterWithSize(packet.SMsgPetitionShowSignatures, size)
	w.Uint64(petitionGUID)
	w.Uint64(ownerGUID)
	w.Uint32(petitionID)
	w.Uint8(uint8(len(signatoryGUIDs)))
	for _, guid := range signatoryGUIDs {
		w.Uint64(guid)
		w.Uint32(0)
	}
	s.gameSocket.Send(w)
}

func (s *GameSession) sendPetitionQueryResponse(p *pbChar.GuildPetition) {
	const minSigns = uint32(9)
	needed := minSigns

	w := packet.NewWriter(packet.SMsgPetitionQueryResponse)
	w.Uint32(p.PetitionID)
	w.Uint64(p.OwnerGUID)
	w.String(p.Name)
	w.Uint8(0) // empty body string
	w.Uint32(needed)
	w.Uint32(needed)
	w.Uint32(0) // bypass client-side limitation
	w.Uint32(0)
	w.Uint32(0)
	w.Uint32(0)
	w.Uint32(0)
	w.Uint16(0)
	w.Uint32(0)
	w.Uint32(0)
	w.Uint32(0)
	for i := 0; i < 10; i++ {
		w.Uint8(0)
	}
	w.Uint32(0)
	w.Uint32(0) // 0 = guild, 1 = arena
	s.gameSocket.Send(w)
}

// HandlePetitionShowSignatures handles CMSG_PETITION_SHOW_SIGNATURES.
// Arena charters are forwarded to the worldserver.
func (s *GameSession) HandlePetitionShowSignatures(ctx context.Context, p *packet.Packet) error {
	petitionGUID := p.Reader().Uint64()

	resp, err := s.charServiceClient.GetGuildPetitionSignatures(ctx, &pbChar.GetGuildPetitionSignaturesRequest{
		Api:              root.Ver,
		RealmID:          root.RealmID,
		PetitionItemGUID: petitionGUID,
	})
	if err != nil {
		return fmt.Errorf("get guild petition signatures: %w", err)
	}

	switch resp.Status {
	case pbChar.GetGuildPetitionSignaturesResponse_NotFound, pbChar.GetGuildPetitionSignaturesResponse_NotGuildPetition:
		s.worldSocket.SendPacket(p)
		return nil
	}

	if s.character.GuildID != 0 {
		return nil
	}

	signatoryGUIDs := make([]uint64, 0, len(resp.Signatures))
	for _, sig := range resp.Signatures {
		signatoryGUIDs = append(signatoryGUIDs, sig.PlayerGUID)
	}

	s.sendPetitionShowSignatures(petitionGUID, s.character.GUID, resp.Petition.PetitionID, signatoryGUIDs)
	return nil
}

func (s *GameSession) HandlePetitionSign(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	petitionGUID := r.Uint64()
	_ = r.Uint8() // unk

	petResp, err := s.charServiceClient.GetGuildPetition(ctx, &pbChar.GetGuildPetitionRequest{
		Api:              root.Ver,
		RealmID:          root.RealmID,
		PetitionItemGUID: petitionGUID,
	})
	if err != nil {
		return fmt.Errorf("get guild petition: %w", err)
	}
	switch petResp.Status {
	case pbChar.GetGuildPetitionResponse_NotFound, pbChar.GetGuildPetitionResponse_NotGuildPetition:
		s.worldSocket.SendPacket(p)
		return nil
	}

	if !s.allowCrossFactionGuilds {
		// CharactersToLoginByGUID covers offline owners (online short data does not).
		ownerLogin, err := s.charServiceClient.CharactersToLoginByGUID(ctx, &pbChar.CharactersToLoginByGUIDRequest{
			Api:           root.SupportedCharServiceVer,
			CharacterGUID: petResp.Petition.OwnerGUID,
			RealmID:       root.RealmID,
		})
		if err != nil {
			return fmt.Errorf("lookup petition owner: %w", err)
		}
		if ownerLogin.Character != nil {
			if isCrossFaction(s.character.Race, uint8(ownerLogin.Character.Race)) {
				s.sendGuildCommandResult(guildCommandCreate, "", guildErrNotAllied)
				return nil
			}
		}
	}

	signResp, err := s.charServiceClient.AddGuildPetitionSignature(ctx, &pbChar.AddGuildPetitionSignatureRequest{
		Api:              root.Ver,
		RealmID:          root.RealmID,
		PetitionItemGUID: petitionGUID,
		SignerGUID:       s.character.GUID,
		SignerAccountID:  s.accountID,
	})
	if err != nil {
		return fmt.Errorf("add guild petition signature: %w", err)
	}

	switch signResp.Result {
	case pbChar.AddGuildPetitionSignatureResponse_Ok:
		s.sendPetitionSignResult(petitionGUID, s.character.GUID, petitionSignOK)
	case pbChar.AddGuildPetitionSignatureResponse_AlreadySigned:
		s.sendPetitionSignResult(petitionGUID, s.character.GUID, petitionSignAlreadySigned)
	case pbChar.AddGuildPetitionSignatureResponse_AlreadyInGuild:
		s.sendGuildCommandResult(guildCommandInvite, s.character.Name, guildErrAlreadyInGuildS)
	case pbChar.AddGuildPetitionSignatureResponse_AlreadyInvitedToGuild:
		const guildErrAlreadyInvitedToGuildS = 4 // ERR_ALREADY_INVITED_TO_GUILD_S
		s.sendGuildCommandResult(guildCommandInvite, s.character.Name, guildErrAlreadyInvitedToGuildS)
	case pbChar.AddGuildPetitionSignatureResponse_CantSignOwn, pbChar.AddGuildPetitionSignatureResponse_MaxSignatures, pbChar.AddGuildPetitionSignatureResponse_NotFound:
		// silent
	default:
	}

	return nil
}

func (s *GameSession) HandleOfferPetition(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	_ = r.Uint32() // junk, not petition type
	petitionGUID := r.Uint64()
	targetGUID := r.Uint64()

	resp, err := s.charServiceClient.GetGuildPetitionSignatures(ctx, &pbChar.GetGuildPetitionSignaturesRequest{
		Api:              root.Ver,
		RealmID:          root.RealmID,
		PetitionItemGUID: petitionGUID,
	})
	if err != nil {
		return fmt.Errorf("get guild petition signatures: %w", err)
	}
	switch resp.Status {
	case pbChar.GetGuildPetitionSignaturesResponse_NotFound, pbChar.GetGuildPetitionSignaturesResponse_NotGuildPetition:
		s.worldSocket.SendPacket(p)
		return nil
	}

	online, err := s.charServiceClient.ShortOnlineCharactersDataByGUIDs(ctx, &pbChar.ShortCharactersDataByGUIDsRequest{
		Api:     root.Ver,
		RealmID: root.RealmID,
		GUIDs:   []uint64{targetGUID},
	})
	if err != nil {
		return fmt.Errorf("lookup offer target: %w", err)
	}
	if len(online.Characters) == 0 {
		return nil
	}
	target := online.Characters[0]

	if !s.allowCrossFactionGuilds && isCrossFaction(s.character.Race, uint8(target.CharRace)) {
		s.sendGuildCommandResult(guildCommandCreate, "", guildErrNotAllied)
		return nil
	}
	// Online CharGuildID is only set at login; re-read guild_member for accuracy.
	loginResp, err := s.charServiceClient.CharactersToLoginByGUID(ctx, &pbChar.CharactersToLoginByGUIDRequest{
		Api:           root.Ver,
		RealmID:       root.RealmID,
		CharacterGUID: target.CharGUID,
	})
	if err != nil {
		return fmt.Errorf("lookup offer target guild: %w", err)
	}
	if loginResp.GetCharacter() != nil && loginResp.Character.GuildID != 0 {
		s.sendGuildCommandResult(guildCommandInvite, target.CharName, guildErrAlreadyInGuildS)
		return nil
	}

	signatoryGUIDs := make([]uint64, 0, len(resp.Signatures))
	for _, sig := range resp.Signatures {
		signatoryGUIDs = append(signatoryGUIDs, sig.PlayerGUID)
	}

	if s.petitionEventsProducer != nil {
		_ = s.petitionEventsProducer.Offered(&events.PetitionEventOfferedPayload{
			RealmID:          root.RealmID,
			PetitionItemGUID: petitionGUID,
			OwnerGUID:        s.character.GUID,
			PetitionID:       resp.Petition.PetitionID,
			TargetGUID:       targetGUID,
			SignatoryGUIDs:   signatoryGUIDs,
		})
	} else {
		s.eventsBroadcaster.NewPetitionOfferedEvent(&eBroadcaster.PetitionOfferedPayload{
			RealmID:          root.RealmID,
			PetitionItemGUID: petitionGUID,
			OwnerGUID:        s.character.GUID,
			PetitionID:       resp.Petition.PetitionID,
			TargetGUID:       targetGUID,
			SignatoryGUIDs:   signatoryGUIDs,
		})
	}

	return nil
}

func (s *GameSession) HandlePetitionQuery(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	_ = r.Uint32() // guild/petition id (legacy)
	petitionGUID := r.Uint64()

	resp, err := s.charServiceClient.GetGuildPetition(ctx, &pbChar.GetGuildPetitionRequest{
		Api:              root.Ver,
		RealmID:          root.RealmID,
		PetitionItemGUID: petitionGUID,
	})
	if err != nil {
		return fmt.Errorf("get guild petition: %w", err)
	}
	switch resp.Status {
	case pbChar.GetGuildPetitionResponse_NotFound, pbChar.GetGuildPetitionResponse_NotGuildPetition:
		s.worldSocket.SendPacket(p)
		return nil
	}

	s.sendPetitionQueryResponse(resp.Petition)
	return nil
}

func (s *GameSession) HandlePetitionRename(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	petitionGUID := r.Uint64()
	newName := r.String()

	resp, err := s.charServiceClient.GetGuildPetition(ctx, &pbChar.GetGuildPetitionRequest{
		Api:              root.Ver,
		RealmID:          root.RealmID,
		PetitionItemGUID: petitionGUID,
	})
	if err != nil {
		return fmt.Errorf("get guild petition: %w", err)
	}
	switch resp.Status {
	case pbChar.GetGuildPetitionResponse_NotFound, pbChar.GetGuildPetitionResponse_NotGuildPetition:
		s.worldSocket.SendPacket(p)
		return nil
	}

	if !isValidCharterName(newName) {
		s.sendGuildCommandResult(guildCommandCreate, newName, guildErrNameInvalid)
		return nil
	}
	exists, err := s.charServiceClient.GuildNameExists(ctx, &pbChar.GuildNameExistsRequest{
		Api:     root.Ver,
		RealmID: root.RealmID,
		Name:    newName,
	})
	if err != nil {
		return fmt.Errorf("check guild name for petition rename: %w", err)
	}
	if exists.GetExists() {
		s.sendGuildCommandResult(guildCommandCreate, newName, guildErrNameExistsS)
		return nil
	}

	renameResp, err := s.charServiceClient.RenameGuildPetition(ctx, &pbChar.RenameGuildPetitionRequest{
		Api:              root.Ver,
		RealmID:          root.RealmID,
		PetitionItemGUID: petitionGUID,
		OwnerGUID:        s.character.GUID,
		NewName:          newName,
	})
	if err != nil {
		return fmt.Errorf("rename guild petition: %w", err)
	}
	if renameResp.Status != pbChar.RenameGuildPetitionResponse_Ok {
		return nil
	}

	w := packet.NewWriter(packet.MsgPetitionRename)
	w.Uint64(petitionGUID)
	w.String(newName)
	s.gameSocket.Send(w)
	return nil
}

func (s *GameSession) HandlePetitionDecline(ctx context.Context, p *packet.Packet) error {
	petitionGUID := p.Reader().Uint64()

	resp, err := s.charServiceClient.GetGuildPetition(ctx, &pbChar.GetGuildPetitionRequest{
		Api:              root.Ver,
		RealmID:          root.RealmID,
		PetitionItemGUID: petitionGUID,
	})
	if err != nil {
		return fmt.Errorf("get guild petition: %w", err)
	}
	switch resp.Status {
	case pbChar.GetGuildPetitionResponse_NotFound, pbChar.GetGuildPetitionResponse_NotGuildPetition:
		s.worldSocket.SendPacket(p)
		return nil
	}

	if s.petitionEventsProducer != nil {
		_ = s.petitionEventsProducer.Declined(&events.PetitionEventDeclinedPayload{
			RealmID:    root.RealmID,
			OwnerGUID:  resp.Petition.OwnerGUID,
			SignerGUID: s.character.GUID,
		})
	} else {
		s.eventsBroadcaster.NewPetitionDeclinedEvent(&eBroadcaster.PetitionDeclinedPayload{
			RealmID:    root.RealmID,
			OwnerGUID:  resp.Petition.OwnerGUID,
			SignerGUID: s.character.GUID,
		})
	}
	return nil
}

const (
	npcFlagPetitioner     uint32 = 0x00040000 // UNIT_NPC_FLAG_PETITIONER
	npcFlagTabardDesigner uint32 = 0x00080000 // UNIT_NPC_FLAG_TABARDDESIGNER
	npcFlagGuildCharter   uint32 = npcFlagPetitioner | npcFlagTabardDesigner
	guildCharterItemEntry uint32 = 5863 // GUILD_CHARTER
)

// HandlePetitionBuy handles CMSG_PETITION_BUY for guild charters.
// Arena / non-tabard petitioners are forwarded to the worldserver.
func (s *GameSession) HandlePetitionBuy(ctx context.Context, p *packet.Packet) error {
	r := p.Reader()
	npcGUID := r.Uint64()
	_ = r.Uint32()
	_ = r.Uint64()
	name := r.String()
	// Skip unused client fields (AC HandlePetitionBuyOpcode).
	_ = r.String()
	for i := 0; i < 7; i++ {
		_ = r.Uint32()
	}
	_ = r.Uint16()
	_ = r.Uint32()
	_ = r.Uint32()
	_ = r.Uint32()
	for i := 0; i < 10; i++ {
		_ = r.String()
	}
	_ = r.Uint32() // clientIndex
	_ = r.Uint32()

	if s.character.GuildID != 0 {
		return nil
	}
	if !isValidCharterName(name) {
		s.sendGuildCommandResult(guildCommandCreate, name, guildErrNameInvalid)
		return nil
	}

	nameTaken, err := s.charServiceClient.GuildNameExists(ctx, &pbChar.GuildNameExistsRequest{
		Api:     root.Ver,
		RealmID: root.RealmID,
		Name:    name,
	})
	if err != nil {
		return fmt.Errorf("check guild name for petition buy: %w", err)
	}
	if nameTaken.GetExists() {
		s.sendGuildCommandResult(guildCommandCreate, name, guildErrNameExistsS)
		return nil
	}

	gameClient, err := s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(s.worldSocket.Address())
	if err != nil {
		return fmt.Errorf("can't get gameServiceClient for petition buy: %w", err)
	}

	// Guild charters are sold by tabard designers; other petitioners stay on world.
	tabard, err := gameClient.CanPlayerInteractWithNPC(ctx, &pbWorld.CanPlayerInteractWithNPCRequest{
		Api:        root.SupportedGameServerVer,
		PlayerGuid: s.character.GUID,
		NpcGuid:    npcGUID,
		NpcFlags:   npcFlagGuildCharter,
	})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			s.worldSocket.SendPacket(p)
			return nil
		}
		return fmt.Errorf("can interact tabard designer: %w", err)
	}
	if !tabard.CanInteract {
		// Not a tabard designer in range (or arena petitioner) — world handles it.
		s.worldSocket.SendPacket(p)
		return nil
	}

	money, err := gameClient.GetMoneyForPlayer(ctx, &pbWorld.GetMoneyForPlayerRequest{
		Api:        root.SupportedGameServerVer,
		PlayerGuid: s.character.GUID,
	})
	if err != nil {
		return fmt.Errorf("get money for petition buy: %w", err)
	}
	cost := s.guildCharterCost
	if money.Money < cost {
		return nil
	}

	money, err = gameClient.GetMoneyForPlayer(ctx, &pbWorld.GetMoneyForPlayerRequest{
		Api:        root.SupportedGameServerVer,
		PlayerGuid: s.character.GUID,
	})
	if err != nil {
		return fmt.Errorf("recheck money for petition buy: %w", err)
	}
	if money.Money < cost {
		return nil
	}
	if _, err := gameClient.ModifyMoneyForPlayer(ctx, &pbWorld.ModifyMoneyForPlayerRequest{
		Api:        root.SupportedGameServerVer,
		PlayerGuid: s.character.GUID,
		Value:      -int32(cost),
	}); err != nil {
		return fmt.Errorf("take money for petition buy: %w", err)
	}

	refund := func() {
		_, _ = gameClient.ModifyMoneyForPlayer(ctx, &pbWorld.ModifyMoneyForPlayerRequest{
			Api:        root.SupportedGameServerVer,
			PlayerGuid: s.character.GUID,
			Value:      int32(cost),
		})
	}

	store, err := gameClient.StoreNewItem(ctx, &pbWorld.StoreNewItemRequest{
		Api:        root.SupportedGameServerVer,
		PlayerGuid: s.character.GUID,
		ItemEntry:  guildCharterItemEntry,
		Count:      1,
	})
	if err != nil {
		refund()
		if status.Code(err) == codes.Unimplemented {
			s.worldSocket.SendPacket(p)
			return nil
		}
		return fmt.Errorf("store guild charter: %w", err)
	}
	if store.Status != pbWorld.StoreNewItemResponse_Ok {
		refund()
		return nil
	}

	// AC charter enchantment = item low (petitionguid / client petition id).
	itemLow := uint32(guid.New(store.ItemGuid).GetCounter())
	enchResp, err := gameClient.SetItemPermanentEnchantment(ctx, &pbWorld.SetItemPermanentEnchantmentRequest{
		Api:           root.SupportedGameServerVer,
		PlayerGuid:    s.character.GUID,
		ItemGuid:      store.ItemGuid,
		Slot:          0,
		EnchantmentId: itemLow,
	})
	if err != nil {
		_, _ = gameClient.DestroyItemsWithGuidsFromPlayer(ctx, &pbWorld.DestroyItemsWithGuidsFromPlayerRequest{
			PlayerGuid: s.character.GUID,
			Guids:      []uint64{store.ItemGuid},
		})
		refund()
		if status.Code(err) == codes.Unimplemented {
			s.worldSocket.SendPacket(p)
			return nil
		}
		return fmt.Errorf("set guild charter enchantment: %w", err)
	}
	if enchResp.Status != pbWorld.SetItemPermanentEnchantmentResponse_Ok {
		_, _ = gameClient.DestroyItemsWithGuidsFromPlayer(ctx, &pbWorld.DestroyItemsWithGuidsFromPlayerRequest{
			PlayerGuid: s.character.GUID,
			Guids:      []uint64{store.ItemGuid},
		})
		refund()
		return nil
	}

	if _, err := s.charServiceClient.UpsertGuildPetition(ctx, &pbChar.UpsertGuildPetitionRequest{
		Api:              root.Ver,
		RealmID:          root.RealmID,
		OwnerGUID:        s.character.GUID,
		PetitionItemGUID: store.ItemGuid,
		Name:             name,
	}); err != nil {
		_, _ = gameClient.DestroyItemsWithGuidsFromPlayer(ctx, &pbWorld.DestroyItemsWithGuidsFromPlayerRequest{
			PlayerGuid: s.character.GUID,
			Guids:      []uint64{store.ItemGuid},
		})
		refund()
		return fmt.Errorf("upsert guild petition: %w", err)
	}

	return nil
}

func isValidCharterName(name string) bool {
	// AC IsValidCharterName: 2–24 Unicode code points, letters/digits/spaces.
	n := utf8.RuneCountInString(name)
	if n < 2 || n > 24 {
		return false
	}
	for _, r := range name {
		if r == ' ' {
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func (s *GameSession) worldGameClient() (pbWorld.WorldServerServiceClient, error) {
	if s.gameServerGRPCClient != nil {
		return s.gameServerGRPCClient, nil
	}
	if s.worldSocket == nil {
		return nil, fmt.Errorf("no world socket")
	}
	if s.gameServerGRPCConnMgr == nil {
		return nil, fmt.Errorf("no game server grpc connection manager")
	}
	return s.gameServerGRPCConnMgr.GRPCConnByGameServerAddress(s.worldSocket.Address())
}

// HandleTurnInPetition handles CMSG_TURN_IN_PETITION for guild charters.
// Arena / unknown petitions are forwarded to the worldserver.
func (s *GameSession) HandleTurnInPetition(ctx context.Context, p *packet.Packet) error {
	petitionGUID := p.Reader().Uint64()

	turnIn, err := s.charServiceClient.ValidateGuildPetitionTurnIn(ctx, &pbChar.ValidateGuildPetitionTurnInRequest{
		Api:              root.Ver,
		RealmID:          root.RealmID,
		PetitionItemGUID: petitionGUID,
		PlayerGUID:       s.character.GUID,
	})
	if err != nil {
		return fmt.Errorf("validate guild petition turn-in: %w", err)
	}

	switch turnIn.Status {
	case pbChar.ValidateGuildPetitionTurnInResponse_NotFound, pbChar.ValidateGuildPetitionTurnInResponse_NotGuildPetition:
		s.worldSocket.SendPacket(p)
		return nil
	case pbChar.ValidateGuildPetitionTurnInResponse_AlreadyInGuild:
		s.sendTurnInPetitionResult(petitionTurnAlreadyInGuild)
		return nil
	case pbChar.ValidateGuildPetitionTurnInResponse_NeedMoreSignatures:
		s.sendTurnInPetitionResult(petitionTurnNeedMoreSignatures)
		return nil
	case pbChar.ValidateGuildPetitionTurnInResponse_NotOwner:
		return nil
	case pbChar.ValidateGuildPetitionTurnInResponse_Ok:
	default:
		return nil
	}

	// AC: GetItemByGuid first — no charter in bags means silent ignore.
	gameClient, err := s.worldGameClient()
	if err != nil {
		return fmt.Errorf("world client for turn-in item check: %w", err)
	}
	items, err := gameClient.GetPlayerItemsByGuids(ctx, &pbWorld.GetPlayerItemsByGuidsRequest{
		Api:        root.SupportedGameServerVer,
		PlayerGuid: s.character.GUID,
		Guids:      []uint64{petitionGUID},
	})
	if err != nil {
		return fmt.Errorf("check charter item for turn-in: %w", err)
	}
	if len(items.GetItems()) == 0 {
		return nil
	}

	createResp, err := s.guildServiceClient.CreateGuild(ctx, &pbGuild.CreateGuildParams{
		Api:            root.Ver,
		RealmID:        root.RealmID,
		LeaderGUID:     s.character.GUID,
		Name:           turnIn.GuildName,
		SignatoryGUIDs: turnIn.SignatoryGUIDs,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.AlreadyExists:
			s.sendGuildCommandResult(guildCommandCreate, turnIn.GuildName, guildErrNameExistsS)
			return nil
		case codes.FailedPrecondition:
			s.sendTurnInPetitionResult(petitionTurnAlreadyInGuild)
			return nil
		case codes.InvalidArgument:
			s.sendGuildCommandResult(guildCommandCreate, turnIn.GuildName, guildErrNameInvalid)
			return nil
		}
		return fmt.Errorf("can't create guild, err: %w", err)
	}

	_, _ = s.charServiceClient.DeleteGuildPetition(ctx, &pbChar.DeleteGuildPetitionRequest{
		Api:              root.Ver,
		RealmID:          root.RealmID,
		PetitionItemGUID: petitionGUID,
	})

	// DestroyItems, not RemoveItems(assign=0) — the latter orphans item_instance rows.
	if _, destErr := gameClient.DestroyItemsWithGuidsFromPlayer(ctx, &pbWorld.DestroyItemsWithGuidsFromPlayerRequest{
		PlayerGuid: s.character.GUID,
		Guids:      []uint64{petitionGUID},
	}); destErr != nil {
		s.logger.Error().Err(destErr).Uint64("petitionGUID", petitionGUID).
			Msg("guild created but failed to destroy charter item on player")
	}

	s.character.GuildID = uint32(createResp.GuildID)
	s.character.GuildRank = 0 // GR_GUILDMASTER

	s.sendGuildCommandResult(guildCommandCreate, turnIn.GuildName, guildErrCommandSuccess)
	s.sendTurnInPetitionResult(petitionTurnOK)

	return nil
}

func (s *GameSession) HandleEventPetitionSignResult(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*eBroadcaster.PetitionSignResultPayload)
	s.sendPetitionSignResult(eventData.PetitionItemGUID, eventData.SignerGUID, eventData.Result)
	return nil
}

func (s *GameSession) HandleEventPetitionOffered(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*eBroadcaster.PetitionOfferedPayload)
	s.sendPetitionShowSignatures(eventData.PetitionItemGUID, eventData.OwnerGUID, eventData.PetitionID, eventData.SignatoryGUIDs)
	return nil
}

func (s *GameSession) HandleEventPetitionDeclined(_ context.Context, e *eBroadcaster.Event) error {
	eventData := e.Payload.(*eBroadcaster.PetitionDeclinedPayload)
	w := packet.NewWriterWithSize(packet.MsgPetitionDecline, 8)
	w.Uint64(eventData.SignerGUID)
	s.gameSocket.Send(w)
	return nil
}
