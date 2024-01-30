package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/walkline/ToCloud9/apps/perun/internal/config"
	"github.com/walkline/ToCloud9/apps/perun/internal/perun"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("can't load config, err: %s", err)
	}

	p, err := perun.NewWithApps(cfg.Apps)
	if err != nil {
		log.Fatalf("failed to create perun app, err: %s", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	wg := sync.WaitGroup{}
	go func() {
		<-sigCh
		wg.Add(1)
		_ = p.Stop()
		wg.Done()
	}()

	if err := p.Run(); err != nil {
		log.Fatalf("perun app error: %s", err)
	}

	wg.Wait()
}
