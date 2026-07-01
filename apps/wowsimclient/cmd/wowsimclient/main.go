package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/walkline/ToCloud9/apps/wowsimclient"
	"github.com/walkline/ToCloud9/apps/wowsimclient/orchestrator"
)

func main() {
	// Support subcommand-style: wowsimclient orchestrator --flags...
	// Go's flag package stops at the first non-flag arg, so we pull the
	// subcommand out of os.Args before parsing flags.
	runMode := "cli"
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		runMode = os.Args[1]
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}

	// CLI flags
	mode := flag.String("mode", "", "Run mode: 'cli' for single bot, 'node' for HTTP API node server, 'orchestrator' for test controller")
	username := flag.String("username", "admin", "Account username")
	password := flag.String("password", "admin", "Account password")
	authServer := flag.String("auth-server", "127.0.0.1:3724", "Auth server address (host:port)")
	charName := flag.String("char-name", "Loadtst", "Character name")
	realmIndex := flag.Int("realm-index", 0, "Realm index (0-based)")
	listenAddr := flag.String("listen", ":8888", "HTTP server listen address (node mode)")
	race := flag.Int("race", 1, "Character race for creation (default: 1=Human)")
	class := flag.Int("class", 1, "Character class for creation (default: 1=Warrior)")
	botMode := flag.String("bot-mode", "grind", "Bot behavior mode: grind, hogger, dungeon, idle, lua")
	bots := flag.Int("bots", 0, "Alias for --num-bots")
	dungeonName := flag.String("dungeon", "", "Dungeon name for dungeon mode (ragefire_chasm, the_deadmines)")
	dataDir := flag.String("data-dir", "", "Path to data directory root containing mmaps/, maps/, vmaps/ for embedded pathfinding")
	pathfindingAddr := flag.String("pathfinding-addr", "", "Address of external pathfinding gRPC service")
	luaScript := flag.String("lua-script", "", "Path to Lua script file")
	deleteExistingChars := flag.Bool("delete-existing-chars", false, "Delete all existing characters on the account before creating the target one (enabled automatically by orchestrator)")
	spawnRateLimit := flag.Int("spawn-rate-limit", 50, "Max bots to spawn per spawn-rate-interval (orchestrator)")
	spawnRateInterval := flag.Duration("spawn-rate-interval", 2*time.Second, "Interval for spawn rate limit (orchestrator)")
	logDecisionsToChat := flag.Bool("log-decisions-to-chat", true, "Bots will /say their major AI decisions (throttled, great for debugging behavior in-game)")
	disableTargetCache := flag.Bool("disable-target-cache", false, "Disable the short-lived target cache in findBestTarget (forces fresh scans every tick). Helps when bots attack dead mobs or ignore live ones due to stale cache.")

	// Orchestrator flags
	numBots := flag.Int("num-bots", 1, "Number of bots to create (orchestrator mode)")
	nodes := flag.String("nodes", "", "Comma-separated list of node addresses (orchestrator mode)")
	accountPrefix := flag.String("account-prefix", "loadbot", "Account name prefix (orchestrator mode)")
	accountPassword := flag.String("account-password", "loadbot", "Account password (orchestrator mode)")
	dbDSN := flag.String("db-dsn", "acore:acore@tcp(127.0.0.1:3306)/acore_auth", "Auth database DSN (orchestrator mode)")
	duration := flag.Duration("duration", 5*time.Minute, "Test duration (orchestrator mode)")

	flag.Parse()

	// --mode flag overrides subcommand if set
	if *mode != "" {
		runMode = *mode
	}

	// --bots N is a convenient alias for --num-bots N
	if *bots > 0 {
		*numBots = *bots
	}

	switch runMode {
	case "cli":
		runCLI(*username, *password, *authServer, *charName, *realmIndex, uint8(*race), uint8(*class),
			*botMode, *dungeonName, *dataDir, *pathfindingAddr, *luaScript, *deleteExistingChars, *logDecisionsToChat, *disableTargetCache)
	case "node", "server":
		runNode(*listenAddr)
	case "orchestrator":
		runOrchestrator(*authServer, *dbDSN, *nodes, *accountPrefix, *accountPassword,
			*numBots, uint8(*race), uint8(*class), *botMode, *dungeonName,
			*dataDir, *pathfindingAddr, *luaScript, *duration, *deleteExistingChars, *spawnRateLimit, *spawnRateInterval,
			*logDecisionsToChat, *disableTargetCache)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", runMode)
		os.Exit(1)
	}
}

