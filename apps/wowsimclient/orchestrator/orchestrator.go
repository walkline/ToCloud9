// Package orchestrator manages distributed load testing by creating accounts,
// setting GM rights, and controlling bot nodes that connect to the game server.
package orchestrator

import (
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// SRP6 parameters matching AzerothCore's implementation
var (
	srp6N = new(big.Int).SetBytes([]byte{
		0x89, 0x4B, 0x64, 0x5E, 0x89, 0xE1, 0x53, 0x5B, 0xBD, 0xAD, 0x5B, 0x8B,
		0x29, 0x06, 0x50, 0x53, 0x08, 0x01, 0xB1, 0x8E, 0xBF, 0xBF, 0x5E, 0x8F,
		0xAB, 0x3C, 0x82, 0x87, 0x2A, 0x3E, 0x9B, 0xB7,
	})
	srp6G = big.NewInt(7)
)

// Config holds orchestrator configuration.
type Config struct {
	// Database connection in MySQL DSN format: user:pass@tcp(host:port)/dbname
	AuthDBDSN       string `json:"auth_db_dsn"`
	WorldDBDSN      string `json:"world_db_dsn"`
	CharactersDBDSN string `json:"characters_db_dsn"`

	// Auth server address for bot connections
	AuthServerAddr string `json:"auth_server_addr"`

	// List of node addresses (bot runner HTTP endpoints)
	NodeAddresses []string `json:"node_addresses"`

	// Account settings
	AccountPrefix   string `json:"account_prefix"`
	AccountPassword string `json:"account_password"`
	NumBots         int    `json:"num_bots"`

	// Pathfinding
	DataDir            string `json:"data_dir"` // root dir with mmaps/, maps/, vmaps/
	PathfindingAddress string `json:"pathfinding_address"`

	// Bot defaults
	DefaultRace  uint8  `json:"default_race"`
	DefaultClass uint8  `json:"default_class"`
	DefaultMode  string `json:"default_mode"`
	DungeonName  string `json:"dungeon_name"`
	LuaScript    string `json:"lua_script"`

	// When true (orchestrator default), bots will delete existing characters
	// on the account before creating the target one.
	DeleteExistingCharacters bool `json:"delete_existing_characters"`

	// Rate limiting for spawning bots (to avoid overwhelming auth/world servers)
	// Spawn at most SpawnRateLimit bots per SpawnRateInterval.
	// Example: 100 bots per 2 seconds.
	SpawnRateLimit    int           `json:"spawn_rate_limit"`
	SpawnRateInterval time.Duration `json:"spawn_rate_interval"`

	// Whether launched bots should speak their AI decisions in /say (for debugging).
	LogDecisionsToChat bool `json:"log_decisions_to_chat"`

	// DisableTargetCache tells launched bots to skip the findBestTarget short cache.
	DisableTargetCache bool `json:"disable_target_cache"`
}

// DefaultConfig returns a config with sensible defaults for AzerothCore.
func DefaultConfig() Config {
	return Config{
		AuthDBDSN:       "acore:acore@tcp(127.0.0.1:3306)/acore_auth",
		WorldDBDSN:      "acore:acore@tcp(127.0.0.1:3306)/acore_world",
		CharactersDBDSN: "acore:acore@tcp(127.0.0.1:3306)/acore_characters",
		AuthServerAddr:  "127.0.0.1:3724",
		AccountPrefix:   "loadbot",
		AccountPassword: "loadbot",
		NumBots:         1,
		DefaultRace:     1, // Human
		DefaultClass:    1, // Warrior
		DefaultMode:     "grind",
		// Orchestrator enables clean character creation by default (delete others first)
		DeleteExistingCharacters: true,
		// Default: spawn at most 20 bots every 1 second
		SpawnRateLimit:    2,
		SpawnRateInterval: 10 * time.Second,
	}
}

// BotAssignment represents a bot assigned to a node.
type BotAssignment struct {
	NodeAddress   string `json:"node_address"`
	BotID         string `json:"bot_id"`
	AccountName   string `json:"account_name"`
	Password      string `json:"password"`
	CharacterName string `json:"character_name"`
	Race          uint8  `json:"race"`
	Class         uint8  `json:"class"`
}

// TestResult holds the aggregate result of a load test.
type TestResult struct {
	StartTime  time.Time       `json:"start_time"`
	EndTime    time.Time       `json:"end_time"`
	BotResults []BotNodeResult `json:"bot_results"`
	TotalBots  int             `json:"total_bots"`
	Errors     int             `json:"errors"`
}

// BotNodeResult is the result from a single bot on a node.
type BotNodeResult struct {
	BotID  string `json:"bot_id"`
	Status string `json:"status"`
	Level  uint32 `json:"level"`
	Kills  int    `json:"kills"`
	Deaths int    `json:"deaths"`
	Error  string `json:"error,omitempty"`
}

// Orchestrator manages account creation and bot distribution.
type Orchestrator struct {
	config Config
	authDB *sql.DB
	mu     sync.Mutex

	assignments []BotAssignment
}

// NewOrchestrator creates and initializes an orchestrator.
func NewOrchestrator(config Config) (*Orchestrator, error) {
	o := &Orchestrator{config: config}

	// For profiling high bot counts, skip DB if not accessible (assume accounts pre-created)
	if config.AuthDBDSN != "" {
		db, err := sql.Open("mysql", config.AuthDBDSN)
		if err == nil {
			db.SetMaxOpenConns(5)
			db.SetConnMaxLifetime(5 * time.Minute)
			if db.Ping() == nil {
				o.authDB = db
			} else {
				db.Close()
			}
		}
	}
	return o, nil
}

// PrepareAccounts creates or reuses bot accounts and grants GM rights.
func (o *Orchestrator) PrepareAccounts() ([]BotAssignment, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	var assignments []BotAssignment
	nodeCount := len(o.config.NodeAddresses)
	if nodeCount == 0 {
		nodeCount = 1
		o.config.NodeAddresses = []string{"local"}
	}

	skipDB := o.authDB == nil
	if skipDB {
		fmt.Println("[Orchestrator] No DB connection (profiling mode); assuming accounts and characters are pre-created.")
	}

	for i := 0; i < o.config.NumBots; i++ {
		accountName := fmt.Sprintf("%s%d", o.config.AccountPrefix, i+1)
		password := o.config.AccountPassword
		charName := generateCharName(i)
		nodeAddr := o.config.NodeAddresses[i%nodeCount]
		race, class := chooseStartingRaceClass(i)

		if !skipDB {
			// Create or update account
			if err := o.ensureAccount(accountName, password); err != nil {
				return nil, fmt.Errorf("ensure account %s: %w", accountName, err)
			}

			// Grant GM rights
			if err := o.setGMLevel(accountName, 3); err != nil {
				return nil, fmt.Errorf("set GM for %s: %w", accountName, err)
			}
		}

		assignments = append(assignments, BotAssignment{
			NodeAddress:   nodeAddr,
			BotID:         fmt.Sprintf("bot-%d", i+1),
			AccountName:   accountName,
			Password:      password,
			CharacterName: charName,
			Race:          race,
			Class:         class,
		})
	}

	o.assignments = assignments
	return assignments, nil
}

func (o *Orchestrator) ensureAccount(username, password string) error {
	usernameUpper := strings.ToUpper(username)
	passwordUpper := strings.ToUpper(password)

	// Compute SRP6 salt and verifier (matching AzerothCore format)
	salt, verifier := computeSRP6Registration(usernameUpper, passwordUpper)

	// Check if account exists
	var id int
	err := o.authDB.QueryRow("SELECT id FROM account WHERE username = ?", usernameUpper).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = o.authDB.Exec(
			`INSERT INTO account (username, salt, verifier, expansion) VALUES (?, ?, ?, 2)`,
			usernameUpper, salt, verifier,
		)
		if err != nil {
			return fmt.Errorf("create account: %w", err)
		}
		fmt.Printf("[Orchestrator] Created account: %s\n", usernameUpper)
	} else if err != nil {
		return err
	} else {
		_, err = o.authDB.Exec(
			`UPDATE account SET salt = ?, verifier = ? WHERE username = ?`,
			salt, verifier, usernameUpper,
		)
		if err != nil {
			return fmt.Errorf("update account: %w", err)
		}
		fmt.Printf("[Orchestrator] Reusing account: %s (id=%d)\n", usernameUpper, id)
	}
	return nil
}

