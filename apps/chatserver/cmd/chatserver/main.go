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
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/walkline/ToCloud9/apps/chatserver/config"
	"github.com/walkline/ToCloud9/apps/chatserver/repo"
	"github.com/walkline/ToCloud9/apps/chatserver/sender"
	"github.com/walkline/ToCloud9/apps/chatserver/server"
	"github.com/walkline/ToCloud9/apps/chatserver/service"
	pbChar "github.com/walkline/ToCloud9/gen/characters/pb"
	"github.com/walkline/ToCloud9/gen/chat/pb"
	shrepo "github.com/walkline/ToCloud9/shared/repo"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	log.Logger = cfg.Logger()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// nats setup
	nc, err := nats.Connect(
		cfg.NatsURL,
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(5),
		nats.Timeout(10*time.Second),
		nats.Name("chatserver"),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to the Nats")
	}
	defer nc.Close()

	serviceID := uuid.New().String()
	log.Info().Str("serviceID", serviceID).Msg("Chat service instance ID")

	charRepo := repo.NewCharactersInMemRepo()

	// Setup MySQL connections for channels
	charDB := shrepo.NewCharactersDB()
	for realmID, connStr := range cfg.CharDBConnection {
		cdb, err := sql.Open("mysql", connStr)
		if err != nil {
			log.Fatal().Err(err).Uint32("realmID", realmID).Msg("can't connect to char db")
		}
		configureDBConn(cdb)
		charDB.SetDBForRealm(realmID, cdb)
	}
	channelsRepo := repo.NewChannelsMYSQL(charDB)

	// Channel manager setup
	channelMgr := service.NewChannelManager(channelsRepo)

	// Characters service client
	charClient := charService(cfg)

	// Preload channels and validate members on startup
	log.Info().Msg("Preloading channels and validating members...")
	for realmID := range cfg.CharDBConnection {
		// First preload all channels from DB
		if err := channelMgr.PreloadChannels(context.Background(), realmID); err != nil {
			log.Error().Err(err).Uint32("realmID", realmID).Msg("Failed to preload channels")
			continue
		}

		// Then validate and prune offline members
		if err := validateChannelMembers(context.Background(), channelMgr, charClient, realmID); err != nil {
			log.Error().Err(err).Uint32("realmID", realmID).Msg("Failed to validate channel members")
		}
	}

	// Start periodic channel cleanup
	cleanupInterval, err := time.ParseDuration(cfg.ChannelCleanupInterval)
	if err != nil {
		log.Fatal().Err(err).Str("interval", cfg.ChannelCleanupInterval).Msg("invalid channel cleanup interval")
	}
	inactiveThreshold, err := time.ParseDuration(cfg.ChannelInactiveThreshold)
	if err != nil {
		log.Fatal().Err(err).Str("threshold", cfg.ChannelInactiveThreshold).Msg("invalid channel inactive threshold")
	}

	log.Info().
		Dur("cleanupInterval", cleanupInterval).
		Dur("inactiveThreshold", inactiveThreshold).
		Msg("Starting channel cleanup scheduler")

	go func() {
		// Add random jitter (0-30s) to prevent all instances from cleaning up simultaneously
		// This spreads out DB load across the cluster
		jitter := time.Duration(time.Now().UnixNano()%30) * time.Second
		log.Info().Dur("jitter", jitter).Msg("Applied startup jitter to cleanup schedule")

		select {
		case <-time.After(jitter):
		case <-ctx.Done():
			return
		}

		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				for realmID := range cfg.CharDBConnection {
					if err := channelMgr.CleanupStaleChannels(ctx, realmID, inactiveThreshold); err != nil {
						log.Error().Err(err).Uint32("realmID", realmID).Msg("Failed to cleanup stale channels")
					}
				}
			case <-ctx.Done():
				log.Info().Msg("Stopping channel cleanup scheduler")
				return
			}
		}
	}()

	// Message producer setup (for broadcasting channel events)
	msgProducer := sender.NewMsgProducerNatsJSON(nc, "ALL") // Broadcast to all gateways

	// listeners setup
	charListener := service.NewCharactersListener(charRepo, channelMgr, nc)
	err = charListener.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't start listen to characters events-broadcaster")
	}

	srListener := service.NewServersRegistryListener(charRepo, nc)
	err = srListener.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't start listen to services registry events-broadcaster")
	}

	channelsListener := service.NewChannelsListener(serviceID, channelMgr, nc)
	err = channelsListener.Listen()
	if err != nil {
		log.Fatal().Err(err).Msg("can't start listen to channel sync events")
	}

	// grpc setup
	lis, err := net.Listen("tcp4", ":"+cfg.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("can't listen for incoming connections")
	}

	grpcServer := grpc.NewServer()
	pb.RegisterChatServiceServer(
		grpcServer,
		server.NewChatService(charRepo, channelMgr, msgProducer, serviceID),
	)

	// graceful shutdown handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		sig := <-sigCh
		fmt.Println("")
		log.Info().Msgf("🧨 Got signal %v, attempting graceful shutdown...", sig)

		// Cancel context to stop background goroutines
		cancel()

		// Stop gRPC server (waits for in-flight requests)
		grpcServer.GracefulStop()

		// Stop listeners
		if err := charListener.Stop(); err != nil {
			log.Error().Err(err).Msg("Failed to stop characters listener")
		}
		if err := srListener.Stop(); err != nil {
			log.Error().Err(err).Msg("Failed to stop services registry listener")
		}
		if err := channelsListener.Stop(); err != nil {
			log.Error().Err(err).Msg("Failed to stop channels listener")
		}

		// Close database connections
		for realmID := range cfg.CharDBConnection {
			if db := charDB.DBByRealm(realmID); db != nil {
				if err := db.Close(); err != nil {
					log.Error().Err(err).Uint32("realmID", realmID).Msg("Failed to close database")
				}
			}
		}

		wg.Done()
	}()

	log.Info().Str("address", lis.Addr().String()).Msg("🚀 Chat Service started!")

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

func charService(cnf *config.Config) pbChar.CharactersServiceClient {
	conn, err := grpc.Dial(cnf.CharServiceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to characters service")
	}

	return pbChar.NewCharactersServiceClient(conn)
}

func validateChannelMembers(ctx context.Context, channelMgr *service.ChannelManager, charClient pbChar.CharactersServiceClient, realmID uint32) error {
	// Get all online players from charserver
	resp, err := charClient.GetOnlineCharacters(ctx, &pbChar.GetOnlineCharactersRequest{
		Api:     "0.0.1",
		RealmID: realmID,
	})
	if err != nil {
		return fmt.Errorf("failed to get online characters: %w", err)
	}

	log.Info().
		Uint32("realmID", realmID).
		Uint32("onlineCount", resp.TotalCount).
		Msg("Retrieved online players from charserver")

	// Build lookup map
	onlineGUIDs := make(map[uint64]bool, len(resp.CharacterGUIDs))
	for _, guid := range resp.CharacterGUIDs {
		onlineGUIDs[guid] = true
	}

	// Prune offline members from all channels
	if err := channelMgr.PruneOfflineMembersFromAllChannels(ctx, realmID, onlineGUIDs); err != nil {
		return fmt.Errorf("failed to prune offline members: %w", err)
	}

	log.Info().Uint32("realmID", realmID).Msg("Channel member validation completed")
	return nil
}