func runCLI(username, password, authServer, charName string, realmIndex int,
	race, class uint8, botMode, dungeonName, dataDir, pathfindingAddr, luaScript string, deleteExistingChars bool, logDecisionsToChat bool, disableTargetCache bool) {

	config := wowsimclient.BotConfig{
		Username:                 username,
		Password:                 password,
		AuthServer:               authServer,
		CharacterName:            charName,
		RealmIndex:               realmIndex,
		Race:                     race,
		Class:                    class,
		Mode:                     botMode,
		DungeonName:              dungeonName,
		DataDir:                  dataDir,
		PathfindingAddress:       pathfindingAddr,
		LuaScript:                luaScript,
		AITickMs:                 200,
		DeleteExistingCharacters: deleteExistingChars, // only when explicitly passed for direct cli
		LogDecisionsToChat:       logDecisionsToChat,
		DisableTargetCache:       disableTargetCache,
	}

	bot := wowsimclient.NewBot("cli-1", config)

	// Handle SIGINT for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down bot...")
		bot.Stop()
	}()

	result := bot.Run()

	fmt.Printf("\n=== Bot Result ===\n")
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Level:  %d\n", result.Level)
	fmt.Printf("Kills:  %d\n", result.Kills)
	fmt.Printf("Deaths: %d\n", result.Deaths)
	if result.Error != "" {
		fmt.Printf("Error:  %s\n", result.Error)
	}

	fmt.Printf("\n=== Events ===\n")
	for _, e := range bot.Events() {
		fmt.Printf("[%s] %s: %s\n", e.Time.Format("15:04:05"), e.Type, e.Message)
	}

	if result.Status == wowsimclient.BotStatusError {
		os.Exit(1)
	}
}

func runNode(listenAddr string) {
	server := wowsimclient.NewServer()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nNode server shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		server.Stop(ctx)
	}()

	if err := server.Start(listenAddr); err != nil {
		fmt.Fprintf(os.Stderr, "Node server error: %v\n", err)
	}
}