// computeSRP6Registration generates salt and verifier matching AzerothCore's SRP6 implementation.
// verifier = g^x mod N where x = SHA1(salt || SHA1(username:password))
// generateCharName produces a WoW-legal character name for bot index i.
// WoW names must be alphabetic only (no numbers, no mixed case "languages").
// Includes a random suffix to avoid collisions with existing characters.
func generateCharName(i int) string {
	// Use a much richer syllable pool + extra random parts to make names
	// highly unique even for hundreds of bots. All alphabetic, 3-12 chars.
	consonantStarts := []string{
		"Ar", "Br", "Cr", "Dr", "El", "Fr", "Gr", "Hr", "Ir", "Jr", "Kr", "Lr", "Mr", "Nr", "Or", "Pr", "Qr", "Rr", "Sr", "Tr", "Ur", "Vr", "Wr", "Xr", "Yr", "Zr",
		"Al", "Bl", "Cl", "Dl", "Fl", "Gl", "Hl", "Kl", "Ll", "Ml", "Nl", "Pl", "Sl", "Tl", "Vl", "Wl", "Yl", "Zl",
		"An", "Bn", "Cn", "Dn", "Fn", "Gn", "Hn", "Kn", "Ln", "Mn", "Nn", "Pn", "Sn", "Tn", "Vn", "Wn", "Yn", "Zn",
		"Ak", "Bk", "Ck", "Dk", "Fk", "Gk", "Hk", "Kk", "Lk", "Mk", "Nk", "Pk", "Sk", "Tk", "Vk", "Wk", "Yk", "Zk",
		"Ag", "Bg", "Cg", "Dg", "Fg", "Gg", "Hg", "Kg", "Lg", "Mg", "Ng", "Pg", "Sg", "Tg", "Vg", "Wg", "Yg", "Zg",
		"Ad", "Bd", "Cd", "Dd", "Fd", "Gd", "Hd", "Kd", "Ld", "Md", "Nd", "Pd", "Sd", "Td", "Vd", "Wd", "Yd", "Zd",
		"Th", "Sh", "Ch", "Ph", "Wh", "Qu", "St", "Sp", "Sk", "Sm", "Sn", "Sw", "Tw", "Tr", "Dr", "Gr", "Kr", "Pr", "Br", "Fr", "Cl", "Fl", "Gl", "Pl", "Sl", "Bl",
	}
	midSyls := []string{
		"ar", "er", "ir", "or", "ur", "yr",
		"al", "el", "il", "ol", "ul", "yl",
		"an", "en", "in", "on", "un", "yn",
		"ak", "ek", "ik", "ok", "uk", "yk",
		"ag", "eg", "ig", "og", "ug", "yg",
		"ad", "ed", "id", "od", "ud", "yd",
		"ath", "eth", "ith", "oth", "uth", "yth",
		"ash", "esh", "ish", "osh", "ush", "ysh",
		"and", "end", "ind", "ond", "und", "ynd",
		"ar", "br", "cr", "dr", "fr", "gr", "kr", "pr", "tr", "vr", "wr",
		"ra", "re", "ri", "ro", "ru", "ry",
		"la", "le", "li", "lo", "lu", "ly",
		"ma", "me", "mi", "mo", "mu", "my",
		"na", "ne", "ni", "no", "nu", "ny",
		"sa", "se", "si", "so", "su", "sy",
		"ta", "te", "ti", "to", "tu", "ty",
		"va", "ve", "vi", "vo", "vu", "vy",
		"za", "ze", "zi", "zo", "zu", "zy",
	}
	endSyls := []string{
		"ar", "er", "ir", "or", "ur",
		"ath", "eth", "ith", "oth", "uth",
		"an", "en", "in", "on", "un",
		"ak", "ek", "ik", "ok", "uk",
		"al", "el", "il", "ol", "ul",
		"ad", "ed", "id", "od", "ud",
		"as", "es", "is", "os", "us",
		"and", "end", "ind", "ond", "und",
		"ard", "erd", "ird", "ord", "urd",
		"ion", "eon", "ian", "aan", "oon",
		"ius", "eus", "ias", "aas", "oos",
	}

	rb := make([]byte, 2)
	rand.Read(rb)
	r1 := int(rb[0])
	r2 := int(rb[1])

	// Build 3-4 syllable name for high uniqueness
	part1 := consonantStarts[r1%len(consonantStarts)]
	part2 := midSyls[r2%len(midSyls)]
	part3 := midSyls[(r1+r2)%len(midSyls)]
	part4 := endSyls[(r1*31+r2)%len(endSyls)]

	name := part1 + part2 + part3 + part4

	// Mix in index for additional uniqueness across sequential bots
	if i > 0 {
		extra := midSyls[i%len(midSyls)]
		name = part1 + extra + part2 + part4
	}

	// Ensure length 3-12 and alphabetic
	if len(name) > 12 {
		name = name[:12]
	}
	if len(name) < 3 {
		name = name + "ar"
	}

	// Capitalize first letter only (WoW style)
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
	}

	return name
}

