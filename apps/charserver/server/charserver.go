package server

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/apps/charserver/service"
	"github.com/walkline/ToCloud9/gen/characters/pb"
)

const (
	ver = "0.0.1"
)

type CharServer struct {
	pb.UnimplementedCharactersServiceServer
	repo            repo.Characters
	whoHandler      repo.WhoHandler
	itemsTemplate   repo.ItemsTemplate
	onlineChars     repo.CharactersOnline
	friendsService  service.FriendsService
	petitionService service.PetitionService
	guildNames      service.GuildNameResolver
}

func NewCharServer(repo repo.Characters, onlineChars repo.CharactersOnline, whoHandler repo.WhoHandler, itemsTemplate repo.ItemsTemplate, friendsService service.FriendsService, petitionService service.PetitionService, guildNames service.GuildNameResolver) pb.CharactersServiceServer {
	return &CharServer{
		repo:            repo,
		whoHandler:      whoHandler,
		itemsTemplate:   itemsTemplate,
		onlineChars:     onlineChars,
		friendsService:  friendsService,
		petitionService: petitionService,
		guildNames:      guildNames,
	}
}

func (c *CharServer) CharactersToLoginForAccount(ctx context.Context, request *pb.CharactersToLoginForAccountRequest) (*pb.CharactersToLoginForAccountResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint32("accountID", request.AccountID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled characters to login request")
	}(time.Now())

	chars, err := c.repo.ListCharactersToLogIn(ctx, request.RealmID, request.AccountID)
	if err != nil {
		return nil, err
	}

	result := make([]*pb.LogInCharacter, 0, len(chars))
	for _, char := range chars {
		equipments := []*pb.EquipmentDisplay{}
		for _, itemID := range char.Equipments {
			if itemID == 0 {
				equipments = append(equipments, &pb.EquipmentDisplay{})
				continue
			}

			item, err := c.itemsTemplate.TemplateByID(itemID)
			if err != nil {
				return nil, err
			}

			equipments = append(equipments, &pb.EquipmentDisplay{
				DisplayInfoID: item.DisplayID,
				InventoryType: uint32(item.InventoryType),
				EnchantmentID: 0,
			})
		}

		item := &pb.LogInCharacter{
			GUID:        char.GUID,
			Name:        char.Name,
			Race:        uint32(char.Race),
			Class:       uint32(char.Class),
			Gender:      uint32(char.Gender),
			Skin:        uint32(char.Skin),
			Face:        uint32(char.Face),
			HairStyle:   uint32(char.HairStyle),
			HairColor:   uint32(char.HairColor),
			FacialStyle: uint32(char.FacialStyle),
			Level:       uint32(char.Level),
			Zone:        char.Zone,
			Map:         char.Map,
			PositionX:   char.PositionX,
			PositionY:   char.PositionY,
			PositionZ:   char.PositionZ,
			GuildID:     char.GuildID,
			GuildRank:   uint32(char.GuildRank),
			PlayerFlags: char.PlayerFlags,
			AtLogin:     uint32(char.AtLoginFlags),
			PetEntry:    char.PetEntry,
			PetModelID:  char.PetModelID,
			PetLevel:    uint32(char.PetLevel),
			Equipments:  equipments,
			Banned:      char.Banned,
			AccountID:   char.AccountID,
		}
		result = append(result, item)
	}

	return &pb.CharactersToLoginForAccountResponse{
		Api:        ver,
		Characters: result,
	}, nil
}

func (c *CharServer) AccountDataForAccount(ctx context.Context, request *pb.AccountDataForAccountRequest) (*pb.AccountDataForAccountResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint32("accountID", request.AccountID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled account data request")
	}(time.Now())

	accountData, err := c.repo.AccountDataForAccountID(ctx, request.RealmID, request.AccountID)
	if err != nil {
		return nil, err
	}

	res := make([]*pb.AccountData, 0, len(accountData))
	for _, item := range accountData {
		res = append(res, &pb.AccountData{
			Type: uint32(item.Type),
			Time: item.Time,
			Data: item.Data,
		})
	}

	return &pb.AccountDataForAccountResponse{
		Api:         ver,
		AccountData: res,
	}, nil
}