func runOrchestrator(authServer, dbDSN, nodesStr, accountPrefix, accountPassword string,
	numBots int, race, class uint8, botMode, dungeonName, dataDir, pathfindingAddr, luaScript string,
	duration time.Duration, deleteExistingChars bool, spawnRateLimit int, spawnRateInterval time.Duration,
	logDecisionsToChat bool, disableTargetCache bool) {

	fmt.Println("=== Load Test Orchestrator ===")

	var nodeAddrs []string
	if nodesStr != "" {
		nodeAddrs = strings.Split(nodesStr, ",")
	}

	cfg := orchestrator.Config{
		AuthDBDSN:                dbDSN,
		AuthServerAddr:           authServer,
		NodeAddresses:            nodeAddrs,
		AccountPrefix:            accountPrefix,
		AccountPassword:          accountPassword,
		NumBots:                  numBots,
		DefaultRace:              race,
		DefaultClass:             class,
		DefaultMode:              botMode,
		DungeonName:              dungeonName,
		DataDir:                  dataDir,
		PathfindingAddress:       pathfindingAddr,
		LuaScript:                luaScript,
		DeleteExistingCharacters: deleteExistingChars || true, // orchestrator enables by default
		SpawnRateLimit:           spawnRateLimit,
		SpawnRateInterval:        spawnRateInterval,
		LogDecisionsToChat:       logDecisionsToChat,
		DisableTargetCache:       disableTargetCache,
	}
	fmt.Printf("[main] orchestrator cfg: spawnRateLimit=%d spawnRateInterval=%v numBots=%d nodes=%v disableTargetCache=%v\n", spawnRateLimit, spawnRateInterval, numBots, nodeAddrs, disableTargetCache)

	orch, err := orchestrator.NewOrchestrator(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create orchestrator: %v\n", err)
		os.Exit(1)
	}
	defer orch.Close()

	// Prepare accounts
	fmt.Println("Preparing accounts...")
	assignments, err := orch.PrepareAccounts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to prepare accounts: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Prepared %d bot accounts\n", len(assignments))
	for _, a := range assignments {
		fmt.Printf("  %s -> %s@%s (char: %s)\n", a.BotID, a.AccountName, a.NodeAddress, a.CharacterName)
	}

	// If nodes are specified, distribute to remote nodes
	if len(nodeAddrs) > 0 {
		fmt.Println("Launching bots on remote nodes (via rate-limited sends)...")
		if err := orch.LaunchBots(assignments); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to launch bots: %v\n", err)
			os.Exit(1)
		}
	}

	// For local execution (no nodes specified), run bots locally
	var localBots []*wowsimclient.Bot
	if len(nodeAddrs) == 0 {
		fmt.Println("Running bots locally (via rate-limited launch)...")
		localAssignments := make([]orchestrator.BotAssignment, 0, len(assignments))
		for _, a := range assignments {
			// For local, we use the assignment struct from orchestrator
			localAssignments = append(localAssignments, a)
		}

		_ = orch.LaunchWithRateLimit(localAssignments, func(a orchestrator.BotAssignment) error {
			config := wowsimclient.BotConfig{
				Username:                 a.AccountName,
				Password:                 a.Password,
				AuthServer:               authServer,
				CharacterName:            a.CharacterName,
				Race:                     a.Race,
				Class:                    a.Class,
				Mode:                     botMode,
				DungeonName:              dungeonName,
				DataDir:                  dataDir,
				PathfindingAddress:       pathfindingAddr,
				LuaScript:                luaScript,
				AITickMs:                 200,
				DeleteExistingCharacters: deleteExistingChars || true, // orchestrator always cleans
				LogDecisionsToChat:       logDecisionsToChat,
				DisableTargetCache:       disableTargetCache,
			}

			bot := wowsimclient.NewBot(a.BotID, config)
			localBots = append(localBots, bot)
			go bot.Run()
			return nil
		})
	}

	// Wait for duration or signal
	fmt.Printf("Test running for %v (press Ctrl+C to stop early)...\n", duration)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("\nStopping test early...")
	case <-time.After(duration):
		fmt.Println("Test duration reached.")
	}

	// Stop local bots
	for _, bot := range localBots {
		bot.Stop()
	}
	time.Sleep(2 * time.Second)

	// Collect results
	fmt.Println("\n=== Test Results ===")
	if len(localBots) > 0 {
		for _, bot := range localBots {
			result := bot.Status()
			printBotResult(result)
			for _, e := range bot.Events() {
				fmt.Printf("  [%s] %s: %s\n", e.Time.Format("15:04:05"), e.Type, e.Message)
			}
		}
	}

	if len(nodeAddrs) > 0 {
		results := orch.CollectResults()
		for _, r := range results {
			data, _ := json.MarshalIndent(r, "  ", "  ")
			fmt.Printf("  %s\n", string(data))
		}
	}
}

func printBotResult(r wowsimclient.BotResult) {
	fmt.Printf("Bot %s: status=%s level=%d kills=%d deaths=%d",
		r.ID, r.Status, r.Level, r.Kills, r.Deaths)
	if r.Error != "" {
		fmt.Printf(" error=%s", r.Error)
	}
	fmt.Println()
}
