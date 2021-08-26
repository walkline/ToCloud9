package service

import (
	"context"
	"github.com/walkline/ToCloud9/gen/servers-registry/pb"

	"github.com/walkline/ToCloud9/apps/authserver/repo"
)

const (
	RealmFlagNone            = 0x0
	RealmFlagVersionMismatch = 0x1
	RealmFlagOffline         = 0x2
	RealmFlagRecommended     = 0x20
)

type RealmListItem struct {
	ID              uint32
	Name            string
	Address         string
	Icon            uint8
	Flag            uint8
	Timezone        uint8
	Locked          uint8
	CharsCount      uint8
	GameBuild       uint32
	PopulationLevel float32
}

type RealmService interface {
	RealmListForAccount(ctx context.Context, account *repo.Account) ([]RealmListItem, error)
}

type realmServiceImpl struct {
	realmRepo    repo.RealmRepo
	servRegistry pb.ServersRegistryServiceClient
}

func NewRealmService(realmRepo repo.RealmRepo, servRegistry pb.ServersRegistryServiceClient) RealmService {
	return &realmServiceImpl{
		realmRepo:    realmRepo,
		servRegistry: servRegistry,
	}
}

func (r *realmServiceImpl) RealmListForAccount(ctx context.Context, account *repo.Account) ([]RealmListItem, error) {
	realms, err := r.realmRepo.LoadRealms(ctx)
	if err != nil {
		return nil, err
	}

	realmIDs := make([]uint32, 0, len(realms))
	for _, realm := range realms {
		realmIDs = append(realmIDs, realm.ID)
	}

	loadBalancersResp, err := r.servRegistry.LoadBalancerForRealms(ctx, &pb.LoadBalancerForRealmsRequest{
		Api:      "v1.0",
		RealmIDs: realmIDs,
	})
	if err != nil {
		return nil, err
	}

	loadBalancersAddressesMap := map[uint32]string{}
	for _, lb := range loadBalancersResp.LoadBalancers {
		loadBalancersAddressesMap[lb.RealmID] = lb.Address
	}

	chars, err := r.realmRepo.CountCharsPerRealmByAccountID(ctx, account.ID)
	if err != nil {
		return nil, err
	}

	realmCharsMap := map[uint32]uint8{}
	for _, char := range chars {
		realmCharsMap[char.RealmID] = char.CharCount
	}

	result := []RealmListItem{}
	for _, realm := range realms {
		address, found := loadBalancersAddressesMap[realm.ID]
		if !found {
			realm.Flag |= RealmFlagOffline
		} else {
			realm.Flag &^= RealmFlagOffline
		}

		result = append(result, RealmListItem{
			ID:         realm.ID,
			Name:       realm.Name,
			Address:    address,
			Icon:       realm.Icon,
			Flag:       realm.Flag,
			Timezone:   realm.Timezone,
			Locked:     0,
			CharsCount: realmCharsMap[realm.ID],
			GameBuild:  realm.GameBuild,
		})
	}

	return result, nil
}