func (c *CharServer) CharactersToLoginByGUID(ctx context.Context, request *pb.CharactersToLoginByGUIDRequest) (*pb.CharactersToLoginByGUIDResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("characterID", request.CharacterGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled characters to login by GUID")
	}(time.Now())

	char, err := c.repo.CharacterToLogInByGUID(ctx, request.RealmID, request.CharacterGUID)
	if err != nil {
		return nil, err
	}
	var charResult *pb.LogInCharacter
	if char != nil {
		equipments := []*pb.EquipmentDisplay{}
		for _, itemID := range char.Equipments {
			if itemID == 0 {
				equipments = append(equipments, &pb.EquipmentDisplay{})
				continue
			}

			item, err := c.itemsTemplate.TemplateByID(itemID)
			if err != nil {
				return nil, err
			}

			equipments = append(equipments, &pb.EquipmentDisplay{
				DisplayInfoID: item.DisplayID,
				InventoryType: uint32(item.InventoryType),
				EnchantmentID: 0,
			})
		}

		charResult = &pb.LogInCharacter{
			GUID:        char.GUID,
			Name:        char.Name,
			Race:        uint32(char.Race),
			Class:       uint32(char.Class),
			Gender:      uint32(char.Gender),
			Skin:        uint32(char.Skin),
			Face:        uint32(char.Face),
			HairStyle:   uint32(char.HairStyle),
			HairColor:   uint32(char.HairColor),
			FacialStyle: uint32(char.FacialStyle),
			Level:       uint32(char.Level),
			Zone:        char.Zone,
			Map:         char.Map,
			PositionX:   char.PositionX,
			PositionY:   char.PositionY,
			PositionZ:   char.PositionZ,
			GuildID:     char.GuildID,
			GuildRank:   uint32(char.GuildRank),
			PlayerFlags: char.PlayerFlags,
			AtLogin:     uint32(char.AtLoginFlags),
			PetEntry:    char.PetEntry,
			PetModelID:  char.PetModelID,
			PetLevel:    uint32(char.PetLevel),
			Equipments:  equipments,
			Banned:      char.Banned,
			AccountID:   char.AccountID,
		}
	}

	return &pb.CharactersToLoginByGUIDResponse{
		Api:       ver,
		Character: charResult,
	}, nil
}

func (c *CharServer) WhoQuery(ctx context.Context, request *pb.WhoQueryRequest) (*pb.WhoQueryResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("characterID", request.CharacterGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled who query")
	}(time.Now())

	chars, err := c.whoHandler.WhoRequest(ctx, request.RealmID, request.CharacterGUID, repo.CharactersWhoQuery{
		LvlMin:    uint8(request.LvlMin),
		LvlMax:    uint8(request.LvlMax),
		ClassMask: request.ClassMask,
		RaceMask:  request.RaceMask,
		Zones:     request.Zones,
		Strings:   request.Strings,
	})
	if err != nil {
		return nil, err
	}
	count := len(chars)
	if count > 50 {
		count = 50
	}

	// Guild ids are snapshotted at login, a guild joined mid-session shows up after relog.
	guildIDs := make([]uint32, 0, count)
	for i := 0; i < count; i++ {
		if chars[i].CharGuildID != 0 {
			guildIDs = append(guildIDs, chars[i].CharGuildID)
		}
	}

	guildNames := map[uint32]string{}
	if len(guildIDs) > 0 {
		guildNames, err = c.guildNames.GuildNamesByIDs(ctx, request.RealmID, guildIDs)
		if err != nil {
			log.Warn().Err(err).Msg("can't resolve guild names for who query")
			guildNames = map[uint32]string{}
		}
	}

	items := make([]*pb.WhoQueryResponse_WhoItem, 0, 50)
	for i := 0; i < count; i++ {
		items = append(items, &pb.WhoQueryResponse_WhoItem{
			Guid:   chars[i].CharGUID,
			Name:   chars[i].CharName,
			Guild:  guildNames[chars[i].CharGuildID],
			Lvl:    uint32(chars[i].CharLevel),
			Class:  uint32(chars[i].CharClass),
			Race:   uint32(chars[i].CharRace),
			Gender: uint32(chars[i].CharGender),
			ZoneID: chars[i].CharZone,
		})
	}

	return &pb.WhoQueryResponse{
		Api:            ver,
		TotalFound:     uint32(len(chars)),
		ItemsToDisplay: items,
	}, nil
}

