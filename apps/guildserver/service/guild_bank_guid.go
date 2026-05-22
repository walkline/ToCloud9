package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/walkline/ToCloud9/apps/guildserver"
	guidpb "github.com/walkline/ToCloud9/gen/guid/pb"
)

var errGuildBankItemGUIDAllocatorMissing = errors.New("guild bank item GUID allocator missing")

// ItemGUIDAllocator allocates native item_instance GUIDs for DB-side stack splits.
type ItemGUIDAllocator interface {
	NextItemGUID(ctx context.Context, realmID uint32) (uint64, error)
}

type guidServiceItemGUIDAllocator struct {
	client guidpb.GuidServiceClient
}

func NewGuidServiceItemGUIDAllocator(client guidpb.GuidServiceClient) ItemGUIDAllocator {
	return &guidServiceItemGUIDAllocator{client: client}
}

func (a *guidServiceItemGUIDAllocator) NextItemGUID(ctx context.Context, realmID uint32) (uint64, error) {
	resp, err := a.client.GetGUIDPool(ctx, &guidpb.GetGUIDPoolRequest{
		Api:             guildserver.Ver,
		RealmID:         realmID,
		GuidType:        guidpb.GuidType_Item,
		DesiredPoolSize: 1,
	})
	if err != nil {
		return 0, fmt.Errorf("get item GUID pool: %w", err)
	}
	if len(resp.GetReceiverGUID()) == 0 || resp.GetReceiverGUID()[0].GetStart() == 0 || resp.GetReceiverGUID()[0].GetStart() > resp.GetReceiverGUID()[0].GetEnd() {
		return 0, errors.New("guid service returned empty item GUID pool")
	}
	return resp.GetReceiverGUID()[0].GetStart(), nil
}
