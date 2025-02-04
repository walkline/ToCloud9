package main

/*
#include <stdint.h>
*/
import "C"
import (
	"context"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/game-server/libsidecar/config"
	"github.com/walkline/ToCloud9/gen/matchmaking/pb"
)

var playerLeftBattlegroundRequestsChan chan matchmakingPlayerLeftBattlegroundRequest
var battlegroundStatusChangedRequestsChan chan matchmakingBattlegroundStatusChangedRequest

type matchmakingPlayerLeftBattlegroundRequest struct {
	player     uint64
	realmID    uint32
	instanceID uint32
}

type matchmakingBattlegroundStatusChangedRequest struct {
	instanceID uint32
	status     uint8
}

func SetupMatchmakingConnection(ctx context.Context, cfg *config.Config) {
	conn, err := grpc.Dial(cfg.MatchmakingServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the matchmaking server")
	}

	matchmakingClient := pb.NewMatchmakingServiceClient(conn)

	playerLeftBattlegroundRequestsChan = make(chan matchmakingPlayerLeftBattlegroundRequest, 100)
	battlegroundStatusChangedRequestsChan = make(chan matchmakingBattlegroundStatusChangedRequest, 50)

	const processorsCount = 4
	for i := 0; i < processorsCount; i++ {
		go func() {
			for {
				select {
				case r := <-playerLeftBattlegroundRequestsChan:
					_, err := matchmakingClient.PlayerLeftBattleground(ctx, &pb.PlayerLeftBattlegroundRequest{
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
					_, err := matchmakingClient.BattlegroundStatusChanged(ctx, &pb.BattlegroundStatusChangedRequest{
						Api:          matchmakingSupportedVer,
						RealmID:      RealmID,
						Status:       pb.BattlegroundStatusChangedRequest_Status(r.status),
						InstanceID:   r.instanceID,
						IsCrossRealm: IsCrossRealm,
					})
					if err != nil {
						log.Err(err).Msg("BattlegroundStatusChanged failed")
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