func (c *CharServer) CharacterOnlineByName(ctx context.Context, request *pb.CharacterOnlineByNameRequest) (*pb.CharacterOnlineByNameResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Str("name", request.CharacterName).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled character online by name")
	}(time.Now())

	char, err := c.onlineChars.OneByRealmAndName(ctx, request.RealmID, request.CharacterName)
	if err != nil {
		return nil, err
	}

	if char == nil {
		return &pb.CharacterOnlineByNameResponse{
			Api:       ver,
			Character: nil,
		}, nil
	}

	return &pb.CharacterOnlineByNameResponse{
		Api: ver,
		Character: &pb.CharacterOnlineByNameResponse_Char{
			RealmID:     char.RealmID,
			GatewayID:   char.GatewayID,
			CharGUID:    char.CharGUID,
			CharName:    char.CharName,
			CharRace:    uint32(char.CharRace),
			CharClass:   uint32(char.CharClass),
			CharGender:  uint32(char.CharGender),
			CharLvl:     uint32(char.CharLevel),
			CharZone:    char.CharZone,
			CharMap:     char.CharMap,
			CharGuildID: uint64(char.CharGuildID),
			AccountID:   char.AccountID,
		},
	}, nil
}

func (c *CharServer) CharacterByName(ctx context.Context, request *pb.CharacterByNameRequest) (*pb.CharacterByNameResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Str("name", request.CharacterName).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled character by name")
	}(time.Now())

	char, err := c.onlineChars.OneByRealmAndName(ctx, request.RealmID, request.CharacterName)
	if err != nil {
		return nil, err
	}

	if char != nil {
		return &pb.CharacterByNameResponse{
			Api: ver,
			Character: &pb.CharacterByNameResponse_Char{
				RealmID:     char.RealmID,
				IsOnline:    true,
				GatewayID:   char.GatewayID,
				CharGUID:    char.CharGUID,
				CharName:    char.CharName,
				CharRace:    uint32(char.CharRace),
				CharClass:   uint32(char.CharClass),
				CharGender:  uint32(char.CharGender),
				CharLvl:     uint32(char.CharLevel),
				CharZone:    char.CharZone,
				CharMap:     char.CharMap,
				CharGuildID: uint64(char.CharGuildID),
				AccountID:   char.AccountID,
			},
		}, nil
	}

	char, err = c.repo.CharacterByName(ctx, request.RealmID, request.CharacterName)
	if err != nil {
		return nil, err
	}

	if char == nil {
		return &pb.CharacterByNameResponse{
			Api:       ver,
			Character: nil,
		}, nil
	}

	return &pb.CharacterByNameResponse{
		Api: ver,
		Character: &pb.CharacterByNameResponse_Char{
			RealmID:     char.RealmID,
			IsOnline:    false,
			GatewayID:   "",
			CharGUID:    char.CharGUID,
			CharName:    char.CharName,
			CharRace:    uint32(char.CharRace),
			CharClass:   uint32(char.CharClass),
			CharGender:  uint32(char.CharGender),
			CharLvl:     uint32(char.CharLevel),
			CharZone:    char.CharZone,
			CharMap:     char.CharMap,
			CharGuildID: uint64(char.CharGuildID),
			AccountID:   char.AccountID,
		},
	}, nil
}

func (c *CharServer) ShortOnlineCharactersDataByGUIDs(ctx context.Context, request *pb.ShortCharactersDataByGUIDsRequest) (*pb.ShortCharactersDataByGUIDsResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Int("guidsSize", len(request.GUIDs)).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled short characters by guids")
	}(time.Now())

	chars, err := c.onlineChars.CharactersByRealmAndGUIDs(ctx, request.RealmID, request.GUIDs)
	if err != nil {
		return nil, err
	}

	res := make([]*pb.ShortCharactersDataByGUIDsResponse_ShortCharData, len(chars))
	for i, char := range chars {
		res[i] = &pb.ShortCharactersDataByGUIDsResponse_ShortCharData{
			RealmID:     char.RealmID,
			IsOnline:    true,
			GatewayID:   char.GatewayID,
			CharGUID:    char.CharGUID,
			CharName:    char.CharName,
			CharRace:    uint32(char.CharRace),
			CharClass:   uint32(char.CharClass),
			CharGender:  uint32(char.CharGender),
			CharLvl:     uint32(char.CharLevel),
			CharZone:    char.CharZone,
			CharMap:     char.CharMap,
			CharGuildID: uint64(char.CharGuildID),
			AccountID:   char.AccountID,
		}
	}

	return &pb.ShortCharactersDataByGUIDsResponse{
		Api:        ver,
		Characters: res,
	}, nil
}

