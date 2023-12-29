package main

import "C"
import (
	"context"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	"github.com/walkline/ToCloud9/game-server/libsidecar/guids"
	guidPB "github.com/walkline/ToCloud9/gen/guid/pb"
)

// TC9GetNextAvailableCharacterGuid returns next available characters GUID. Thread unsafe.
//
//export TC9GetNextAvailableCharacterGuid
func TC9GetNextAvailableCharacterGuid() uint64 {
	return charactersGuidsIterator.Next()
}

// TC9GetNextAvailableItemGuid returns next available item GUID. Thread unsafe.
//
//export TC9GetNextAvailableItemGuid
func TC9GetNextAvailableItemGuid() uint64 {
	return itemsGuidsIterator.Next()
}

// TC9GetNextAvailableInstanceGuid returns next available dungeon/raid instance GUID. Thread unsafe.
//
//export TC9GetNextAvailableInstanceGuid
func TC9GetNextAvailableInstanceGuid() uint64 {
	return instancesGuidsIterator.Next()
}

var charactersGuidsIterator guids.GuidProvider
var itemsGuidsIterator guids.GuidProvider
var instancesGuidsIterator guids.GuidProvider

func SetupGuidProviders(realmID uint32, cfg *config.Config) {
	// pctToTriggerUpdate is percent of used guids to trigger
	// request to add new guids to the pool.
	const pctToTriggerUpdate float32 = 65

	var err error
	charactersGuidsIterator, err = guids.NewThreadUnsafeGuidProvider(
		context.Background(),
		guids.NewCharactersGRPCDiapasonsProvider(guidServiceClient, realmID, uint64(cfg.CharacterGuidsBufferSize)),
		pctToTriggerUpdate,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create characters guid provider")
	}

	itemsGuidsIterator, err = guids.NewThreadUnsafeGuidProvider(
		context.Background(),
		guids.NewItemsGRPCDiapasonsProvider(guidServiceClient, realmID, uint64(cfg.ItemGuidsBufferSize)),
		pctToTriggerUpdate,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create items guid provider")
	}

	instancesGuidsIterator, err = guids.NewThreadUnsafeGuidProvider(
		context.Background(),
		guids.NewInstancesGRPCDiapasonsProvider(guidServiceClient, realmID, uint64(cfg.InstanceGuidsBufferSize)),
		pctToTriggerUpdate,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create items guid provider")
	}
}

var guidServiceClient guidPB.GuidServiceClient

func SetupGuidServiceConnection(cfg *config.Config) *grpc.ClientConn {
	conn, err := grpc.Dial(cfg.GuidProviderServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the guid provider service")
	}

	guidServiceClient = guidPB.NewGuidServiceClient(conn)

	return conn
}
