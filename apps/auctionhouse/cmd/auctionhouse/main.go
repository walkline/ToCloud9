package main

import (
	"context"
	"database/sql"
	"encoding/json"
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

	"github.com/walkline/ToCloud9/apps/auctionhouse/config"
	"github.com/walkline/ToCloud9/apps/auctionhouse/repo"
	"github.com/walkline/ToCloud9/apps/auctionhouse/server"
	"github.com/walkline/ToCloud9/apps/auctionhouse/service"
	"github.com/walkline/ToCloud9/gen/auctionhouse/pb"
	pbMail "github.com/walkline/ToCloud9/gen/mail/pb"
	shrepo "github.com/walkline/ToCloud9/shared/repo"
	"github.com/walkline/ToCloud9/shared/events"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = cfg.Logger()

	charDB := shrepo.NewCharactersDB()
	var realmIDs []uint32
	for realmID, connStr := range cfg.CharDBConnection {
		cdb, err := sql.Open("mysql", connStr)
		if err != nil {
			log.Fatal().Err(err).Uint32("realmID", realmID).Msg("can't connect to char db")
		}
		configureDBConn(cdb)
		charDB.SetDBForRealm(realmID, cdb)
		realmIDs = append(realmIDs, realmID)
	}

	auctionRepo, err := repo.NewAuctionMySQLRepo(charDB)
	if err != nil {
		log.Fatal().Err(err).Msg("can't create auction repo")
	}

	// Connect to world database for item templates
	worldDB, err := sql.Open("mysql", cfg.WorldDBConnection)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to world db")
	}
	defer worldDB.Close()
	configureDBConn(worldDB)

	// Load item templates
	itemTemplates, err := repo.NewItemTemplateCache(worldDB)
	if err != nil {
		log.Fatal().Err(err).Msg("can't load item templates")
	}

	// Connect to mail service
	mailConn, err := grpc.Dial(cfg.MailServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to mail service")
	}
	defer mailConn.Close()
	mailClient := pbMail.NewMailServiceClient(mailConn)

	// Connect to NATS
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to NATS")
	}
	defer nc.Close()

	eventsProducer := events.NewAuctionHouseProducer(nc)
	auctionService := service.NewAuctionService(auctionRepo, mailClient, eventsProducer, itemTemplates)

	// Subscribe to auction events from other instances
	subscribeToAuctionEvents(nc, auctionService)

	// Load auctions from DB for all realms
	for _, realmID := range realmIDs {
		if err := auctionService.LoadAuctions(context.Background(), realmID); err != nil {
			log.Fatal().Err(err).Uint32("realmID", realmID).Msg("can't load auctions")
		}
	}

	// Start expiration ticker
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.ExpiredAuctionsCheckSecsDelay) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				for _, realmID := range realmIDs {
					auctionService.ProcessExpiredAuctions(context.Background(), realmID)
				}
			}
		}
	}()

	// gRPC setup
	lis, err := net.Listen("tcp4", ":"+cfg.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen for incoming connections")
	}

	grpcServer := grpc.NewServer()
	ahServer := server.NewAuctionHouseServer(auctionService)
	if cfg.LogLevel == zerolog.DebugLevel {
		ahServer = server.NewAuctionHouseDebugLoggerMiddleware(ahServer, log.Logger)
	}
	pb.RegisterAuctionHouseServiceServer(grpcServer, ahServer)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		sig := <-sigCh
		fmt.Println("")
		log.Info().Msgf("Got signal %v, attempting graceful shutdown...", sig)
		grpcServer.GracefulStop()
		wg.Done()
	}()

	log.Info().Str("address", lis.Addr().String()).Msg("Auction House Service started!")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("couldn't serve")
	}

	wg.Wait()

	log.Info().Msg("Server successfully stopped.")
}

func configureDBConn(db *sql.DB) {
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Minute * 4)
	db.SetConnMaxIdleTime(time.Minute * 8)
}

func subscribeToAuctionEvents(nc *nats.Conn, svc *service.AuctionService) {
	// Subscribe to auction created events
	_, err := nc.Subscribe(events.AuctionHouseEventAuctionCreated, func(msg *nats.Msg) {
		var payload events.AuctionHouseEventAuctionCreatedPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal auction created event")
			return
		}
		svc.HandleAuctionCreated(&payload)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to subscribe to auction created events")
	}

	// Subscribe to bid placed events
	_, err = nc.Subscribe(events.AuctionHouseEventBidPlaced, func(msg *nats.Msg) {
		var payload events.AuctionHouseEventBidPlacedPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal bid placed event")
			return
		}
		svc.HandleBidPlaced(&payload)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to subscribe to bid placed events")
	}

	// Subscribe to auction canceled events
	_, err = nc.Subscribe(events.AuctionHouseEventAuctionCanceled, func(msg *nats.Msg) {
		var payload events.AuctionHouseEventAuctionCanceledPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal auction canceled event")
			return
		}
		svc.HandleAuctionCanceled(&payload)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to subscribe to auction canceled events")
	}

	// Subscribe to auction expired events
	_, err = nc.Subscribe(events.AuctionHouseEventAuctionExpired, func(msg *nats.Msg) {
		var payload events.AuctionHouseEventAuctionExpiredPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal auction expired event")
			return
		}
		svc.HandleAuctionExpired(&payload)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to subscribe to auction expired events")
	}

	log.Info().Msg("Subscribed to auction house events")
}