func computeSRP6Registration(username, password string) ([]byte, []byte) {
	// Generate random 32-byte salt
	salt := make([]byte, 32)
	rand.Read(salt)

	// x = SHA1(salt || SHA1(username:password))
	credHash := sha1.Sum([]byte(username + ":" + password))
	xInput := make([]byte, 0, 32+20)
	xInput = append(xInput, salt...)
	xInput = append(xInput, credHash[:]...)
	xHash := sha1.Sum(xInput)

	// Convert x to big.Int (little-endian)
	xReversed := make([]byte, len(xHash))
	for i := 0; i < len(xHash); i++ {
		xReversed[i] = xHash[len(xHash)-1-i]
	}
	x := new(big.Int).SetBytes(xReversed)

	// verifier = g^x mod N
	v := new(big.Int).Exp(srp6G, x, srp6N)

	// Convert verifier to 32-byte little-endian array
	vBytes := v.Bytes()
	verifier := make([]byte, 32)
	for i := 0; i < len(vBytes) && i < 32; i++ {
		verifier[i] = vBytes[len(vBytes)-1-i]
	}

	return salt, verifier
}

func (o *Orchestrator) setGMLevel(username string, level int) error {
	usernameUpper := strings.ToUpper(username)

	var accountID int
	err := o.authDB.QueryRow("SELECT id FROM account WHERE username = ?", usernameUpper).Scan(&accountID)
	if err != nil {
		return fmt.Errorf("find account %s: %w", usernameUpper, err)
	}

	// AzerothCore uses account_access table for GM levels
	// SecurityLevel: 0=Player, 1=Moderator, 2=GameMaster, 3=Admin
	_, err = o.authDB.Exec(
		`INSERT INTO account_access (id, gmlevel, RealmID) VALUES (?, ?, -1)
		 ON DUPLICATE KEY UPDATE gmlevel = ?`,
		accountID, level, level,
	)
	if err != nil {
		return fmt.Errorf("set GM level: %w", err)
	}
	return nil
}

