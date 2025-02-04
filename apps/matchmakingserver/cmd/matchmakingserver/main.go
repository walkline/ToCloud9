package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
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
	"github.com/walkline/ToCloud9/gen/matchmaking/pb"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/gameserver/conn"
	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = cfg.Logger()

	wdb, err := sql.Open("mysql", cfg.WorldDBConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to world db")
	}

	charDB := shrepo.NewCharactersDB()

	realmIDs := []uint32{}
	for realmID, charDBConn := range cfg.CharDBConnection {
		cdb, err := sql.Open("mysql", charDBConn)
		if err != nil {
			log.Fatal().Err(err).Msg("can't connect to char db")
		}

		charDB.SetDBForRealm(realmID, cdb)

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
		battlegroupsRepo,
		repo.NewBattlegroundInMemRepo(),
		crossRealmTracker,
		events.NewMatchmakingServiceProducerNatsJSON(nc, matchmaking.Ver),
		serversRegistryClient,
		gameserverConnMgr,
		realmIDs,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create BattleGroundService service")
	}

	charLis := service.NewCharactersListener(bgService, nc)
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

	lis, err := net.Listen("tcp4", ":"+cfg.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen for incoming connections")
	}

	grpcServer := grpc.NewServer()
	matchmakingServer := server.NewMatchmakingServer(bgService, gameserverConnMgr)
	if cfg.LogLevel == zerolog.DebugLevel {
		matchmakingServer = server.NewMatchmakingDebugLoggerMiddleware(matchmakingServer, log.Logger)
	}
	pb.RegisterMatchmakingServiceServer(grpcServer, matchmakingServer)

	go bgService.ProcessExpiredBattlegroundInvites(ctx)

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
