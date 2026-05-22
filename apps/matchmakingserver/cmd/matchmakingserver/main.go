package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	matchmaking "github.com/walkline/ToCloud9/apps/matchmakingserver"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/config"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/repo"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/server"
	"github.com/walkline/ToCloud9/apps/matchmakingserver/service"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbGroup "github.com/walkline/ToCloud9/gen/group/pb"
	"github.com/walkline/ToCloud9/gen/matchmaking/pb"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/gameserver/conn"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = cfg.Logger()

	healthCheckServer := healthandmetrics.NewServer(cfg.HealthCheckPort, nil)
	go func() {
		if err := healthCheckServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("can't start health check server")
		}
	}()
	defer func() {
		if err := healthCheckServer.Shutdown(context.Background()); err != nil {
			log.Error().Err(err).Msg("can't shutdown health check server")
		}
	}()

	wdb, err := sql.Open("mysql", cfg.WorldDBConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to world db")
	}

	realmIDs := []uint32{}
	for realmID := range cfg.CharDBConnection {
		realmIDs = append(realmIDs, realmID)
	}

	nc, err := nats.Connect(
		cfg.NatsURL,
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(5),
		nats.Timeout(10*time.Second),
		nats.Name("matchmaking"),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to nats")
	}
	defer nc.Close()

	serversRegistryClient := servRegistryService(cfg)
	registerMatchmakingServer(serversRegistryClient, cfg, healthCheckServer.StartedAtUnixMs())

	battlegroupsRepo, err := repo.NewBattleGroupsInMemWithConfigValue(cfg.BattleGroups)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create BattleGroupsInMem repository")
	}

	crossRealmTracker, err := service.NewCrossRealmNodesTracker(serversRegistryClient)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create CrossRealmNodesTracker service")
	}

	gameserverConnMgr := conn.NewGameServerGRPCConnMgr()
	bgService, err := service.NewBattleGroundService(
		repo.NewMySQLBattlegroundTemplateRepo(wdb),
		repo.NewCharserverArenaTeamRepo(charService(cfg)),
		battlegroupsRepo,
		repo.NewBattlegroundInMemRepo(),
		crossRealmTracker,
		events.NewMatchmakingServiceProducerNatsJSON(nc, matchmaking.Ver),
		serversRegistryClient,
		gameserverConnMgr,
		cfg.ArenaStartMatchmakerRating,
		service.ArenaRatingConfig{
			WinModifier1:       float64(cfg.ArenaWinRatingModifier1),
			WinModifier2:       float64(cfg.ArenaWinRatingModifier2),
			LoseModifier:       float64(cfg.ArenaLoseRatingModifier),
			MatchmakerModifier: float64(cfg.ArenaMatchmakerRatingModifier),
			MaxAllowedMMRDrop:  cfg.MaxAllowedMMRDrop,
		},
		realmIDs,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create BattleGroundService service")
	}
	matchmakingEventsProducer := events.NewMatchmakingServiceProducerNatsJSON(nc, matchmaking.Ver)
	groupServiceClient := groupService(cfg)
	lfgService := service.NewLFGServiceWithBattleGroupsAndGroupRegistrar(battlegroupsRepo, crossRealmTracker, groupServiceClient, matchmakingEventsProducer)
	lfgMaterializer := service.NewLFGMaterializer(serversRegistryClient, gameserverConnMgr, groupServiceClient)

	charLis := service.NewCharactersListener(bgService, lfgService, nc)
	if err = charLis.Listen(); err != nil {
		log.Fatal().Err(err).Msg("can't start char listener")
	}
	defer charLis.Stop()

	registryList := service.NewServersRegistryListener(
		nc,
		[]service.ServersRegistryGSAddedConsumer{crossRealmTracker},
		[]service.ServersRegistryGSRemovedConsumer{crossRealmTracker, bgService},
	)
	if err = registryList.Listen(); err != nil {
		log.Fatal().Err(err).Msg("can't start registry listener")
	}
	defer registryList.Stop()

	lfgMaterializerListener := service.NewLFGMaterializerListener(nc, lfgMaterializer, lfgService)
	if err = lfgMaterializerListener.Listen(); err != nil {
		log.Fatal().Err(err).Msg("can't start LFG materializer listener")
	}
	defer lfgMaterializerListener.Stop()

	lis, err := net.Listen("tcp4", ":"+cfg.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen for incoming connections")
	}

	grpcServer := grpc.NewServer()
	matchmakingServer := server.NewMatchmakingServer(bgService, lfgService, gameserverConnMgr)
	if cfg.LogLevel == zerolog.DebugLevel {
		matchmakingServer = server.NewMatchmakingDebugLoggerMiddleware(matchmakingServer, log.Logger)
	}
	pb.RegisterMatchmakingServiceServer(grpcServer, matchmakingServer)

	go bgService.ProcessExpiredBattlegroundInvites(ctx)
	go lfgService.ProcessExpiredLfgProposals(ctx)

	// graceful shutdown handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		sig := <-sigCh
		fmt.Println("")
		log.Info().Msgf("🧨 Got signal %v, attempting graceful shutdown...", sig)
		cancel()
		grpcServer.GracefulStop()
		wg.Done()
	}()

	log.Info().Str("address", lis.Addr().String()).Msg("🚀 Matchmaking Service started!")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("couldn't serve")
	}

	wg.Wait()

	log.Info().Msg("👍 Server successfully stopped.")
}

func servRegistryService(cnf *config.Config) pbServ.ServersRegistryServiceClient {
	conn, err := grpc.Dial(cnf.ServersRegistryServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to servers registry service")
	}

	return pbServ.NewServersRegistryServiceClient(conn)
}

func registerMatchmakingServer(client pbServ.ServersRegistryServiceClient, cnf *config.Config, startedAtUnixMs int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.RegisterMatchmakingServer(ctx, &pbServ.RegisterMatchmakingServerRequest{
		Api:               matchmaking.SupportedServerRegistryVer,
		ServicePort:       uint32(cnf.PortInt()),
		HealthPort:        uint32(cnf.HealthCheckPortInt()),
		PreferredHostName: cnf.PreferredHostname,
		StartedAtUnixMs:   startedAtUnixMs,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("can't register matchmaking server")
	}
}

func groupService(cnf *config.Config) pbGroup.GroupServiceClient {
	conn, err := grpc.Dial(cnf.GroupServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to group service")
	}

	return pbGroup.NewGroupServiceClient(conn)
}

func charService(cnf *config.Config) pbChar.CharactersServiceClient {
	conn, err := grpc.Dial(cnf.CharServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to character service")
	}

	return pbChar.NewCharactersServiceClient(conn)
}
