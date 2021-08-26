package main

import (
	"context"
	"database/sql"
	"net"
	"time"

	//_ "net/http/pprof"

	_ "github.com/go-sql-driver/mysql"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	root "github.com/walkline/ToCloud9/apps/game-load-balancer"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/config"
	events_broadcaster "github.com/walkline/ToCloud9/apps/game-load-balancer/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/repo"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/service"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/session"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/sockets/gamesocket"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"github.com/walkline/ToCloud9/shared/events"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
)

func main() {
	// debugging with pprof
	//go func() {
	//	http.ListenAndServe("localhost:6060", nil)
	//}()

	conf, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = conf.Logger()

	authDB, err := sql.Open("mysql", conf.AuthDBConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to auth db")
	}
	defer authDB.Close()

	accountRepo, err := repo.NewAccountMySQLRepo(authDB)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create account repo")
	}

	l, err := net.Listen("tcp4", "0.0.0.0:"+conf.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't start listening")
	}
	defer l.Close()

	charClient := charService(conf)
	chatClient := chatService(conf)
	servRegistryClient := servRegistryService(conf)

	healthandmetrics.EnableActiveConnectionsMetrics()
	healthCheckServer := healthandmetrics.NewServer(conf.HealthCheckPort, true)

	go func() {
		err = healthCheckServer.ListenAndServe()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to ListenAndServe health check server")
		}
	}()

	id := registerLoadBalancer(servRegistryClient, conf)

	nc, err := nats.Connect(conf.NatsURL, nats.PingInterval(20*time.Second), nats.MaxPingsOutstanding(5), nats.Timeout(10*time.Second))
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to nats")
	}
	defer nc.Close()

	broadcaster := events_broadcaster.NewBroadcaster()

	chatListener := service.NewChatNatsListener(nc, id, broadcaster)
	err = chatListener.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to chat events-broadcaster")
	}

	producer := events.NewLoadBalancerProducerNatsJSON(nc, root.Ver, root.RealmID, id)

	log.Info().
		Str("address", l.Addr().String()).
		Msg("🚀 Game Load Balancer started!")

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal().Err(err).Msg("can't accept connection")
		}

		//pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)

		s := gamesocket.NewGameSocket(conn, accountRepo, session.GameSessionParams{
			CharServiceClient:     charClient,
			ServersRegistryClient: servRegistryClient,
			ChatServiceClient:     chatClient,
			EventsProducer:        producer,
			EventsBroadcaster:     broadcaster,
		})
		go func() {
			healthandmetrics.ActiveConnectionsMetrics.Inc()
			defer healthandmetrics.ActiveConnectionsMetrics.Dec()

			// blocks until connection close
			s.ListenAndProcess(context.Background())
		}()
	}
}

func charService(cnf *config.Config) pbChar.CharactersServiceClient {
	conn, err := grpc.Dial(cnf.CharServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to characters service")
	}

	return pbChar.NewCharactersServiceClient(conn)
}

func servRegistryService(cnf *config.Config) pbServ.ServersRegistryServiceClient {
	conn, err := grpc.Dial(cnf.ServersRegistryServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to servers registry service")
	}

	return pbServ.NewServersRegistryServiceClient(conn)
}

func chatService(cnf *config.Config) pbChat.ChatServiceClient {
	conn, err := grpc.Dial(cnf.ChatServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to servers registry service")
	}

	return pbChat.NewChatServiceClient(conn)
}

func registerLoadBalancer(servRegistryClient pbServ.ServersRegistryServiceClient, conf *config.Config) string {
	r, err := servRegistryClient.RegisterLoadBalancer(context.Background(), &pbServ.RegisterLoadBalancerRequest{
		Api:               root.SupportedServerRegistryVer,
		GamePort:          uint32(conf.PortInt()),
		HealthPort:        uint32(conf.HealthCheckPortInt()),
		RealmID:           root.RealmID,
		PreferredHostName: conf.PreferredHostname,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("can't register load balancer")
	}
	return r.Id
}