func (c *CharServer) SavePlayerPosition(ctx context.Context, request *pb.SavePlayerPositionRequest) (*pb.SavePlayerPositionResponse, error) {
	err := c.repo.SaveCharacterPosition(ctx, request.RealmID, request.CharGUID, request.MapID, request.X, request.Y, request.Z, request.O)
	if err != nil {
		return nil, err
	}

	return &pb.SavePlayerPositionResponse{
		Api: ver,
	}, nil
}

func (c *CharServer) GetFriendsList(ctx context.Context, request *pb.GetFriendsListRequest) (*pb.GetFriendsListResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled get friends list")
	}(time.Now())

	friendsList, err := c.friendsService.GetFriendsList(ctx, request.RealmID, request.PlayerGUID)
	if err != nil {
		return nil, err
	}

	friends := make([]*pb.GetFriendsListResponse_Friend, 0, len(friendsList.Friends))
	for _, friend := range friendsList.Friends {
		friends = append(friends, &pb.GetFriendsListResponse_Friend{
			Guid:    friend.GUID,
			Note:    friend.Note,
			Status:  uint32(friend.Status),
			Area:    friend.Area,
			Level:   friend.Level,
			ClassID: friend.ClassID,
		})
	}

	ignored := make([]*pb.GetFriendsListResponse_IgnoredPlayer, 0, len(friendsList.Ignored))
	for _, guid := range friendsList.Ignored {
		ignored = append(ignored, &pb.GetFriendsListResponse_IgnoredPlayer{
			Guid: guid,
		})
	}

	return &pb.GetFriendsListResponse{
		Api:     ver,
		Friends: friends,
		Ignored: ignored,
	}, nil
}

func (c *CharServer) AddFriend(ctx context.Context, request *pb.AddFriendRequest) (*pb.AddFriendResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint64("friendGUID", request.FriendGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled add friend")
	}(time.Now())

	result, err := c.friendsService.AddFriend(ctx, request.RealmID, request.PlayerGUID, request.FriendGUID, request.FriendName, request.Note)
	if err != nil {
		return nil, err
	}

	return &pb.AddFriendResponse{
		Api:     ver,
		Result:  result.Result,
		Status:  uint32(result.Status),
		Area:    result.Area,
		Level:   result.Level,
		ClassID: result.ClassID,
	}, nil
}

func (c *CharServer) RemoveFriend(ctx context.Context, request *pb.RemoveFriendRequest) (*pb.RemoveFriendResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint64("friendGUID", request.FriendGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled remove friend")
	}(time.Now())

	err := c.friendsService.RemoveFriend(ctx, request.RealmID, request.PlayerGUID, request.FriendGUID)
	if err != nil {
		return nil, err
	}

	return &pb.RemoveFriendResponse{
		Api: ver,
	}, nil
}

func (c *CharServer) SetFriendNote(ctx context.Context, request *pb.SetFriendNoteRequest) (*pb.SetFriendNoteResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint64("friendGUID", request.FriendGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled set friend note")
	}(time.Now())

	err := c.friendsService.SetFriendNote(ctx, request.RealmID, request.PlayerGUID, request.FriendGUID, request.Note)
	if err != nil {
		return nil, err
	}

	return &pb.SetFriendNoteResponse{
		Api: ver,
	}, nil
}

func (c *CharServer) AddIgnore(ctx context.Context, request *pb.AddIgnoreRequest) (*pb.AddIgnoreResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint64("ignoredGUID", request.IgnoredGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled add ignore")
	}(time.Now())

	result, err := c.friendsService.AddIgnore(ctx, request.RealmID, request.PlayerGUID, request.IgnoredGUID)
	if err != nil {
		return nil, err
	}

	return &pb.AddIgnoreResponse{
		Api:    ver,
		Result: result,
	}, nil
}

func (c *CharServer) RemoveIgnore(ctx context.Context, request *pb.RemoveIgnoreRequest) (*pb.RemoveIgnoreResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint64("ignoredGUID", request.IgnoredGUID).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled remove ignore")
	}(time.Now())

	err := c.friendsService.RemoveIgnore(ctx, request.RealmID, request.PlayerGUID, request.IgnoredGUID)
	if err != nil {
		return nil, err
	}

	return &pb.RemoveIgnoreResponse{
		Api: ver,
	}, nil
}

