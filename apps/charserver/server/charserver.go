package server

import (
	"context"

	"github.com/walkline/ToCloud9/apps/charserver/repo"
	"github.com/walkline/ToCloud9/gen/characters/pb"
)

const (
	ver = "0.0.1"
)

type CharServer struct {
	repo          repo.Characters
	itemsTemplate repo.ItemsTemplate
}

func NewCharServer(repo repo.Characters, itemsTemplate repo.ItemsTemplate) pb.CharactersServiceServer {
	return &CharServer{
		repo:          repo,
		itemsTemplate: itemsTemplate,
	}
}

func (c CharServer) CharactersToLoginForAccount(ctx context.Context, request *pb.CharactersToLoginForAccountRequest) (*pb.CharactersToLoginForAccountResponse, error) {
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

func (c CharServer) AccountDataForAccount(ctx context.Context, request *pb.AccountDataForAccountRequest) (*pb.AccountDataForAccountResponse, error) {
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

func (c CharServer) CharactersToLoginByGUID(ctx context.Context, request *pb.CharactersToLoginByGUIDRequest) (*pb.CharactersToLoginByGUIDResponse, error) {
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
