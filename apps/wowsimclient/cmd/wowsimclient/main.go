package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/walkline/ToCloud9/apps/wowsimclient"
)

func main() {
	// CLI flags
	mode := flag.String("mode", "cli", "Run mode: 'cli' for single bot, 'server' for HTTP API server")
	username := flag.String("username", "admin", "Account username")
	password := flag.String("password", "admin", "Account password")
	authServer := flag.String("auth-server", "127.0.0.1:3724", "Auth server address (host:port)")
	charName := flag.String("char-name", "Loadtestbot", "Character name")
	realmIndex := flag.Int("realm-index", 0, "Realm index (0-based)")
	listenAddr := flag.String("listen", ":8888", "HTTP server listen address (server mode only)")
	race := flag.Int("race", 5, "Character race for creation (default: 5=Undead)")
	class := flag.Int("class", 1, "Character class for creation (default: 1=Warrior)")

	flag.Parse()

	switch *mode {
	case "cli":
		runCLI(*username, *password, *authServer, *charName, *realmIndex, uint8(*race), uint8(*class))
	case "server":
		runServer(*listenAddr)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", *mode)
		os.Exit(1)
	}
}

func runCLI(username, password, authServer, charName string, realmIndex int, race, class uint8) {
	config := wowsimclient.BotConfig{
		Username:      username,
		Password:      password,
		AuthServer:    authServer,
		CharacterName: charName,
		RealmIndex:    realmIndex,
		Race:          race,
		Class:         class,
	}

	bot := wowsimclient.NewBot("cli-1", config)
	result := bot.Run()

	if result.Status == wowsimclient.BotStatusError {
		fmt.Fprintf(os.Stderr, "Bot failed: %s\n", result.Error)
		os.Exit(1)
	}

	fmt.Println("Bot completed successfully")
}

func runServer(listenAddr string) {
	server := wowsimclient.NewServer()
	if err := server.Start(listenAddr); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
