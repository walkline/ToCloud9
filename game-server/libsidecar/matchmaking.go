package main

/*
#include <stdint.h>
*/
import "C"
import (
	"context"
	"unsafe"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	"github.com/walkline/ToCloud9/gen/matchmaking/pb"
	"github.com/walkline/ToCloud9/shared/wow/guid"
)

var playerLeftBattlegroundRequestsChan chan matchmakingPlayerLeftBattlegroundRequest
var battlegroundStatusChangedRequestsChan chan matchmakingBattlegroundStatusChangedRequest
var lfgDungeonCompletedRequestsChan chan matchmakingLfgDungeonCompletedRequest
var matchmakingServiceClient pb.MatchmakingServiceClient

type matchmakingPlayerLeftBattlegroundRequest struct {
	player     uint64
	realmID    uint32
	instanceID uint32
}

type matchmakingBattlegroundStatusChangedRequest struct {
	instanceID uint32
	status     uint8
}

type matchmakingLfgDungeonCompletedRequest struct {
	completedDungeonEntry uint32
	selectedDungeonEntry  uint32
	players               []uint64
}

func SetupMatchmakingConnection(ctx context.Context, cfg *config.Config) {
	conn, err := grpc.Dial(cfg.MatchmakingServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the matchmaking server")
	}

	matchmakingServiceClient = pb.NewMatchmakingServiceClient(conn)

	playerLeftBattlegroundRequestsChan = make(chan matchmakingPlayerLeftBattlegroundRequest, 100)
	battlegroundStatusChangedRequestsChan = make(chan matchmakingBattlegroundStatusChangedRequest, 50)
	lfgDungeonCompletedRequestsChan = make(chan matchmakingLfgDungeonCompletedRequest, 50)

	const processorsCount = 4
	for i := 0; i < processorsCount; i++ {
		go func() {
			for {
				select {
				case r := <-playerLeftBattlegroundRequestsChan:
					_, err := matchmakingServiceClient.PlayerLeftBattleground(ctx, &pb.PlayerLeftBattlegroundRequest{
						Api:          matchmakingSupportedVer,
						RealmID:      r.realmID,
						PlayerGUID:   r.player,
						InstanceID:   r.instanceID,
						IsCrossRealm: IsCrossRealm,
					})
					if err != nil {
						log.Err(err).Msg("PlayerLeftBattleground failed")
					}
				case r := <-battlegroundStatusChangedRequestsChan:
					_, err := matchmakingServiceClient.BattlegroundStatusChanged(ctx, &pb.BattlegroundStatusChangedRequest{
						Api:          matchmakingSupportedVer,
						RealmID:      RealmID,
						Status:       pb.BattlegroundStatusChangedRequest_Status(r.status),
						InstanceID:   r.instanceID,
						IsCrossRealm: IsCrossRealm,
					})
					if err != nil {
						log.Err(err).Msg("BattlegroundStatusChanged failed")
					}
				case r := <-lfgDungeonCompletedRequestsChan:
					_, err := matchmakingServiceClient.CompleteLfgDungeon(ctx, &pb.CompleteLfgDungeonRequest{
						Api:                   matchmakingSupportedVer,
						CompletedDungeonEntry: r.completedDungeonEntry,
						SelectedDungeonEntry:  r.selectedDungeonEntry,
						Players:               lfgCompletedPlayersForRequest(r.players),
					})
					if err != nil {
						log.Err(err).Msg("CompleteLfgDungeon failed")
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		<-ctx.Done()

		close(playerLeftBattlegroundRequestsChan)
		close(battlegroundStatusChangedRequestsChan)
		close(lfgDungeonCompletedRequestsChan)

		if err := conn.Close(); err != nil {
			log.Fatal().Err(err).Msg("can't close matchmaking connection")
		}
	}()
}

// TC9PlayerLeftBattleground notifies matchmaking server that player left battleground
//
//export TC9PlayerLeftBattleground
func TC9PlayerLeftBattleground(playerGUID C.uint64_t, realmID C.uint32_t, instanceID C.uint32_t) {
	playerLeftBattlegroundRequestsChan <- matchmakingPlayerLeftBattlegroundRequest{
		player:     uint64(playerGUID),
		realmID:    uint32(realmID),
		instanceID: uint32(instanceID),
	}
}

// TC9BattlegroundStatusChanged notifies matchmaking server that battleground status changed
//
//export TC9BattlegroundStatusChanged
func TC9BattlegroundStatusChanged(instanceID C.uint32_t, status C.uint8_t) {
	battlegroundStatusChangedRequestsChan <- matchmakingBattlegroundStatusChangedRequest{
		instanceID: uint32(instanceID),
		status:     uint8(status),
	}
}

// TC9CompleteLfgDungeon notifies matchmaking server that AzerothCore completed an LFG dungeon.
//
//export TC9CompleteLfgDungeon
func TC9CompleteLfgDungeon(completedDungeonEntry C.uint32_t, selectedDungeonEntry C.uint32_t, players *C.uint64_t, playersSize C.int) {
	lfgDungeonCompletedRequestsChan <- matchmakingLfgDungeonCompletedRequest{
		completedDungeonEntry: uint32(completedDungeonEntry),
		selectedDungeonEntry:  uint32(selectedDungeonEntry),
		players:               uint64SliceFromC(players, playersSize),
	}
}

func uint64SliceFromC(values *C.uint64_t, size C.int) []uint64 {
	if values == nil || size <= 0 {
		return nil
	}
	result := make([]uint64, 0, int(size))
	stride := unsafe.Sizeof(*values)
	for i := 0; i < int(size); i++ {
		value := *(*C.uint64_t)(unsafe.Pointer(uintptr(unsafe.Pointer(values)) + uintptr(i)*stride))
		if value == 0 {
			continue
		}
		result = append(result, uint64(value))
	}
	return result
}

func lfgCompletedPlayersForRequest(players []uint64) []*pb.CompleteLfgDungeonPlayer {
	result := make([]*pb.CompleteLfgDungeonPlayer, 0, len(players))
	for _, player := range players {
		result = append(result, &pb.CompleteLfgDungeonPlayer{
			RealmID:    guid.PlayerRealmIDOrDefault(RealmID, player),
			PlayerGUID: guid.PlayerLowGUID(player),
		})
	}
	return result
}
