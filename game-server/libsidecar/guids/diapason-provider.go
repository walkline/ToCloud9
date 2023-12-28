package guids

import (
	"context"
	"fmt"

	"github.com/walkline/ToCloud9/gen/guid/pb"
)

type diapason struct {
	start uint64
	end   uint64
}

type DiapasonsProvider interface {
	NewDiapasons(ctx context.Context) ([]diapason, error)
}

func NewCharactersGRPCDiapasonsProvider(client pb.GuidServiceClient, realmID uint32, desiredPoolSize uint64) DiapasonsProvider {
	return NewGRPCDiapasonsProvider(client, realmID, pb.GuidType_Character, desiredPoolSize)
}

func NewItemsGRPCDiapasonsProvider(client pb.GuidServiceClient, realmID uint32, desiredPoolSize uint64) DiapasonsProvider {
	return NewGRPCDiapasonsProvider(client, realmID, pb.GuidType_Item, desiredPoolSize)
}

func NewInstancesGRPCDiapasonsProvider(client pb.GuidServiceClient, realmID uint32, desiredPoolSize uint64) DiapasonsProvider {
	return NewGRPCDiapasonsProvider(client, realmID, pb.GuidType_Instance, desiredPoolSize)
}

func NewGRPCDiapasonsProvider(client pb.GuidServiceClient, realmID uint32, guidType pb.GuidType, desiredPoolSize uint64) DiapasonsProvider {
	f := func(ctx context.Context) ([]diapason, error) {
		resp, err := client.GetGUIDPool(ctx, &pb.GetGUIDPoolRequest{
			Api:             "", // TODO: add supported version here.
			RealmID:         realmID,
			GuidType:        guidType,
			DesiredPoolSize: desiredPoolSize,
		})
		if err != nil {
			return nil, fmt.Errorf("can't fetch guid diapasons for type %d, err: %w", guidType, err)
		}

		res := make([]diapason, len(resp.ReceiverGUID))
		for i := range resp.ReceiverGUID {
			res[i].start = resp.ReceiverGUID[i].Start
			res[i].end = resp.ReceiverGUID[i].End
		}

		return res, nil
	}
	return diapasonsProviderFunc(f)
}

type diapasonsProviderFunc func(ctx context.Context) ([]diapason, error)

func (f diapasonsProviderFunc) NewDiapasons(ctx context.Context) ([]diapason, error) {
	return f(ctx)
}
