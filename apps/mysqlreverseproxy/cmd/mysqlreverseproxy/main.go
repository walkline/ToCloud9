package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/mysqlreverseproxy/config"
	"github.com/walkline/ToCloud9/apps/mysqlreverseproxy/server"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config file")
	}

	log.Logger = cfg.Logger()

	srv := server.NewServer(cfg.Port, cfg.Username, cfg.Password, cfg.CharDBConnection)

	ctx, cancel := context.WithCancel(context.Background())

	// graceful shutdown handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Println("")
		log.Info().Msgf("🧨 Got signal %v, attempting graceful shutdown...", sig)
		cancel()
	}()

	log.Info().Str("port", cfg.Port).Int("realms_count", len(cfg.CharDBConnection)).Msg("🚀 Cross-realm mysql reverse proxy started!")

	err = srv.ListenAndServe(ctx)
	if err != nil {
		log.Err(err).Msg("failed to start server")
	}

	log.Info().Msg("👍 Server successfully stopped.")
}