// LaunchWithRateLimit executes the given launch function for each assignment,
// throttling so that at most SpawnRateLimit bots are started per SpawnRateInterval.
// This is used internally for both remote and local spawning.
func (o *Orchestrator) LaunchWithRateLimit(assignments []BotAssignment, launchFn func(BotAssignment) error) error {
	if len(assignments) == 0 {
		return nil
	}

	limit := o.config.SpawnRateLimit
	interval := o.config.SpawnRateInterval

	fmt.Printf("[Orchestrator] LaunchWithRateLimit: %d assignments, limit=%d interval=%v\n", len(assignments), limit, interval)

	if limit <= 0 || interval <= 0 {
		// No rate limit configured
		fmt.Println("[Orchestrator] Rate limit disabled (limit<=0 or interval<=0), launching all immediately")
		for _, a := range assignments {
			if err := launchFn(a); err != nil {
				return err
			}
		}
		return nil
	}

	perBotDelay := interval / time.Duration(limit)
	if perBotDelay < 0 {
		perBotDelay = 0
	}
	fmt.Printf("[Orchestrator] Rate limit active: per-bot delay=%v (total window ~%v for %d)\n", perBotDelay, interval, limit)

	for i, a := range assignments {
		if i > 0 && perBotDelay > 0 {
			fmt.Printf("[Orchestrator] rate-limit: sleeping %v before bot %d/%d (%s)\n", perBotDelay, i+1, len(assignments), a.BotID)
			time.Sleep(perBotDelay)
		}
		fmt.Printf("[Orchestrator] rate-limit: launching bot %d/%d %s now\n", i+1, len(assignments), a.BotID)
		if err := launchFn(a); err != nil {
			return err
		}
		if i%200 == 0 {
			time.Sleep(time.Second * 5)
		}
	}
	return nil
}