func (c *CharServer) NotifyStatusChange(ctx context.Context, request *pb.NotifyStatusChangeRequest) (*pb.NotifyStatusChangeResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint64("playerGUID", request.PlayerGUID).
			Uint32("status", request.Status).
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled notify status change")
	}(time.Now())

	err := c.friendsService.NotifyStatusChange(ctx, request.RealmID, request.PlayerGUID, uint8(request.Status), request.Area, request.Level, request.ClassID)
	if err != nil {
		return nil, err
	}

	return &pb.NotifyStatusChangeResponse{
		Api: ver,
	}, nil
}

func (c *CharServer) GetOnlineCharacters(ctx context.Context, request *pb.GetOnlineCharactersRequest) (*pb.GetOnlineCharactersResponse, error) {
	defer func(t time.Time) {
		log.Debug().
			Uint32("realmID", request.RealmID).
			Str("timeTook", time.Since(t).String()).
			Msg("Handled get online characters")
	}(time.Now())

	guids, err := c.onlineChars.AllGUIDsByRealm(ctx, request.RealmID)
	if err != nil {
		return nil, err
	}

	return &pb.GetOnlineCharactersResponse{
		Api:             ver,
		CharacterGUIDs:  guids,
		TotalCount:      uint32(len(guids)),
	}, nil
}

func (c *CharServer) GetGuildPetition(ctx context.Context, request *pb.GetGuildPetitionRequest) (*pb.GetGuildPetitionResponse, error) {
	p, err := c.petitionService.GetPetition(ctx, request.RealmID, request.PetitionItemGUID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return &pb.GetGuildPetitionResponse{Api: ver, Status: pb.GetGuildPetitionResponse_NotFound}, nil
	}
	if p.Type != repo.GuildCharterType {
		return &pb.GetGuildPetitionResponse{Api: ver, Status: pb.GetGuildPetitionResponse_NotGuildPetition}, nil
	}
	return &pb.GetGuildPetitionResponse{
		Api:      ver,
		Status:   pb.GetGuildPetitionResponse_Ok,
		Petition: toPBPetition(request.PetitionItemGUID, p, 0),
	}, nil
}

func (c *CharServer) GetGuildPetitionSignatures(ctx context.Context, request *pb.GetGuildPetitionSignaturesRequest) (*pb.GetGuildPetitionSignaturesResponse, error) {
	p, sigs, err := c.petitionService.GetSignatures(ctx, request.RealmID, request.PetitionItemGUID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return &pb.GetGuildPetitionSignaturesResponse{Api: ver, Status: pb.GetGuildPetitionSignaturesResponse_NotFound}, nil
	}
	if p.Type != repo.GuildCharterType {
		return &pb.GetGuildPetitionSignaturesResponse{Api: ver, Status: pb.GetGuildPetitionSignaturesResponse_NotGuildPetition}, nil
	}

	pbSigs := make([]*pb.GuildPetitionSignature, 0, len(sigs))
	for _, sig := range sigs {
		pbSigs = append(pbSigs, &pb.GuildPetitionSignature{
			PlayerGUID: sig.PlayerGUID,
			AccountID:  sig.AccountID,
		})
	}
	return &pb.GetGuildPetitionSignaturesResponse{
		Api:        ver,
		Status:     pb.GetGuildPetitionSignaturesResponse_Ok,
		Petition:   toPBPetition(request.PetitionItemGUID, p, uint32(len(sigs))),
		Signatures: pbSigs,
	}, nil
}

func (c *CharServer) AddGuildPetitionSignature(ctx context.Context, request *pb.AddGuildPetitionSignatureRequest) (*pb.AddGuildPetitionSignatureResponse, error) {
	result, ownerGUID, err := c.petitionService.AddSignature(ctx, request.RealmID, request.PetitionItemGUID, request.SignerGUID, request.SignerAccountID)
	if err != nil {
		return nil, err
	}
	return &pb.AddGuildPetitionSignatureResponse{
		Api:              ver,
		Result:           pb.AddGuildPetitionSignatureResponse_Result(result),
		OwnerGUID:        ownerGUID,
		PetitionItemGUID: request.PetitionItemGUID,
		SignerGUID:       request.SignerGUID,
	}, nil
}

