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

	root "github.com/walkline/ToCloud9/apps/game-load-balancer"
	"github.com/walkline/ToCloud9/apps/mailserver/config"
	"github.com/walkline/ToCloud9/apps/mailserver/repo"
	"github.com/walkline/ToCloud9/apps/mailserver/server"
	"github.com/walkline/ToCloud9/apps/mailserver/service"
	"github.com/walkline/ToCloud9/gen/mail/pb"
	"github.com/walkline/ToCloud9/shared/events"
	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = cfg.Logger()

	cdb, err := sql.Open("mysql", cfg.CharDBConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to char db")
	}

	configureDBConn(cdb)

	charDB := shrepo.NewCharactersDB()
	charDB.SetDBForRealm(1, cdb)
	guildsRepo, err := repo.NewMailMySQLRepo(charDB)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create guilds repo")
	}

	nc, err := nats.Connect(
		cfg.NatsURL,
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(5),
		nats.Timeout(10*time.Second),
		nats.Name("mailserver"),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to nats")
	}
	defer nc.Close()

	mailService := service.NewMailService(
		guildsRepo,
		events.NewMailServiceProducerNatsJSON(nc, root.Ver),
		time.Second*time.Duration(cfg.DefaultMailExpirationTimeSecs),
	)

	ticker := service.NewMailsCleanupTicker([]uint32{1}, time.Second*time.Duration(cfg.ExpiredMailsCleanupSecsDelay), mailService)
	go ticker.Start(context.TODO())

	// grpc setup
	lis, err := net.Listen("tcp4", ":"+cfg.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen for incoming connections")
	}

	grpcServer := grpc.NewServer()
	mailServer := server.NewMailServer(mailService)
	if cfg.LogLevel == zerolog.DebugLevel {
		mailServer = server.NewMailDebugLoggerMiddleware(mailServer, log.Logger)
	}
	pb.RegisterMailServiceServer(grpcServer, mailServer)

	// graceful shutdown handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		sig := <-sigCh
		fmt.Println("")
		log.Info().Msgf("🧨 Got signal %v, attempting graceful shutdown...", sig)
		grpcServer.GracefulStop()
		wg.Done()
	}()

	log.Info().Str("address", lis.Addr().String()).Msg("🚀 Mail Service started!")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("couldn't serve")
	}

	wg.Wait()

	log.Info().Msg("👍 Server successfully stopped.")
}

func configureDBConn(db *sql.DB) {
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Minute * 4)
	db.SetConnMaxIdleTime(time.Minute * 8)
}
