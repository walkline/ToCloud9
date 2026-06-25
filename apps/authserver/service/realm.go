package service

import (
	"context"
	"time"

	"github.com/walkline/ToCloud9/apps/authserver/repo"
	"github.com/walkline/ToCloud9/gen/servers-registry/pb"
)

const (
	RealmFlagNone            = 0x0
	RealmFlagVersionMismatch = 0x1
	RealmFlagOffline         = 0x2
	RealmFlagRecommended     = 0x20

	offlineRealmAddress = "0.0.0.0:0"

	realmGatewayLookupTimeout = 2 * time.Second
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

	gatewayCtx := ctx
	var cancel context.CancelFunc
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > realmGatewayLookupTimeout {
		gatewayCtx, cancel = context.WithTimeout(ctx, realmGatewayLookupTimeout)
		defer cancel()
	}

	gatewaysResp, err := r.servRegistry.GatewaysForRealms(gatewayCtx, &pb.GatewaysForRealmsRequest{
		Api:      "v1.0",
		RealmIDs: realmIDs,
	})

	gatewaysAddressesMap := map[uint32]string{}
	if err == nil && gatewaysResp != nil {
		for _, lb := range gatewaysResp.Gateways {
			gatewaysAddressesMap[lb.RealmID] = lb.Address
		}
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
		address, found := gatewaysAddressesMap[realm.ID]
		if !found || address == "" {
			realm.Flag |= RealmFlagOffline
			address = offlineRealmAddress
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