func (c *CharServer) RenameGuildPetition(ctx context.Context, request *pb.RenameGuildPetitionRequest) (*pb.RenameGuildPetitionResponse, error) {
	ok, notGuild, notOwner, err := c.petitionService.Rename(ctx, request.RealmID, request.PetitionItemGUID, request.OwnerGUID, request.NewName)
	if err != nil {
		return nil, err
	}
	status := pb.RenameGuildPetitionResponse_NotFound
	switch {
	case ok:
		status = pb.RenameGuildPetitionResponse_Ok
	case notGuild:
		status = pb.RenameGuildPetitionResponse_NotGuildPetition
	case notOwner:
		status = pb.RenameGuildPetitionResponse_NotOwner
	}
	return &pb.RenameGuildPetitionResponse{Api: ver, Status: status}, nil
}

func (c *CharServer) DeleteGuildPetition(ctx context.Context, request *pb.DeleteGuildPetitionRequest) (*pb.DeleteGuildPetitionResponse, error) {
	found, err := c.petitionService.Delete(ctx, request.RealmID, request.PetitionItemGUID)
	if err != nil {
		return nil, err
	}
	status := pb.DeleteGuildPetitionResponse_Ok
	if !found {
		status = pb.DeleteGuildPetitionResponse_NotFound
	}
	return &pb.DeleteGuildPetitionResponse{Api: ver, Status: status}, nil
}

func (c *CharServer) ValidateGuildPetitionTurnIn(ctx context.Context, request *pb.ValidateGuildPetitionTurnInRequest) (*pb.ValidateGuildPetitionTurnInResponse, error) {
	status, name, signatories, err := c.petitionService.ValidateTurnIn(ctx, request.RealmID, request.PetitionItemGUID, request.PlayerGUID)
	if err != nil {
		return nil, err
	}

	var pbStatus pb.ValidateGuildPetitionTurnInResponse_Status
	switch status {
	case service.TurnInOK:
		pbStatus = pb.ValidateGuildPetitionTurnInResponse_Ok
	case service.TurnInNotFound:
		pbStatus = pb.ValidateGuildPetitionTurnInResponse_NotFound
	case service.TurnInNotOwner:
		pbStatus = pb.ValidateGuildPetitionTurnInResponse_NotOwner
	case service.TurnInNotGuild:
		pbStatus = pb.ValidateGuildPetitionTurnInResponse_NotGuildPetition
	case service.TurnInNeedMoreSignatures:
		pbStatus = pb.ValidateGuildPetitionTurnInResponse_NeedMoreSignatures
	case service.TurnInAlreadyInGuild:
		pbStatus = pb.ValidateGuildPetitionTurnInResponse_AlreadyInGuild
	default:
		pbStatus = pb.ValidateGuildPetitionTurnInResponse_NotFound
	}

	return &pb.ValidateGuildPetitionTurnInResponse{
		Api:            ver,
		Status:         pbStatus,
		GuildName:      name,
		SignatoryGUIDs: signatories,
	}, nil
}

func (c *CharServer) UpsertGuildPetition(ctx context.Context, request *pb.UpsertGuildPetitionRequest) (*pb.UpsertGuildPetitionResponse, error) {
	err := c.petitionService.UpsertGuildPetition(ctx, request.RealmID, request.OwnerGUID, request.PetitionItemGUID, request.Name)
	if err != nil {
		return &pb.UpsertGuildPetitionResponse{Api: ver, Status: pb.UpsertGuildPetitionResponse_Failed}, err
	}
	return &pb.UpsertGuildPetitionResponse{Api: ver, Status: pb.UpsertGuildPetitionResponse_Ok}, nil
}

func (c *CharServer) GuildNameExists(ctx context.Context, request *pb.GuildNameExistsRequest) (*pb.GuildNameExistsResponse, error) {
	exists, err := c.petitionService.GuildNameExists(ctx, request.RealmID, request.Name)
	if err != nil {
		return nil, err
	}
	return &pb.GuildNameExistsResponse{Api: ver, Exists: exists}, nil
}

func toPBPetition(itemGUID uint64, p *repo.Petition, signatureCount uint32) *pb.GuildPetition {
	return &pb.GuildPetition{
		PetitionItemGUID: itemGUID,
		PetitionItemLow:  p.ItemLow,
		PetitionID:       p.ItemLow,
		OwnerGUID:        p.OwnerGUID,
		Name:             p.Name,
		Type:             uint32(p.Type),
		SignatureCount:   signatureCount,
	}
}
