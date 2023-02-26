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

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/apps/guidserver/config"
	"github.com/walkline/ToCloud9/apps/guidserver/repo"
	"github.com/walkline/ToCloud9/apps/guidserver/server"
	"github.com/walkline/ToCloud9/apps/guidserver/service"
	"github.com/walkline/ToCloud9/gen/guid/pb"
	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = cfg.Logger()

	// grpc setup
	lis, err := net.Listen("tcp4", ":"+cfg.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen for incoming connections")
	}

	grpcServer := grpc.NewServer()
	guidServer := server.NewGuidServer(createGuidService(cfg))
	if cfg.LogLevel == zerolog.DebugLevel {
		guidServer = server.NewGuidsDebugLoggerMiddleware(guidServer, log.Logger)
	}
	pb.RegisterGuidServiceServer(grpcServer, guidServer)

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

	log.Info().Str("address", lis.Addr().String()).Msg("🚀 GUID Service started!")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("couldn't serve")
	}

	wg.Wait()

	log.Info().Msg("👍 Server successfully stopped.")
}

func createGuidService(cfg *config.Config) service.GuidService {
	cdb, err := sql.Open("mysql", cfg.CharDBConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to char db")
	}
	defer cdb.Close()

	charDB := shrepo.NewCharactersDB()
	charDB.SetDBForRealm(1, cdb)

	charRepo, err := repo.NewMysqlMaxGuidRepo(charDB)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create char repo")
	}

	opt, err := redis.ParseURL(cfg.RedisConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the redis")
	}

	rdb := redis.NewClient(opt)

	service, err := service.NewGuidService(context.Background(), charRepo, repo.NewRedisMaxGuidStorage(rdb, 10), []uint32{1}, 4)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create guid service")
	}

	return service
}
