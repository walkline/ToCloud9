package main

import "C"

import (
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	"github.com/walkline/ToCloud9/game-server/libsidecar/guids"
	guidPB "github.com/walkline/ToCloud9/gen/guid/pb"
)

// TC9GetNextAvailableCharacterGuid returns next available characters GUID. Thread unsafe.
//
//export TC9GetNextAvailableCharacterGuid
func TC9GetNextAvailableCharacterGuid(realmID int) uint64 {
	if realmID == 0 {
		realmID = int(RealmID)
	}
	return charactersGuidsIterator.Next(uint32(realmID))
}

// TC9GetNextAvailableItemGuid returns next available item GUID. Thread unsafe.
//
//export TC9GetNextAvailableItemGuid
func TC9GetNextAvailableItemGuid(realmID int) uint64 {
	if realmID == 0 {
		realmID = int(RealmID)
	}
	return itemsGuidsIterator.Next(uint32(realmID))
}

// TC9GetNextAvailableInstanceGuid returns next available dungeon/raid instance GUID. Thread unsafe.
//
//export TC9GetNextAvailableInstanceGuid
func TC9GetNextAvailableInstanceGuid(realmID int) uint64 {
	if realmID == 0 {
		realmID = int(RealmID)
	}
	return instancesGuidsIterator.Next(uint32(realmID))
}

var charactersGuidsIterator *guids.CrossrealmMgr

var itemsGuidsIterator *guids.CrossrealmMgr

var instancesGuidsIterator *guids.CrossrealmMgr

func SetupGuidProviders(realmID uint32, cfg *config.Config) {
	// pctToTriggerUpdate is percent of used guids to trigger
	// request to add new guids to the pool.
	const pctToTriggerUpdate float32 = 65

	charactersGuidsIterator = guids.NewCrossRealmMgr(guidServiceClient, guidPB.GuidType_Character, uint64(cfg.CharacterGuidsBufferSize), pctToTriggerUpdate, realmID)
	itemsGuidsIterator = guids.NewCrossRealmMgr(guidServiceClient, guidPB.GuidType_Item, uint64(cfg.ItemGuidsBufferSize), pctToTriggerUpdate, realmID)
	instancesGuidsIterator = guids.NewCrossRealmMgr(guidServiceClient, guidPB.GuidType_Instance, uint64(cfg.InstanceGuidsBufferSize), pctToTriggerUpdate, realmID)
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