// LaunchBots sends bot configurations to all nodes.
func (o *Orchestrator) LaunchBots(assignments []BotAssignment) error {
	remoteAssignments := make([]BotAssignment, 0, len(assignments))
	for _, a := range assignments {
		if a.NodeAddress != "local" {
			remoteAssignments = append(remoteAssignments, a)
		}
	}
	fmt.Printf("[Orchestrator] LaunchBots: %d remote assignments to rate-limit launch\n", len(remoteAssignments))

	return o.LaunchWithRateLimit(remoteAssignments, func(a BotAssignment) error {
		req := NodeLaunchRequest{
			BotID:               a.BotID,
			Username:            a.AccountName,
			Password:            a.Password,
			AuthServer:          o.config.AuthServerAddr,
			CharacterName:       a.CharacterName,
			Race:                a.Race,
			Class:               a.Class,
			Mode:                o.config.DefaultMode,
			DungeonName:         o.config.DungeonName,
			DataDir:             o.config.DataDir,
			PathfindingAddr:     o.config.PathfindingAddress,
			LuaScript:           o.config.LuaScript,
			DeleteExistingChars: true, // orchestrator always requests clean slate
			LogDecisionsToChat:  o.config.LogDecisionsToChat,
			DisableTargetCache:  o.config.DisableTargetCache,
		}

		if err := o.sendToNode(a.NodeAddress, "/launch", req); err != nil {
			return fmt.Errorf("launch bot %s on %s: %w", a.BotID, a.NodeAddress, err)
		}
		return nil
	})
}

// CollectResults queries all nodes for bot status.
func (o *Orchestrator) CollectResults() []BotNodeResult {
	var results []BotNodeResult

	for _, a := range o.assignments {
		if a.NodeAddress == "local" {
			continue
		}

		result := o.queryNodeStatus(a.NodeAddress, a.BotID)
		results = append(results, result)
	}

	return results
}

// UpdateLuaScripts sends a Lua script update to all running bots via their nodes.
func (o *Orchestrator) UpdateLuaScripts(luaCode string) error {
	for _, a := range o.assignments {
		if a.NodeAddress == "local" {
			continue
		}

		req := map[string]string{
			"bot_id":   a.BotID,
			"lua_code": luaCode,
		}
		if err := o.sendToNode(a.NodeAddress, "/lua", req); err != nil {
			fmt.Printf("[Orchestrator] Failed to update Lua on %s: %v\n", a.NodeAddress, err)
		}
	}
	return nil
}

// Close releases resources.
func (o *Orchestrator) Close() {
	if o.authDB != nil {
		o.authDB.Close()
	}
}

