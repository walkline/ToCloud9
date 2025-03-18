package main

import (
	"context"
	"database/sql"
	"net"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	root "github.com/walkline/ToCloud9/apps/gateway"
	"github.com/walkline/ToCloud9/apps/gateway/config"
	eventsBroadcaster "github.com/walkline/ToCloud9/apps/gateway/events-broadcaster"
	"github.com/walkline/ToCloud9/apps/gateway/repo"
	"github.com/walkline/ToCloud9/apps/gateway/service"
	"github.com/walkline/ToCloud9/apps/gateway/session"
	"github.com/walkline/ToCloud9/apps/gateway/sockets/gamesocket"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	pbChat "github.com/walkline/ToCloud9/gen/chat/pb"
	pbGroup "github.com/walkline/ToCloud9/gen/group/pb"
	pbGuild "github.com/walkline/ToCloud9/gen/guilds/pb"
	pbMail "github.com/walkline/ToCloud9/gen/mail/pb"
	pbMM "github.com/walkline/ToCloud9/gen/matchmaking/pb"
	pbServ "github.com/walkline/ToCloud9/gen/servers-registry/pb"
	"github.com/walkline/ToCloud9/shared/events"
	gameserverconn "github.com/walkline/ToCloud9/shared/gameserver/conn"
	"github.com/walkline/ToCloud9/shared/healthandmetrics"
	sharedRepo "github.com/walkline/ToCloud9/shared/repo"
	//_ "net/http/pprof"
)

func main() {
	//debugging with pprof
	//go func() {
	//	fmt.Println("???")
	//	fmt.Println(http.ListenAndServe(":8333", nil))
	//}()

	//runtime.SetBlockProfileRate(1)

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

	//configureDBConn(authDB)

	accountRepo, err := repo.NewAccountMySQLRepo(authDB, repo.StatementsBuilderForSchema(sharedRepo.ParseSchemaType(conf.DBSchemaType)))
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
	guildClient := guildService(conf)
	mailClient := mailService(conf)
	groupClient := groupService(conf)
	matchmakingClient := matchmakingService(conf)

	healthandmetrics.EnableActiveConnectionsMetrics()
	healthCheckServer := healthandmetrics.NewServer(conf.HealthCheckPort, promhttp.Handler())

	go func() {
		err = healthCheckServer.ListenAndServe()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to ListenAndServe health check server")
		}
	}()

	root.RetrievedGatewayID = registerGateway(servRegistryClient, conf)

	nc, err := nats.Connect(
		conf.NatsURL,
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(5),
		nats.Timeout(10*time.Second),
		nats.Name("gateway-"+root.RetrievedGatewayID),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to nats")
	}
	defer nc.Close()

	broadcaster := eventsBroadcaster.NewBroadcaster()

	chatListener := service.NewChatNatsListener(nc, root.RetrievedGatewayID, broadcaster)
	err = chatListener.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to chat events-broadcaster")
	}

	guildListener := service.NewGuildNatsListener(nc, broadcaster)
	err = guildListener.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to guild events-broadcaster")
	}

	mailListener := service.NewMailNatsListener(nc, broadcaster)
	err = mailListener.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to mail events-broadcaster")
	}

	groupListener := service.NewGroupNatsListener(nc, broadcaster)
	err = groupListener.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to group events-broadcaster")
	}

	mmListener := service.NewMatchmakingNatsListener(nc, broadcaster)
	err = mmListener.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen to matchmaking events-broadcaster")
	}

	producer := events.NewGatewayProducerNatsJSON(nc, root.Ver, root.RealmID, root.RetrievedGatewayID)
	charsUpdsBarrier := service.NewCharactersUpdatesBarrier(&log.Logger, producer, time.Second)
	go charsUpdsBarrier.Run(context.TODO())

	realmNamesServive, err := service.NewRealmNamesService(context.Background(), repo.NewRealmNamesMySQLRepo(authDB))
	if err != nil {
		log.Fatal().Err(err).Msg("can't create realm names service")
	}

	log.Info().
		Str("address", l.Addr().String()).
		Msg("🚀 Gateway started!")

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal().Err(err).Msg("can't accept connection")
		}

		//pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)

		s := gamesocket.NewGameSocket(conn, accountRepo, session.GameSessionParams{
			CharServiceClient:                charClient,
			ServersRegistryClient:            servRegistryClient,
			ChatServiceClient:                chatClient,
			GuildsServiceClient:              guildClient,
			MailServiceClient:                mailClient,
			MatchmakingServiceClient:         matchmakingClient,
			GroupServiceClient:               groupClient,
			EventsProducer:                   producer,
			EventsBroadcaster:                broadcaster,
			CharsUpdsBarrier:                 charsUpdsBarrier,
			RealmNamesService:                realmNamesServive,
			GameServerGRPCConnMgr:            gameserverconn.DefaultGameServerGRPCConnMgr,
			PacketProcessTimeout:             time.Second * time.Duration(conf.PacketProcessTimeoutSecs),
			ShowGameserverConnChangeToClient: conf.ShowGameserverConnChangeToClient,
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

func matchmakingService(cnf *config.Config) pbMM.MatchmakingServiceClient {
	conn, err := grpc.Dial(cnf.MatchmakingServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to matchmaking service")
	}

	return pbMM.NewMatchmakingServiceClient(conn)
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
		log.Fatal().Err(err).Msg("can't connect to chat service")
	}

	return pbChat.NewChatServiceClient(conn)
}

func guildService(cnf *config.Config) pbGuild.GuildServiceClient {
	conn, err := grpc.Dial(cnf.GuildsServiceAddress, grpc.WithInsecure(), grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		dialer := net.Dialer{Timeout: time.Second * 5}
		return dialer.DialContext(ctx, "tcp", s)
	}))
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to guilds service")
	}

	return pbGuild.NewGuildServiceClient(conn)
}

func mailService(cnf *config.Config) pbMail.MailServiceClient {
	conn, err := grpc.Dial(cnf.MailServiceAddress, grpc.WithInsecure(), grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		dialer := net.Dialer{Timeout: time.Second * 5}
		return dialer.DialContext(ctx, "tcp", s)
	}))
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to mail service")
	}

	return pbMail.NewMailServiceClient(conn)
}

func groupService(cnf *config.Config) pbGroup.GroupServiceClient {
	conn, err := grpc.Dial(cnf.GroupServiceAddress, grpc.WithInsecure(), grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		dialer := net.Dialer{Timeout: time.Second * 5}
		return dialer.DialContext(ctx, "tcp", s)
	}))
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to group service")
	}

	return pbGroup.NewGroupServiceClient(conn)
}

func registerGateway(servRegistryClient pbServ.ServersRegistryServiceClient, conf *config.Config) string {
	r, err := servRegistryClient.RegisterGateway(context.Background(), &pbServ.RegisterGatewayRequest{
		Api:               root.SupportedServerRegistryVer,
		GamePort:          uint32(conf.PortInt()),
		HealthPort:        uint32(conf.HealthCheckPortInt()),
		RealmID:           root.RealmID,
		PreferredHostName: conf.PreferredHostname,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("can't register gateway")
	}
	return r.Id
}

func configureDBConn(db *sql.DB) {
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(5)
	db.SetConnMaxLifetime(time.Minute * 4)
	db.SetConnMaxIdleTime(time.Minute * 8)
}