func (o *Orchestrator) sendToNode(nodeAddr, path string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s%s", nodeAddr, path)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("node returned %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func (o *Orchestrator) queryNodeStatus(nodeAddr, botID string) BotNodeResult {
	url := fmt.Sprintf("http://%s/status?id=%s", nodeAddr, botID)
	resp, err := http.Get(url)
	if err != nil {
		return BotNodeResult{BotID: botID, Status: "error", Error: err.Error()}
	}
	defer resp.Body.Close()

	var result BotNodeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return BotNodeResult{BotID: botID, Status: "error", Error: "decode error"}
	}
	return result
}

// chooseStartingRaceClass returns a race/class pair for bot index i so that
// characters are created on many different starting zones.
// Uses correct project race IDs: BloodElf=10, Draenei=11 (see shared/wow/race.go).
// Multiple entries to better utilize Blood Elf and Draenei starting zones.
func chooseStartingRaceClass(i int) (uint8, uint8) {
	// Distinct starting zones:
	// Alliance:
	//   1=Human (Northshire), 3=Dwarf/Gnome (Dun Morogh), 4=NightElf (Teldrassil), 11=Draenei (Azuremyst)
	// Horde:
	//   2=Orc, 8=Troll (Durotar), 6=Tauren (Mulgore), 5=Undead (Tirisfal), 10=BloodElf (Eversong)
	combos := []struct{ Race, Class uint8 }{
		// Alliance
		{1, 1},  // Human Warrior - Northshire
		{1, 2},  // Human Paladin
		{3, 1},  // Dwarf Warrior - Dun Morogh
		{3, 2},  // Dwarf Paladin
		{3, 3},  // Dwarf Hunter
		{4, 1},  // Night Elf Warrior - Teldrassil
		{4, 3},  // Night Elf Hunter
		{4, 11}, // Night Elf Druid
		{7, 1},  // Gnome Warrior - Dun Morogh
		{7, 8},  // Gnome Mage
		// Draenei - Azuremyst Isle (multiple to utilize the zone)
		{11, 1}, // Draenei Warrior
		{11, 2}, // Draenei Paladin
		{11, 3}, // Draenei Hunter
		{11, 7}, // Draenei Shaman

		// Horde
		{2, 1},  // Orc Warrior - Durotar
		{2, 3},  // Orc Hunter
		{2, 7},  // Orc Shaman
		{8, 1},  // Troll Warrior - Durotar
		{8, 3},  // Troll Hunter
		{8, 7},  // Troll Shaman
		{6, 1},  // Tauren Warrior - Mulgore
		{6, 3},  // Tauren Hunter
		{6, 7},  // Tauren Shaman
		{6, 11}, // Tauren Druid
		{5, 1},  // Undead Warrior - Tirisfal Glades
		{5, 5},  // Undead Priest
		{5, 8},  // Undead Mage
		// Blood Elf - Eversong Woods (multiple to utilize the zone)
		{10, 2}, // Blood Elf Paladin
		{10, 3}, // Blood Elf Hunter
		{10, 5}, // Blood Elf Priest
		{10, 8}, // Blood Elf Mage
	}
	c := combos[i%len(combos)]
	return c.Race, c.Class
}

// NodeLaunchRequest is what the orchestrator sends to a node to launch a bot.
type NodeLaunchRequest struct {
	BotID               string `json:"bot_id"`
	Username            string `json:"username"`
	Password            string `json:"password"`
	AuthServer          string `json:"auth_server"`
	CharacterName       string `json:"character_name"`
	Race                uint8  `json:"race"`
	Class               uint8  `json:"class"`
	Mode                string `json:"mode"`
	DungeonName         string `json:"dungeon_name"`
	DataDir             string `json:"data_dir"`
	PathfindingAddr     string `json:"pathfinding_addr"`
	LuaScript           string `json:"lua_script"`
	DeleteExistingChars bool   `json:"delete_existing_chars"`
	LogDecisionsToChat  bool   `json:"log_decisions_to_chat"`
	DisableTargetCache  bool   `json:"disable_target_cache"`
}
