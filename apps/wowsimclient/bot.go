package wowsimclient

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math"
	mathrand "math/rand"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/walkline/ToCloud9/apps/wowsimclient/behaviortree"
	"github.com/walkline/ToCloud9/apps/wowsimclient/gamedata"
	"github.com/walkline/ToCloud9/apps/wowsimclient/luaengine"
	"github.com/walkline/ToCloud9/apps/wowsimclient/navigation"
)

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// BotConfig holds configuration for a single bot instance
type BotConfig struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	AuthServer    string `json:"auth_server"`
	CharacterName string `json:"character_name"`
	RealmIndex    int    `json:"realm_index"`

	Race   uint8 `json:"race"`
	Class  uint8 `json:"class"`
	Gender uint8 `json:"gender"`

	// Navigation
	DataDir            string `json:"data_dir"` // root containing mmaps/, maps/, vmaps/ subdirs
	PathfindingAddress string `json:"pathfinding_address"`

	// Lua
	LuaScript string `json:"lua_script"`

	// Behavior mode
	Mode string `json:"mode"` // "grind", "hogger", "dungeon", "idle", "lua"

	// AI tick interval for load testing tuning (default 200ms)
	AITickMs int `json:"ai_tick_ms"`

	// Dungeon name (for dungeon mode)
	DungeonName string `json:"dungeon_name"`

	// Database DSN for character database (for pre-setup via DB)
	CharDBDSN string `json:"char_db_dsn"`

	// If true, before creating a new character, delete all existing ones on the account.
	// This is enabled by the orchestrator (for clean runs) or explicitly via --delete-existing-chars.
	DeleteExistingCharacters bool `json:"delete_existing_characters"`

	// LogDecisionsToChat makes the bot speak high-level AI decisions in /say (throttled).
	// Useful for observing what the bot is doing while watching it in-game.
	LogDecisionsToChat bool `json:"log_decisions_to_chat"`

	// DisableTargetCache disables the short-lived target cache in findBestTarget.
	// This forces a fresh scan of nearby units on every evaluation. Useful for
	// debugging cases where bots attack dead creatures or ignore live ones due to
	// stale cached target information.
	DisableTargetCache bool `json:"disable_target_cache"`
}

// BotStatus represents the current state of a bot
type BotStatus string

const (
	BotStatusIdle           BotStatus = "idle"
	BotStatusAuthenticating BotStatus = "authenticating"
	BotStatusConnecting     BotStatus = "connecting"
	BotStatusInWorld        BotStatus = "in_world"
	BotStatusDone           BotStatus = "done"
	BotStatusError          BotStatus = "error"
)

// BotResult holds the result of a bot run
type BotResult struct {
	ID     string    `json:"id"`
	Status BotStatus `json:"status"`
	Error  string    `json:"error,omitempty"`
	Level  uint32    `json:"level,omitempty"`
	Kills  int       `json:"kills,omitempty"`
	Deaths int       `json:"deaths,omitempty"`
}

// BotEvent is a notable event that occurred during the bot's life.
type BotEvent struct {
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Message string    `json:"message"`
}

// Bot implements the WoW client bot logic with behavior tree AI.
type Bot struct {
	id     string
	config BotConfig
	status BotStatus
	err    error
	mu     sync.Mutex

	world *WorldClient
	nav   navigation.Navigator
	lua   *luaengine.Engine
	tree  *behaviortree.Tree
	bb    *behaviortree.Blackboard

	// Stats
	kills  int
	deaths int
	events []BotEvent

	// Movement state is now fully delegated to the separate MovementController.
	moveController *MovementController
	isMoving       bool // mirror for quick checks; controller is source of truth

	// Current pursuit target GUID for sticky chasing of moving creatures.
	// This ensures we keep updating the destination instead of heading to a stale snapshot.
	grindTargetGUID       uint64
	lastPursuitUpdate     time.Time
	lastBetterTargetCheck time.Time
	lastMoveToTargetPos   [3]float32
	lastMoveToTargetTime  time.Time

	// lastPursuedTargetPos keeps the last known position for the current pursuit target
	// so we can continue chasing even if the object temporarily goes out of range or is removed.
	lastPursuedTargetGUID uint64
	lastPursuedTargetPos  [3]float32
	lastPursuedTargetTime time.Time

	lastMovementPacket time.Time
	targetCacheGUID    uint64
	targetCacheTime    time.Time

	// Combat state
	lastLootGUID    uint64
	lastCastTime    time.Time // GCD tracking
	lastVictoryRush bool      // Victory Rush proc available

	// For unstick from bad/dead targets we selected but never entered real combat with
	currentTargetSetAt  time.Time
	lastEngagedGUID     uint64
	engagedTargetHealth uint32 // health when we first engaged; used to detect no-progress on "live" targets

	// Decision chat throttling (so we can see high-level AI choices in-game without flooding chat)
	lastDecisionChat time.Time

	// Separate throttle for "why I think mob is alive" debug messages in chat
	lastAliveReasonChat time.Time

	// per-bot "known dead" guids. This gives each bot its own version of which objects are dead,
	// even if the live cache from server packets still has positive health (stale 8/55 etc).
	// Once we infer dead (low health no progress, dyn flag, etc.), we keep treating it dead
	// in *this bot's* view until we see a positive health update.
	knownDead   map[uint64]bool
	knownDeadMu sync.Mutex

	// Stop channel
	stopCh chan struct{}
}

// NewBot creates a new bot
func NewBot(id string, config BotConfig) *Bot {
	if config.Race == 0 {
		config.Race = 5
	}
	if config.Class == 0 {
		config.Class = 1
	}
	if config.Mode == "" {
		config.Mode = "grind"
	}
	if config.AITickMs <= 0 {
		config.AITickMs = 200
	}
	if !config.LogDecisionsToChat {
		// Default to true: decisions will be spoken in /say (throttled) so they are visible in chat.
		config.LogDecisionsToChat = true
	}
	// Force on for debugging (user requested to see chat messages)
	config.LogDecisionsToChat = true

	// DisableTargetCache defaults to false (enable the 800ms target cache).
	// Set to true via flag to force fresh scans every tick for debugging stale target issues.

	return &Bot{
		id:     id,
		config: config,
		status: BotStatusIdle,
		stopCh: make(chan struct{}),
	}
}

// Run executes the full bot flow: authenticate, connect, enter world, run AI loop.
func (b *Bot) Run() BotResult {
	b.setStatus(BotStatusAuthenticating)
	b.log("Starting bot for %s@%s, char: %s, mode: %s",
		b.config.Username, b.config.AuthServer, b.config.CharacterName, b.config.Mode)

	// Step 1: Authenticate
	authClient := NewAuthClient(b.config.Username, b.config.Password)
	realms, err := authClient.Authenticate(b.config.AuthServer)
	if err != nil {
		return b.fail("authentication failed: %v", err)
	}
	if len(realms) == 0 {
		return b.fail("no realms available")
	}

	realmIdx := b.config.RealmIndex
	if realmIdx >= len(realms) {
		realmIdx = 0
	}
	realm := realms[realmIdx]
	b.log("Authenticated. Connecting to realm: %s at %s", realm.Name, realm.Address)

	// Step 2: Connect to worldserver
	b.setStatus(BotStatusConnecting)
	b.world = NewWorldClient(b.config.Username, authClient.SessionKey(), b.log)

	if err := b.world.Connect(realm.Address); err != nil {
		return b.fail("connect to worldserver failed: %v", err)
	}

	// Set up callbacks
	charListCh := make(chan []CharEnumEntry, 1)
	charCreateCh := make(chan uint8, 1)
	b.world.OnCharList = func(chars []CharEnumEntry) {
		select {
		case charListCh <- chars:
		default:
		}
	}
	b.world.OnCharCreateResult = func(data []byte) {
		if len(data) > 0 {
			select {
			case charCreateCh <- data[0]:
			default:
			}
		}
	}
	b.world.OnKill = func(victimGUID uint64) {
		b.kills++
		victim := b.world.GetObject(victimGUID)
		name := fmt.Sprintf("GUID:%d", victimGUID)
		if victim != nil {
			name = fmt.Sprintf("Entry:%d", victim.Entry)
		}
		b.addEvent("kill", "Killed %s (total kills: %d)", name, b.kills)
		b.logDecision("Killed %s (kills=%d)", name, b.kills)
		// detailed OnKill debug removed from console (use chat decisions)

		// Immediately mark dead in cache so IsAlive() and finders see death without waiting for update packets.
		b.world.MarkObjectDead(victimGUID)
		b.markKnownDead(victimGUID)

		// Only set lastLoot if the corpse is close; we do NOT want to path across the world to loot.
		// If far, just clear states and let grind/wander take over.
		setForLoot := false
		if victim != nil {
			d := victim.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)
			// dist log removed from console
			if d <= 12.0 {
				b.lastLootGUID = victimGUID
				setForLoot = true
			}
		} else if b.world.TargetGUID() == victimGUID {
			// No obj but it was our target: don't set far loot
			b.lastLootGUID = 0
		}
		if !setForLoot && b.world.TargetGUID() == victimGUID {
			b.lastLootGUID = 0
		}

		// If this kill was our current target or we were attacking it, clear combat state NOW.
		if b.world.TargetGUID() == victimGUID {
			b.world.ClearTarget()
			b.world.ClearCombat()
			b.world.AttackStop()
			b.stopCurrentMove()
			b.currentTargetSetAt = time.Time{}
			b.lastEngagedGUID = 0
			// cleared log removed from console (chat will show via decisions)
		}

		b.lastVictoryRush = true // Victory Rush proc
	}
	b.world.OnDeath = func() {
		b.deaths++
		b.addEvent("death", "Bot died! (total deaths: %d)", b.deaths)
	}
	b.world.OnLevelUp = func(newLevel uint32) {
		b.addEvent("levelup", "Reached level %d", newLevel)
	}
	b.world.OnCombatStart = func(attacker, victim uint64) {
		if victim == b.world.charGUID || attacker == b.world.charGUID {
			b.log("Combat started (attacker: %d, victim: %d)", attacker, victim)
		}
	}
	b.world.OnInvalidTarget = func(victimGUID uint64) {
		b.markKnownDead(victimGUID)
		b.logDecision("Server says target invalid (dead/friendly/etc), marking as such GUID=%d", victimGUID)
		b.world.ClearTarget()
		b.world.ClearCombat()
	}
	b.world.OnLootOpened = func(lootGUID uint64, items []LootItem) {
		b.handleLootOpened(lootGUID, items)
	}

	// Start world client
	worldErrCh := make(chan error, 1)
	go func() {
		worldErrCh <- b.world.Run()
	}()

	time.Sleep(2 * time.Second)

	// Step 3: Character list
	b.world.SendReadyForAccountDataTimes()
	b.world.SendRealmSplit()
	if err := b.world.RequestCharList(); err != nil {
		return b.fail("request char list failed: %v", err)
	}

	var chars []CharEnumEntry
	select {
	case chars = <-charListCh:
	case <-time.After(120 * time.Second):
		return b.fail("timeout waiting for character list")
	}

	// Optional: delete all existing characters before creating (orchestrator or explicit flag only)
	if b.config.DeleteExistingCharacters && len(chars) > 0 {
		b.log("DeleteExistingCharacters enabled: deleting %d existing character(s)...", len(chars))
		for _, ch := range chars {
			if err := b.world.DeleteCharacter(ch.GUID); err != nil {
				b.log("Warning: failed to delete character %s (GUID %d): %v", ch.Name, ch.GUID, err)
			} else {
				b.log("Deleted character %s", ch.Name)
			}
			time.Sleep(150 * time.Millisecond)
		}
		// Refresh list after deletes
		time.Sleep(500 * time.Millisecond)
		b.world.SendReadyForAccountDataTimes()
		b.world.SendRealmSplit()
		if err := b.world.RequestCharList(); err != nil {
			return b.fail("request char list after delete failed: %v", err)
		}
		select {
		case chars = <-charListCh:
		case <-time.After(120 * time.Second):
			return b.fail("timeout waiting for character list after deletes")
		}
		b.log("Character list refreshed after deletes (%d remaining)", len(chars))
	}

	// Step 4: Find or create character
	var charGUID uint64
	found := false
	for _, ch := range chars {
		if strings.EqualFold(ch.Name, b.config.CharacterName) {
			charGUID = ch.GUID
			found = true
			b.log("Found character %s (GUID: %d, Level: %d)", ch.Name, ch.GUID, ch.Level)
			break
		}
	}

	if !found {
		// Generate a highly unique starting name. Orchestrator also generates one,
		// but we keep trying fresh unique names on NAME_IN_USE.
		charName := b.config.CharacterName
		var createResult uint8
		for attempt := 0; attempt < 10; attempt++ {
			if attempt > 0 {
				// Generate a completely fresh unique name instead of just appending.
				// This lets the process (or orchestrator on relaunch) keep trying until success.
				charName = generateUniqueCharName(attempt + int(time.Now().UnixNano()%1000))
			}
			b.log("Creating character %s (attempt %d)...", charName, attempt+1)
			if err := b.world.CreateCharacter(
				charName, b.config.Race, b.config.Class, b.config.Gender,
				0, 0, 0, 0, 0, 0,
			); err != nil {
				return b.fail("create character failed: %v", err)
			}

			select {
			case createResult = <-charCreateCh:
			case <-time.After(120 * time.Second):
				return b.fail("timeout waiting for character creation")
			}

			if createResult == 0x2F { // CHAR_CREATE_SUCCESS
				b.config.CharacterName = charName
				break
			}
			if createResult != 0x32 { // Not CHAR_CREATE_NAME_IN_USE
				return b.fail("character creation failed with code 0x%X", createResult)
			}
			b.log("Name %s already in use, generating a new unique name...", charName)
		}
		if createResult != 0x2F {
			return b.fail("character creation failed after retries, last code 0x%X", createResult)
		}

		b.log("Character created, requesting updated char list")
		b.world.SendReadyForAccountDataTimes()
		b.world.SendRealmSplit()
		if err := b.world.RequestCharList(); err != nil {
			return b.fail("request char list after create failed: %v", err)
		}

		select {
		case chars = <-charListCh:
		case <-time.After(120 * time.Second):
			return b.fail("timeout waiting for char list after create")
		}

		for _, ch := range chars {
			if strings.EqualFold(ch.Name, b.config.CharacterName) {
				charGUID = ch.GUID
				found = true
				break
			}
		}
		if !found {
			return b.fail("character not found after creation")
		}
	}

	// Step 5: Pre-login DB setup (set position/level before logging in)
	b.preLoginDBSetup()

	// Step 5b: Login
	b.log("Logging in with character GUID %d", charGUID)
	if err := b.world.LoginCharacter(charGUID); err != nil {
		return b.fail("login character failed: %v", err)
	}

	select {
	case <-b.world.loginDone:
	case <-time.After(120 * time.Second):
		return b.fail("timeout waiting for world login")
	case err := <-worldErrCh:
		return b.fail("world connection died during login: %v", err)
	}

	b.setStatus(BotStatusInWorld)
	x, y, z, o, m := b.world.Position()
	_ = x
	_ = y
	_ = z
	_ = o // position available for movement controller
	b.log("Character in world on map %d", m)

	time.Sleep(1 * time.Second)
	b.world.SetActiveMover(charGUID)
	time.Sleep(500 * time.Millisecond)

	b.ensureMovementController()

	// Wait a bit more for all initial data to arrive
	time.Sleep(1 * time.Second)

	// Complete any cinematic (new characters get a cinematic that may block chat)
	b.world.CompleteCinematic()
	time.Sleep(500 * time.Millisecond)

	// Step 7: Initialize navigation
	b.initNavigation()

	// Snap to real ground height right after entering the world.
	// This prevents the bot from starting under the map due to DB position
	// or login placement not being perfectly on terrain.
	if b.nav != nil {
		x, y, z, _, mapID := b.world.Position()
		probeZ := z + 5.0
		if gh, ok := b.nav.GetHeight(mapID, x, y, probeZ); ok {
			b.world.UpdatePosition(x, y, gh, b.world.orientation)
		}
	} else {
		// no nav available for ground snap
	}

	// Step 8: Initialize Lua engine
	b.lua = luaengine.NewEngine(b)
	if b.config.LuaScript != "" {
		if err := b.lua.DoFile(b.config.LuaScript); err != nil {
			b.log("Failed to load Lua script: %v", err)
		}
	}

	// Step 9: Build behavior tree
	b.bb = behaviortree.NewBlackboard()
	b.bb.Set("mode", b.config.Mode)
	b.bb.Set("setup_done", true) // Pre-setup already completed
	b.tree = behaviortree.NewTree(b.buildBehaviorTree())
	b.tree.Blackboard = b.bb

	// Step 10: Run AI loop
	b.addEvent("start", "Bot entered world, starting AI loop (mode: %s, level: %d)", b.config.Mode, b.world.PlayerLevel())

	// Log Hogger specifically when found (reduced logging)
	if b.config.Mode == "hogger" {
		b.world.OnObjectUpdate = func(guid uint64, obj *WorldObject) {
			if obj.TypeID == ObjectTypeUnit && obj.Entry == gamedata.HoggerInfo.Entry {
				b.log("Hogger update: GUID=%d HP=%d/%d Pos=(%.1f,%.1f,%.1f)",
					guid, obj.Health(), obj.MaxHealth(), obj.PosX, obj.PosY, obj.PosZ)
			}
		}
	}

	b.runAILoop(worldErrCh)

	b.world.Close()
	if b.nav != nil {
		b.nav.Close()
	}
	if b.lua != nil {
		b.lua.Close()
	}

	b.setStatus(BotStatusDone)
	b.log("Bot finished. Kills: %d, Deaths: %d", b.kills, b.deaths)

	return BotResult{
		ID:     b.id,
		Status: BotStatusDone,
		Level:  b.world.PlayerLevel(),
		Kills:  b.kills,
		Deaths: b.deaths,
	}
}

// preLoginDBSetup modifies the character's level, position, spells, and equipment
// in the database BEFORE logging in. This is called after character creation/finding
// but before LoginCharacter. The character must be offline for DB changes to take effect.
func (b *Bot) preLoginDBSetup() {
	var level int
	var posX, posY, posZ float64
	var mapID int
	needsUpdate := false

	_ = b.config // config used below

	switch b.config.Mode {
	case "hogger":
		// Hogger is level 11 elite (rank 1) with ~666 HP.
		// A real player would typically fight him around level 12-15 in a group,
		// or solo at level 15+ with decent gear. We use level 15 with full gear.
		level = 15
		// Spawn near the road between Goldshire and Hogger's area
		// This is a safe area free of hostile mobs
		posX = -9819.0
		posY = 450.0
		posZ = 34.0
		mapID = 0
		needsUpdate = true
	case "dungeon":
		dungeonName := b.config.DungeonName
		if dungeonName == "" {
			dungeonName = "ragefire_chasm"
		}
		info, ok := gamedata.Dungeons[dungeonName]
		if !ok {
			b.log("Pre-login: Unknown dungeon %s", dungeonName)
			return
		}
		level = int((info.MinLevel + info.MaxLevel) / 2)
		posX, posY, posZ = float64(info.EntranceX), float64(info.EntranceY), float64(info.EntranceZ)
		mapID = int(info.EntranceMapID)
		needsUpdate = true
	default:
		// no position override for this mode - using server start location
	}

	// Always force correct starting map/pos from gamedata for the race, to ensure correct mapID for pathfinding (esp. Blood Elf 530, Draenei 530)
	// This overrides server default if it sends wrong map (e.g. 0) for starting zones.
	startMap, startX, startY, startZ := gamedata.RaceStartPosition(b.config.Race)
	if startMap != 0 || (posX == 0 && posY == 0) { // if we have good start data
		mapID = int(startMap)
		posX = float64(startX)
		posY = float64(startY)
		posZ = float64(startZ)
		needsUpdate = true
		// forcing race start position for correct map/level in pathfinding
	}

	if !needsUpdate {
		// skipping DB update (position already correct)
		return
	}

	dsn := b.config.CharDBDSN
	if dsn == "" {
		dsn = "acore:acore@tcp(127.0.0.1:3306)/acore_characters"
	}

	db, err := openDB(dsn)
	if err != nil {
		b.log("Pre-login: Cannot connect to DB: %v", err)
		return
	}
	defer db.Close()

	charName := b.config.CharacterName
	b.log("Pre-login: Setting %s to level=%d pos=(%.1f,%.1f,%.1f) map=%d",
		charName, level, posX, posY, posZ, mapID)

	// Update level, position, clear ghost/dead flags, and set full health
	// health=1 is a placeholder; the server will cap it to maxhealth on login
	_, err = db.Exec(
		`UPDATE characters SET level=?, position_x=?, position_y=?, position_z=?, map=?,
		 health=99999, power1=99999, playerFlags=playerFlags&~(16|32)
		 WHERE name=? AND online=0`,
		level, posX, posY, posZ, mapID, charName,
	)
	if err != nil {
		b.log("Pre-login: DB update failed: %v", err)
		return
	}
	_ = charName // logged via normal flow if needed

	// Get character GUID for spell/item setup
	var charGUID uint64
	err = db.QueryRow("SELECT guid FROM characters WHERE name=?", charName).Scan(&charGUID)
	if err != nil {
		b.log("Pre-login: Can't find character GUID: %v", err)
		return
	}

	// Give the character appropriate spells for their level
	b.grantSpells(db, charGUID, uint32(level))

	// Give the character equipment appropriate for their level
	b.grantEquipment(db, charGUID, uint32(level))

	b.log("Pre-login: Setup complete for character GUID %d", charGUID)
}

// grantSpells inserts all necessary warrior spells for the given level.
func (b *Bot) grantSpells(db *sql.DB, charGUID uint64, level uint32) {
	allSpells := []uint32{}
	for id, info := range gamedata.WarriorSpells {
		if info.Level <= level {
			allSpells = append(allSpells, id)
		}
	}
	for _, spellID := range allSpells {
		db.Exec("INSERT IGNORE INTO character_spell (guid, spell, specMask) VALUES (?, ?, 255)", charGUID, spellID)
	}
	// Armor proficiencies
	proficiencies := []uint32{
		9116,  // Shield proficiency
		750,   // Plate Mail (not available until 40 but add anyway)
		8737,  // Mail armor proficiency
		9077,  // Leather proficiency
		9078,  // Cloth proficiency
		196,   // One-Handed Axes
		197,   // Two-Handed Axes
		198,   // One-Handed Maces
		199,   // Two-Handed Maces
		200,   // Polearms
		201,   // One-Handed Swords
		202,   // Two-Handed Swords
		227,   // Staves
		264,   // Bows
		5011,  // Crossbows
		266,   // Guns
		15590, // Fist Weapons
	}
	for _, spellID := range proficiencies {
		db.Exec("INSERT IGNORE INTO character_spell (guid, spell, specMask) VALUES (?, ?, 255)", charGUID, spellID)
	}
	b.log("Pre-login: %d spells + proficiencies added for level %d", len(allSpells), level)
}

// grantEquipment gives the character a set of appropriate gear via direct DB inserts.
// This creates item_instance entries and character_inventory entries for equipped slots.
func (b *Bot) grantEquipment(db *sql.DB, charGUID uint64, level uint32) {
	// Equipment loadout: slot -> itemEntry
	// Using green/blue mail items appropriate for level 15 warrior
	equipment := map[int]uint32{
		// slot 4 = chest: Ironforge Breastplate (entry 6731, mail, armor 198, iLvl 20)
		4: 6731,
		// slot 6 = legs: Foreman's Leggings (entry 2166, mail, armor 147, iLvl 20)
		6: 2166,
		// slot 7 = feet: Silver-linked Footguards (entry 12982, mail, armor 129, iLvl 21)
		7: 12982,
		// slot 8 = wrists: Cavedweller Bracers (entry 14147, mail, armor 78, iLvl 18)
		8: 14147,
		// slot 9 = hands: Polar Gauntlets (entry 7606, mail, armor 109, iLvl 22)
		9: 7606,
		// slot 5 = waist: Stormbringer Belt (entry 12978, mail, armor 104, iLvl 20)
		5: 12978,
		// slot 2 = shoulders: Rough Bronze Shoulders (entry 3480, mail, armor 130, iLvl 22)
		2: 3480,
		// slot 15 = main hand (2H): Rhahk'Zor's Hammer (entry 5187, 2H mace, 45-68 dmg, iLvl 20)
		15: 5187,
	}

	// Check which slots already have items
	rows, err := db.Query(
		"SELECT slot FROM character_inventory WHERE guid=? AND bag=0 AND slot < 19",
		charGUID,
	)
	if err != nil {
		b.log("Pre-login: Failed to query inventory: %v", err)
		return
	}
	equippedSlots := make(map[int]bool)
	for rows.Next() {
		var slot int
		rows.Scan(&slot)
		equippedSlots[slot] = true
	}
	rows.Close()

	// Get the next available item_instance GUID (table has no auto_increment)
	var maxItemGUID uint32
	err = db.QueryRow("SELECT COALESCE(MAX(guid), 0) FROM item_instance").Scan(&maxItemGUID)
	if err != nil {
		b.log("Pre-login: Failed to get max item GUID: %v", err)
		return
	}
	nextItemGUID := maxItemGUID + 1

	enchantments := "0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 "
	itemsAdded := 0
	for slot, itemEntry := range equipment {
		if equippedSlots[slot] {
			continue // Already has an item in this slot
		}

		itemGUID := nextItemGUID
		nextItemGUID++

		// Create item_instance with explicit GUID
		_, err := db.Exec(
			"INSERT INTO item_instance (guid, itemEntry, owner_guid, count, durability, enchantments) VALUES (?, ?, ?, 1, 100, ?)",
			itemGUID, itemEntry, charGUID, enchantments,
		)
		if err != nil {
			b.log("Pre-login: Failed to create item %d (guid %d): %v", itemEntry, itemGUID, err)
			continue
		}

		// Put item in equipped slot
		_, err = db.Exec(
			"INSERT INTO character_inventory (guid, bag, slot, item) VALUES (?, 0, ?, ?)",
			charGUID, slot, itemGUID,
		)
		if err != nil {
			b.log("Pre-login: Failed to equip item %d to slot %d: %v", itemEntry, slot, err)
			db.Exec("DELETE FROM item_instance WHERE guid=?", itemGUID)
			continue
		}
		itemsAdded++
	}
	b.log("Pre-login: Equipped %d items for character", itemsAdded)
}

func (b *Bot) initNavigation() {
	if b.config.PathfindingAddress != "" {
		nav, err := navigation.NewRemoteNavigator(b.config.PathfindingAddress)
		if err != nil {
			b.log("Failed to connect to pathfinding service: %v, falling back to embedded", err)
		} else {
			b.nav = nav
			b.log("Using remote pathfinding service at %s", b.config.PathfindingAddress)
			return
		}
	}
	if b.config.DataDir != "" {
		b.nav = navigation.NewEmbeddedNavigator(b.config.DataDir)
		b.log("Using embedded pathfinding with data dir %s", b.config.DataDir)
	} else {
		b.log("No pathfinding configured, movement will be direct")
	}
}

func (b *Bot) ensureMovementController() {
	if b.moveController != nil || b.world == nil {
		return
	}
	sender := &worldMovementSender{w: b.world}
	cfg := DefaultMovementConfig()
	// Use slightly more frequent HB during bot AI for responsiveness, but still client-like
	cfg.HeartbeatInterval = 400 * time.Millisecond
	speed := b.world.MoveSpeed()
	if speed <= 0 {
		speed = 7.0
	}
	b.moveController = NewMovementController(sender, speed, b.nav, cfg)
	// Seed the controller with the real position we just got from the server so we don't start at (0,0,0)
	cx, cy, cz, co, _ := b.world.Position()
	b.moveController.initPositionFromWorld(cx, cy, cz, co)
}

func (b *Bot) runAILoop(worldErrCh chan error) {
	tick := time.Duration(b.config.AITickMs) * time.Millisecond
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	heartbeatTicker := time.NewTicker(5 * time.Second)
	defer heartbeatTicker.Stop()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()
	var pingSeq uint32

	for {
		select {
		case <-b.stopCh:
			return
		case err := <-worldErrCh:
			if err != nil {
				b.log("World connection error: %v", err)
			}
			return
		case <-heartbeatTicker.C:
			// Only force a keepalive when not actively driving movement via updateMovement.
			// Frequent HBs for smooth movement are sent from the AI tick / updateMovement path.
			if !b.isMoving {
				b.world.SendHeartbeat()
			}
		case <-pingTicker.C:
			pingSeq++
			b.world.SendPing(pingSeq)
		case <-ticker.C:
			tickStart := time.Now()
			if b.world != nil {
				// Very verbose position debug on every tick (can be noisy, but useful to catch teleports)
				// px, py, pz, po, pmid := b.world.Position()
				// if time.Since(b.lastDecisionChat) > 2*time.Second {
				// 	b.log("[DEBUG-POS] tick pos: map=%d (%.1f,%.1f,%.1f) o=%.2f", pmid, px, py, pz, po)
				// }
			}
			// Lua tick gets priority
			if b.lua != nil && b.lua.CallTick() {
				b.updateMovement()
				elapsed := time.Since(tickStart)
				if elapsed > 300*time.Millisecond {
					ms := elapsed.Milliseconds()
					b.log("AI tick update took %dms (lua path)", ms)
					if b.config.LogDecisionsToChat {
						chat := fmt.Sprintf("[TICK] %dms", ms)
						_ = b.world.SendChatMessage(ChatMsgSay, LangCommon, chat)
					}
				}
				continue
			}

			// Movement update
			b.updateMovement()

			// Persistent live pursuit: keep path to current target's live position if far.
			// Throttle re-path to ~300ms to avoid jitter and high CPU from re-SetPath every tick.
			// This prevents stale paths (reaching old pos while mob moved) and smooths movement.
			if guid := b.world.TargetGUID(); guid != 0 {
				if t := b.world.GetObject(guid); t != nil && t.IsAlive() && !b.isKnownDead(guid) {
					tx, ty, tz := t.InterpolatedPosition()
					b.lastPursuedTargetGUID = guid
					b.lastPursuedTargetPos = [3]float32{tx, ty, tz}
					b.lastPursuedTargetTime = time.Now()
					dx := tx - b.world.posX
					dy := ty - b.world.posY
					dist2d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
					targetMoved := b.lastMoveToTargetTime.IsZero() ||
						(math.Abs(float64(tx-b.lastMoveToTargetPos[0])) > 3.0 ||
							math.Abs(float64(ty-b.lastMoveToTargetPos[1])) > 3.0 ||
							math.Abs(float64(tz-b.lastMoveToTargetPos[2])) > 3.0)
					if dist2d > 2.5 && (b.lastPursuitUpdate.IsZero() || time.Since(b.lastPursuitUpdate) > 1000*time.Millisecond || targetMoved) {
						b.moveToPoint(tx, ty, tz)
						b.lastPursuitUpdate = time.Now()
						b.lastMoveToTargetPos = [3]float32{tx, ty, tz}
						b.lastMoveToTargetTime = time.Now()
					}
				} else if b.lastPursuedTargetGUID == guid && time.Since(b.lastPursuedTargetTime) < 30*time.Second {
					// Use last known pos to continue pursuit even if object temporarily not visible
					tx, ty, tz := b.lastPursuedTargetPos[0], b.lastPursuedTargetPos[1], b.lastPursuedTargetPos[2]
					dx := tx - b.world.posX
					dy := ty - b.world.posY
					dist2d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
					if dist2d > 2.5 {
						b.moveToPoint(tx, ty, tz)
						b.lastMoveToTargetPos = [3]float32{tx, ty, tz}
						b.lastMoveToTargetTime = time.Now()
					}
				}
			}

			// Cleanup: if current target is dead or gone, clear it so we don't stay "looking"
			// at a dead creature and can fall through to wander/explore.
			// Also clear lastLootGUID for dead/gone loot targets so we don't get stuck on them.
			if tg := b.world.TargetGUID(); tg != 0 {
				t := b.world.GetObject(tg)
				if t == nil || !t.IsAlive() {
					b.world.MarkObjectDead(tg)
					b.world.ClearTarget()
					b.world.ClearCombat()
					b.stopCurrentMove()
					if b.lastLootGUID == tg {
						b.lastLootGUID = 0
					}
				} else if t.value(UnitNPCFlags) != 0 || !b.isHostileFaction(t.value(UnitFieldFaction)) {
					// Drop friendly vendors/NPCs even if we had them targeted (data may update late or target came from elsewhere)
					b.world.ClearTarget()
					b.world.ClearCombat()
					b.stopCurrentMove()
					if b.lastLootGUID == tg {
						b.lastLootGUID = 0
					}
				}
			}
			if lg := b.lastLootGUID; lg != 0 {
				obj := b.world.GetObject(lg)
				if obj == nil || !obj.IsAlive() {
					b.lastLootGUID = 0
				}
			}

			// Unstick from a selected target that never led to combat (may be dead on server but cache stale, or unattackable)
			if tg := b.world.TargetGUID(); tg != 0 && !b.currentTargetSetAt.IsZero() {
				if time.Since(b.currentTargetSetAt) > 12*time.Second && !b.world.InCombat() {
					tgo := b.world.GetObject(tg)
					currH := uint32(0)
					if tgo != nil {
						currH = tgo.Health()
					}
					noProgress := b.engagedTargetHealth == 0 || currH >= b.engagedTargetHealth
					if noProgress || tgo == nil || !tgo.IsAlive() {
						b.world.ClearTarget()
						b.world.ClearCombat()
						b.stopCurrentMove()
						b.currentTargetSetAt = time.Time{}
						b.lastEngagedGUID = 0
						b.engagedTargetHealth = 0
					}
				}
			}

			// Behavior tree tick
			// NOTE: We removed the periodic tick snap here. The MovementController snaps Z
			// on every Update/advance (live during movement) and at SetPath. This prevents
			// occasional large "drastic" corrections in the tick when the bot has moved over
			// terrain with height change. Decisions use the world pos which is kept current
			// by the controller.
			b.tree.Tick()

			elapsed := time.Since(tickStart)
			if elapsed > 300*time.Millisecond {
				ms := elapsed.Milliseconds()
				b.log("AI tick update took %dms", ms)
				if b.config.LogDecisionsToChat {
					chat := fmt.Sprintf("[TICK] %dms", ms)
					_ = b.world.SendChatMessage(ChatMsgSay, LangCommon, chat)
				}
			}

			// Periodic state - only to chat on major decisions, not every tick (console unreadable at scale)
			// removed the every-tick b.log DEBUG STATE spam
			_ = b.world.TargetGUID() // keep variable usage minimal if needed elsewhere

			// Periodic status log removed from console (noisy at scale; use chat decisions)
			// if needed, decisions will surface key state via logDecision to /say

		}
	}
}

// ============================================================
// Behavior tree construction
// ============================================================

func (b *Bot) buildBehaviorTree() behaviortree.Node {
	return behaviortree.NewSelector("root",
		// Priority 1: Handle death
		behaviortree.NewSequence("handle_death",
			behaviortree.NewCondition("is_dead", func(bb *behaviortree.Blackboard) bool {
				return !b.IsAlive() || b.deaths > 0 && b.world.Health() == 0
			}),
			behaviortree.NewAction("release_and_respawn", func(bb *behaviortree.Blackboard) behaviortree.Status {
				deathTime := bb.GetInt("death_time")
				now := int(time.Now().Unix())
				if deathTime == 0 {
					// First tick after death: release spirit
					b.log("Dead, releasing spirit...")
					bb.Set("death_time", now)
					// Stop any movement
					b.isMoving = false
					if b.moveController != nil {
						b.moveController.Stop(time.Now())
					}
					b.world.AttackStop()
					b.world.RepopRequest()
					return behaviortree.Running
				}
				elapsed := now - deathTime
				if elapsed < 32 {
					// Wait for corpse reclaim timer (30s default in AzerothCore)
					if elapsed%5 == 0 {
						b.log("Waiting to reclaim corpse (%ds elapsed, need 30s)...", elapsed)
					}
					return behaviortree.Running
				}
				// Try to reclaim corpse
				b.log("Reclaiming corpse after %ds...", elapsed)
				b.world.ReclaimCorpse()
				// Reset death tracking after a delay for server to process
				bb.Set("death_time", 0)
				return behaviortree.Success
			}),
		),

		// Priority 2: Loot nearby corpses (only if the corpse is already close; we never path to dead)
		behaviortree.NewSequence("loot_nearby",
			behaviortree.NewCondition("has_lootable_target", func(bb *behaviortree.Blackboard) bool {
				if b.lastLootGUID == 0 {
					return false
				}
				obj := b.world.GetObject(b.lastLootGUID)
				if obj == nil || !obj.IsAlive() {
					return false
				}
				d := obj.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)
				closeEnough := d <= 10.0
				if !closeEnough {
					b.logDecision("loot cond: corpse too far dist=%.1f", d)
				}
				return closeEnough
			}),
			behaviortree.NewAction("loot_target", func(bb *behaviortree.Blackboard) behaviortree.Status {
				return b.actionLoot()
			}),
		),

		// Priority 3: Fight current target
		behaviortree.NewSequence("fight_target",
			behaviortree.NewCondition("in_combat", func(bb *behaviortree.Blackboard) bool {
				return b.world.InCombat()
			}),
			behaviortree.NewAction("combat_rotation", func(bb *behaviortree.Blackboard) behaviortree.Status {
				return b.actionCombatRotation()
			}),
		),

		// Priority 4: Mode-specific behavior
		b.buildModeBehavior(),
	)
}

func (b *Bot) buildModeBehavior() behaviortree.Node {
	switch b.config.Mode {
	case "hogger":
		return b.buildHoggerBehavior()
	case "dungeon":
		return b.buildDungeonBehavior()
	case "idle":
		return behaviortree.NewAction("idle", func(bb *behaviortree.Blackboard) behaviortree.Status {
			return behaviortree.Running
		})
	case "lua":
		return behaviortree.NewAction("lua_control", func(bb *behaviortree.Blackboard) behaviortree.Status {
			return behaviortree.Running
		})
	default: // "grind"
		return b.buildGrindBehavior()
	}
}

func (b *Bot) buildGrindBehavior() behaviortree.Node {
	return behaviortree.NewSelector("grind",
		// Find and attack nearby mobs
		behaviortree.NewSequence("find_and_fight",
			behaviortree.NewCondition("find_target", func(bb *behaviortree.Blackboard) bool {
				// Stick to current pursuit target even if it has moved out of initial scan range.
				// This prevents the bot from committing to an old position and waiting there
				// while the creature has walked away.
				if b.grindTargetGUID != 0 {
					if t := b.world.GetObject(b.grindTargetGUID); t != nil && t.IsAlive() && !b.isKnownDead(b.grindTargetGUID) {
						// re-validate not friendly
						if t.value(UnitNPCFlags) == 0 && b.isHostileFaction(t.value(UnitFieldFaction)) {
							return true
						}
					}
					b.grindTargetGUID = 0
				}

				target := b.findBestTarget(38)
				if target != nil {
					b.grindTargetGUID = target.GUID
					d := target.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)
					deadF := (target.Values[UnitFieldFlags] & UnitFlagDead) != 0
					b.sendAliveReasonChat("FOUND target Entry=%d dist=%.1fyd: h=%d/%d IsAlive=%v deadFlag=%v f=0x%x npc=0x%x fac=%d",
						target.Entry, d, target.Health(), target.MaxHealth(), target.IsAlive(), deadF, target.Values[UnitFieldFlags], target.Values[UnitNPCFlags], target.Values[UnitFieldFaction])
					return true
				}
				b.logDecision("No suitable target, wandering to find mobs")
				return false
			}),
			behaviortree.NewAction("engage_target", func(bb *behaviortree.Blackboard) behaviortree.Status {
				return b.actionEngageTarget(b.grindTargetGUID)
			}),
		),

		// Wander to find mobs
		behaviortree.NewAction("wander", func(bb *behaviortree.Blackboard) behaviortree.Status {
			return b.actionWander()
		}),
	)
}

func (b *Bot) buildHoggerBehavior() behaviortree.Node {
	var hoggerGUID uint64
	hoggerKilled := false
	battleShoutUsed := false
	return behaviortree.NewSelector("hogger_hunt",
		// Priority 1: If Hogger is killed, we're done
		behaviortree.NewSequence("hogger_done_check",
			behaviortree.NewCondition("hogger_killed", func(bb *behaviortree.Blackboard) bool {
				return hoggerKilled
			}),
			behaviortree.NewAction("celebrate", func(bb *behaviortree.Blackboard) behaviortree.Status {
				b.addEvent("hogger_kill", "Hogger has been slain!")
				return behaviortree.Running // Stay alive for logging
			}),
		),

		// Priority 2: Find and fight Hogger
		behaviortree.NewSequence("find_hogger",
			behaviortree.NewCondition("hogger_visible", func(bb *behaviortree.Blackboard) bool {
				units := b.world.GetNearbyUnits(80)
				for _, u := range units {
					if u.Entry == gamedata.HoggerInfo.Entry {
						if u.IsAlive() {
							hoggerGUID = u.GUID
							return true
						}
						// Hogger dead = we killed him (or he died)
						hoggerKilled = true
					}
				}
				return false
			}),
			behaviortree.NewAction("attack_hogger", func(bb *behaviortree.Blackboard) behaviortree.Status {
				// Pre-combat: use Battle Shout before engaging
				if !battleShoutUsed {
					if b.world.IsSpellReady(6673) {
						b.log("Pre-combat: casting Battle Shout")
						b.world.CastSpell(6673, 0)
					} else if b.world.IsSpellReady(2457) {
						b.log("Pre-combat: casting Battle Shout (Rank 1)")
						b.world.CastSpell(2457, 0)
					}
					battleShoutUsed = true
				}

				target := b.world.GetObject(hoggerGUID)
				if target == nil || !target.IsAlive() {
					return behaviortree.Failure
				}

				dist := target.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)

				// Use Charge if in range (8-25 yards) and not in combat
				if dist >= 8 && dist <= 25 && !b.world.InCombat() && b.world.IsSpellReady(100) {
					b.log("Charging Hogger! dist=%.1f", dist)
					b.world.SetTarget(hoggerGUID)
					b.world.CastSpell(100, hoggerGUID) // Charge
					return behaviortree.Running
				}

				return b.actionEngageTarget(hoggerGUID)
			}),
		),

		// Priority 3: Fight anything that attacks us (clear adds before Hogger)
		behaviortree.NewSequence("fight_attackers",
			behaviortree.NewCondition("being_attacked", func(bb *behaviortree.Blackboard) bool {
				return b.world.InCombat()
			}),
			behaviortree.NewAction("fight_back", func(bb *behaviortree.Blackboard) behaviortree.Status {
				return b.actionCombatRotation()
			}),
		),

		// Priority 4: Move toward Hogger's spawn while waiting
		behaviortree.NewAction("move_to_hogger_area", func(bb *behaviortree.Blackboard) behaviortree.Status {
			hx := float32(gamedata.HoggerInfo.PosX)
			hy := float32(gamedata.HoggerInfo.PosY)
			hz := float32(gamedata.HoggerInfo.PosZ)
			dist := float32(math.Sqrt(float64(
				(hx-b.world.posX)*(hx-b.world.posX) +
					(hy-b.world.posY)*(hy-b.world.posY))))
			if dist > 10 {
				b.moveToPoint(hx, hy, hz)
				return behaviortree.Running
			}
			return b.actionWander()
		}),
	)
}

func (b *Bot) buildDungeonBehavior() behaviortree.Node {
	var dungeonTargetGUID uint64
	// Setup is already done in preSetup(), just fight and explore
	return behaviortree.NewSelector("dungeon",
		// Priority 1: Fight current target in combat
		behaviortree.NewSequence("dungeon_in_combat",
			behaviortree.NewCondition("in_combat", func(bb *behaviortree.Blackboard) bool {
				return b.world.InCombat()
			}),
			behaviortree.NewAction("dungeon_combat_rotation", func(bb *behaviortree.Blackboard) behaviortree.Status {
				return b.actionCombatRotation()
			}),
		),

		// Priority 2: Find and engage enemies
		behaviortree.NewSequence("find_dungeon_mob",
			behaviortree.NewCondition("enemy_in_range", func(bb *behaviortree.Blackboard) bool {
				target := b.findBestTarget(40)
				if target != nil {
					dungeonTargetGUID = target.GUID
					return true
				}
				return false
			}),
			behaviortree.NewAction("fight_dungeon_mob", func(bb *behaviortree.Blackboard) behaviortree.Status {
				return b.actionEngageTarget(dungeonTargetGUID)
			}),
		),

		// Priority 3: Explore dungeon
		behaviortree.NewAction("explore_dungeon", func(bb *behaviortree.Blackboard) behaviortree.Status {
			return b.actionWander()
		}),
	)
}

// ============================================================
// Bot actions (used by behavior tree)
// ============================================================

func (b *Bot) findBestTarget(maxDist float32) *WorldObject {
	// noisy enter log removed - console unreadable at scale.
	// decisions + alive reasons go via logDecision / sendAliveReasonChat to chat.
	now := time.Now()

	// When DisableTargetCache is set we always do a full fresh scan.
	// This helps when the bot is attacking dead creatures (stale "alive" in cache)
	// or wandering while live mobs are nearby (stale "no target" decision).
	if !b.config.DisableTargetCache && b.targetCacheGUID != 0 {
		// Always fetch fresh object to get current position and state.
		// Using cached object snapshot was causing stale positions for moving mobs,
		// leading to attacking "mobs that were not there" or at old locations.
		c := b.world.GetObject(b.targetCacheGUID)
		if c == nil {
			b.targetCacheGUID = 0
		} else if b.isKnownDead(c.GUID) {
			if c.Health() > 0 {
				b.clearKnownDead(c.GUID)
			} else {
				b.targetCacheGUID = 0
			}
		} else if !c.IsAlive() {
			b.targetCacheGUID = 0
		} else if now.Sub(b.targetCacheTime) < 800*time.Millisecond {
			d := c.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)
			if d <= maxDist {
				// Re-check npc flags / faction in case data updated since cache
				npcf := c.value(UnitNPCFlags)
				fac := c.value(UnitFieldFaction)
				if npcf != 0 || !b.isHostileFaction(fac) {
					b.targetCacheGUID = 0
				} else {
					return c
				}
			}
		}
	}

	units := b.world.GetNearbyUnits(maxDist)
	skippedDead := 0
	skippedNotHostile := 0
	skippedFriendlyNPC := 0
	skippedLevel := 0
	skippedLowHP := 0

	var candidates []*WorldObject
	for _, u := range units {
		if b.isKnownDead(u.GUID) {
			if u.Health() > 0 {
				b.clearKnownDead(u.GUID)
				// fall through, treat as possibly alive now
			} else {
				skippedDead++
				continue
			}
		}
		if !u.IsAlive() {
			skippedDead++
			continue
		}
		flags := u.value(UnitFieldFlags)
		if flags&UnitFlagNotAttackable != 0 {
			skippedNotHostile++
			continue
		}

		// Skip NPCs that have interaction flags (quest givers, vendors, trainers, guards, gossip, etc.).
		// These are almost always friendly and should never be attacked.
		npcFlags := u.value(UnitNPCFlags)
		if npcFlags != 0 {
			skippedFriendlyNPC++
			continue
		}

		faction := u.value(UnitFieldFaction)
		hostile := b.isHostileFaction(faction)
		if !hostile {
			skippedNotHostile++
			continue
		}
		ulevel := u.Level()
		myLevel := b.world.PlayerLevel()
		// Allow a bit wider level range so fresh low-level bots in starting zones
		// can actually find and prioritize killing appropriate mobs instead of
		// only wandering.
		if ulevel > myLevel+6 {
			skippedLevel++
			continue
		}
		if u.MaxHealth() <= 1 {
			skippedLowHP++
			continue
		}
		candidates = append(candidates, u)
	}

	var best *WorldObject
	if len(candidates) > 0 {
		// Pick a random candidate to spread bots across different mobs.
		// This prevents all bots targeting the same mob and its position at the time of selection (outdated location).
		idx := mathrand.Intn(len(candidates))
		best = candidates[idx]
	}

	if best != nil {
		bestDist := best.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)
		// send useful selection info to chat instead of console spam
		b.logDecision("findBestTarget chose Entry=%d dist=%.1f (skipped dead=%d notHostile=%d friendlyNPC=%d level=%d lowHP=%d total=%d)", best.Entry, bestDist, skippedDead, skippedNotHostile, skippedFriendlyNPC, skippedLevel, skippedLowHP, len(units))
	} else {
		b.logDecision("findBestTarget no target (skipped dead=%d notHostile=%d friendlyNPC=%d level=%d lowHP=%d total=%d)", skippedDead, skippedNotHostile, skippedFriendlyNPC, skippedLevel, skippedLowHP, len(units))
	}

	b.targetCacheGUID = 0
	if best != nil {
		b.targetCacheGUID = best.GUID
	}
	b.targetCacheTime = now
	return best
}

// isHostileFaction checks if a faction template ID is hostile to the player.
// Uses known hostile faction IDs from AzerothCore's factiontemplate_dbc.
func (b *Bot) isHostileFaction(factionTemplate uint32) bool {
	// Explicit friendly / neutral factions that should NEVER be attacked
	// (quest NPCs, vendors, guards, city factions, trainers, etc.)
	switch factionTemplate {
	case 35: // "Friendly" - commonly used for many helpful NPCs
		return false
	case 11, 12, 13: // Common city/guard factions (Stormwind, Ironforge, etc.)
		return false
	case 55, 57, 59, 60: // Other common friendly/neutral
		return false
	case 4, 5, 6, 161, 162: // Additional common starting area / city friendly factions
		return false
	}

	// Known hostile faction templates from AzerothCore:
	// These have FACTION_TEMPLATE_FLAG_HOSTILE_BY_DEFAULT or are Monster factions
	switch factionTemplate {
	case 7: // Defias Brotherhood
		return true
	case 14: // Monster (generic hostile)
		return true
	case 16: // Monster (hostile to all)
		return true
	case 17: // Defias Brotherhood
		return true
	case 20: // Redridge Gnolls
		return true
	case 21: // Gnoll - Riverpaw
		return true
	case 22: // Undead, Scourge
		return true
	case 24: // Beast - Ravager
		return true
	case 25: // Monster (Kobolds etc)
		return true
	case 26: // Defias
		return true
	case 28: // Murloc
		return true
	case 29: // Gnoll - Shadowhide
		return true
	case 32: // Monster (Diseased wolves etc)
		return true
	case 33: // Gnoll - Mosshide
		return true
	case 34: // Monster (hostile to alliance)
		return true
	case 45: // Ogre
		return true
	case 48: // Pirate
		return true
	case 49: // Dalaran
		return true
	case 51: // Syndicate
		return true
	case 54: // Murloc (hostile)
		return true
	case 57: // Lost Ones
		return true
	case 66: // Blackrock
		return true
	case 73: // Dark Iron Dwarves
		return true
	case 80: // Blackfathom
		return true
	case 83: // Scorpid
		return true
	case 87: // Bloodsail Buccaneers
		return true
	case 90: // Burning Blade
		return true
	case 93: // Flamekin
		return true
	case 168: // Enemy (generic)
		return true
	}
	// Default to hostile.
	// Combined with the npcFlags check above (which skips almost all friendly NPCs),
	// this lets us attack wild monsters even if their faction ID is not in the explicit list.
	return true
}

func (b *Bot) actionEngageTarget(guid uint64) behaviortree.Status {
	// Prefer fresh scan but throttle to avoid CPU from full scans + pathing every tick.
	// Check for better target only every 2s or so.
	if b.lastBetterTargetCheck.IsZero() || time.Since(b.lastBetterTargetCheck) > 2*time.Second {
		b.lastBetterTargetCheck = time.Now()
		if fresh := b.findBestTarget(40); fresh != nil && fresh.IsAlive() {
			freshDist := fresh.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)
			oldDist := float32(9999)
			currentObj := b.world.GetObject(guid)
			if currentObj != nil {
				oldDist = currentObj.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)
			}
			currentAlive := currentObj != nil && currentObj.IsAlive()
			// Switch if the passed guid is bad or the fresh one is significantly closer/better.
			// Note: GetObject can return nil if the object was destroyed/expired (e.g. dead mob cleaned up).
			if guid == 0 || !currentAlive || freshDist < oldDist*0.8 {
				guid = fresh.GUID
			}
		}
	}
	b.grindTargetGUID = guid

	target := b.world.GetObject(guid)
	if target == nil || !target.IsAlive() || b.isKnownDead(guid) {
		b.stopCurrentMove()
		b.world.ClearTarget()
		b.world.ClearCombat()
		b.markKnownDead(guid)
		b.grindTargetGUID = 0
		return behaviortree.Failure
	}
	if target.value(UnitNPCFlags) != 0 || !b.isHostileFaction(target.value(UnitFieldFaction)) {
		b.sendAliveReasonChat("ABORT friendly NPC GUID=%d Entry=%d npc=0x%x fac=%d", guid, target.Entry, target.value(UnitNPCFlags), target.value(UnitFieldFaction))
		b.world.ClearTarget()
		b.world.ClearCombat()
		b.stopCurrentMove()
		b.grindTargetGUID = 0
		return behaviortree.Failure
	}
	// Print to chat the reason we think it's alive (for debugging dead creature attacks)
	deadF := (target.value(UnitFieldFlags) & UnitFlagDead) != 0
	b.sendAliveReasonChat("ENGAGE GUID=%d Entry=%d: h=%d/%d IsAlive=%v deadFlag=%v flags=0x%x npc=0x%x fac=%d",
		guid, target.Entry, target.Health(), target.MaxHealth(), target.IsAlive(), deadF,
		target.value(UnitFieldFlags), target.value(UnitNPCFlags), target.value(UnitFieldFaction))

	// Unstick: if we have been "looking at" this target for >6s without entering real combat, drop it.
	// Prevents standing forever on dead/stuck/unattackable mobs that our cache still thinks alive.
	age := time.Duration(0)
	if !b.currentTargetSetAt.IsZero() {
		age = time.Since(b.currentTargetSetAt)
	}
	currHealth := uint32(0)
	if target != nil {
		currHealth = target.Health()
	}
	noProgress := b.engagedTargetHealth == 0 || currHealth >= b.engagedTargetHealth

	// Act faster on low health targets that show no progress (likely dead but health not updated to 0 in cache)
	unstickThreshold := 12 * time.Second
	if currHealth > 0 && currHealth < 20 {
		unstickThreshold = 2500 * time.Millisecond
	}
	if age > unstickThreshold {
		if noProgress {
			if currHealth > 0 && currHealth < 20 {
				b.markKnownDead(guid)
				b.logDecision("Forcing dead on low-health no-progress target h=%d", currHealth)
			}
			b.logDecision("Unsticking from target (no progress)")
			b.world.ClearTarget()
			b.world.ClearCombat()
			b.stopCurrentMove()
			b.currentTargetSetAt = time.Time{}
			b.lastEngagedGUID = 0
			b.engagedTargetHealth = 0
			b.grindTargetGUID = 0
			return behaviortree.Failure
		}
	}
	if b.lastEngagedGUID == guid {
		// re-tick debug moved to chat decisions only
	}

	// Use interpolated position for moving targets
	tx, ty, tz := target.InterpolatedPosition()
	dx := tx - b.world.posX
	dy := ty - b.world.posY
	dist2d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	dist := target.DistanceTo(b.world.posX, b.world.posY, b.world.posZ) // 3D for logs etc.

	// Use Charge if in range (8-25 yards) and not in combat
	if dist >= 8 && dist <= 25 && !b.world.InCombat() && b.world.IsSpellReady(100) {
		b.log("Charging target Entry=%d GUID=%d dist=%.1f", target.Entry, guid, dist)
		b.logDecision("Charging target (Entry=%d)", target.Entry)
		b.world.SetTarget(guid)
		if b.lastEngagedGUID != guid {
			b.currentTargetSetAt = time.Now()
			b.lastEngagedGUID = guid
			b.engagedTargetHealth = target.Health()
		}
		b.world.CastSpell(100, guid) // Charge
		return behaviortree.Running
	}

	// Set target early so we are attacking while closing the gap
	b.world.SetTarget(guid)
	if b.lastEngagedGUID != guid {
		b.currentTargetSetAt = time.Now()
		b.lastEngagedGUID = guid
		if target != nil {
			b.engagedTargetHealth = target.Health()
		}
	}

	// If too far in horizontal, move closer (follow to within 2 yards XY). Use 2D so height diffs don't stop pursuit.
	if dist2d > 2.0 {
		b.logDecision("Moving toward target (dist2d=%.1fyd, 3d=%.1f)", dist2d, dist)
		// Face target while approaching
		facing := float32(math.Atan2(float64(dy), float64(dx)))
		if math.Abs(float64(b.world.orientation-facing)) > 0.3 {
			b.world.SetFacing(facing)
		}
		b.moveToPoint(tx, ty, tz)
		return behaviortree.Running
	}

	// Stop moving, start attacking
	if b.isMoving {
		dx = tx - b.world.posX
		dy = ty - b.world.posY
		facing := float32(math.Atan2(float64(dy), float64(dx)))
		b.world.orientation = facing
		b.world.MoveStop()
		b.world.SendHeartbeat()
		b.isMoving = false
		if b.moveController != nil {
			b.moveController.Stop(time.Now())
		}
	}

	// Face the target
	dx = tx - b.world.posX
	dy = ty - b.world.posY
	facing := float32(math.Atan2(float64(dy), float64(dx)))
	if math.Abs(float64(b.world.orientation-facing)) > 0.1 {
		b.world.SetFacing(facing)
	}

	// Only claim "melee engage" when actually close. Otherwise it's pursuit.
	curDist := target.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)
	playerX, playerY, playerZ := b.world.posX, b.world.posY, b.world.posZ
	mobX, mobY, mobZ := tx, ty, tz
	if curDist <= 3.0 {
		b.sendAliveReasonChat("ENGAGE melee GUID=%d Entry=%d: h=%d/%d IsAlive=%v flags=0x%x dist=%.1f player=(%.1f,%.1f,%.1f) mob=(%.1f,%.1f,%.1f)",
			guid, target.Entry, target.Health(), target.MaxHealth(), target.IsAlive(), target.Values[UnitFieldFlags], curDist, playerX, playerY, playerZ, mobX, mobY, mobZ)
		b.log("ATTACK ENGAGE melee GUID=%d Entry=%d dist=%.1f player=(%.1f,%.1f,%.1f) mob=(%.1f,%.1f,%.1f)", guid, target.Entry, curDist, playerX, playerY, playerZ, mobX, mobY, mobZ)
	}
	// Debug to catch attacking non-existing or far mob (console for visibility)
	if obj := b.world.GetObject(guid); obj == nil {
		b.log("ATTACK on missing object GUID=%d dist=%.1f player=(%.1f,%.1f,%.1f) mob=(%.1f,%.1f,%.1f)", guid, curDist, playerX, playerY, playerZ, mobX, mobY, mobZ)
	} else if !obj.IsAlive() {
		b.log("ATTACK on dead object GUID=%d dist=%.1f player=(%.1f,%.1f,%.1f) mob=(%.1f,%.1f,%.1f)", guid, curDist, playerX, playerY, playerZ, mobX, mobY, mobZ)
	}
	b.world.AttackSwing(guid)

	return behaviortree.Running
}

func (b *Bot) actionCombatRotation() behaviortree.Status {
	targetGUID := b.world.TargetGUID()
	// console enter log removed - use chat for decisions
	b.logDecision("In combat rotation on target")

	// Opportunistic fresh target selection while in combat rotation.
	// Prevents continuing to "attack" a creature that died while we weren't looking.
	if targetGUID != 0 {
		cur := b.world.GetObject(targetGUID)
		if cur == nil || !cur.IsAlive() || b.isKnownDead(targetGUID) {
			b.world.MarkObjectDead(targetGUID)
			b.markKnownDead(targetGUID)
			targetGUID = 0
		} else if cur.value(UnitNPCFlags) != 0 || !b.isHostileFaction(cur.value(UnitFieldFaction)) {
			b.sendAliveReasonChat("ABORT combat friendly NPC GUID=%d Entry=%d npc=0x%x fac=%d", targetGUID, cur.Entry, cur.value(UnitNPCFlags), cur.value(UnitFieldFaction))
			b.world.ClearTarget()
			b.world.ClearCombat()
			targetGUID = 0
		}
	}
	if targetGUID == 0 {
		if best := b.findBestTarget(35); best != nil {
			targetGUID = best.GUID
			b.world.SetTarget(targetGUID)
			b.currentTargetSetAt = time.Now()
			b.lastEngagedGUID = targetGUID
			deadF := (best.Values[UnitFieldFlags] & UnitFlagDead) != 0
			b.sendAliveReasonChat("COMBAT picked GUID=%d Entry=%d: h=%d/%d IsAlive=%v deadFlag=%v f=0x%x npc=0x%x",
				targetGUID, best.Entry, best.Health(), best.MaxHealth(), best.IsAlive(), deadF, best.Values[UnitFieldFlags], best.Values[UnitNPCFlags])
			b.log("ATTACK COMBAT picked GUID=%d Entry=%d", targetGUID, best.Entry)
		}
	}
	if targetGUID == 0 {
		// No target but in combat - find what's attacking us
		newTarget := b.findBestTarget(30)
		if newTarget != nil {
			b.world.SetTarget(newTarget.GUID)
			if b.lastEngagedGUID != newTarget.GUID {
				b.currentTargetSetAt = time.Now()
				b.lastEngagedGUID = newTarget.GUID
				b.engagedTargetHealth = newTarget.Health()
			}
			px, py, pz := b.world.posX, b.world.posY, b.world.posZ
			mx, my, mz := newTarget.PosX, newTarget.PosY, newTarget.PosZ
			b.log("ATTACK (new combat target) GUID=%d Entry=%d player=(%.1f,%.1f,%.1f) mob=(%.1f,%.1f,%.1f)", newTarget.GUID, newTarget.Entry, px, py, pz, mx, my, mz)
			b.world.AttackSwing(newTarget.GUID)
			return behaviortree.Running
		}
		b.world.ClearTarget()
		b.world.ClearCombat()
		return behaviortree.Failure
	}

	target := b.world.GetObject(targetGUID)
	if target == nil || !target.IsAlive() || b.isKnownDead(targetGUID) {
		d := float32(999)
		if target != nil {
			d = target.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)
		}
		b.logDecision("Target dead, switching to next action")
		b.world.AttackStop()
		b.world.ClearTarget()
		b.world.ClearCombat()
		b.stopCurrentMove()
		b.markKnownDead(targetGUID)
		// Only set lastLoot for opportunistic close loot. Do not run to corpses.
		if target != nil && d <= 12.0 {
			b.lastLootGUID = targetGUID
		} else {
			b.lastLootGUID = 0
		}

		// Check if something else is attacking us
		newTarget := b.findBestTarget(30)
		if newTarget != nil {
			b.world.SetTarget(newTarget.GUID)
			if b.lastEngagedGUID != newTarget.GUID {
				b.currentTargetSetAt = time.Now()
				b.lastEngagedGUID = newTarget.GUID
				b.engagedTargetHealth = newTarget.Health()
			}
			px, py, pz := b.world.posX, b.world.posY, b.world.posZ
			mx, my, mz := newTarget.PosX, newTarget.PosY, newTarget.PosZ
			b.log("ATTACK (new combat target) GUID=%d Entry=%d player=(%.1f,%.1f,%.1f) mob=(%.1f,%.1f,%.1f)", newTarget.GUID, newTarget.Entry, px, py, pz, mx, my, mz)
			b.world.AttackSwing(newTarget.GUID)
			return behaviortree.Running
		}
		// No more valid target: return Failure so grind/wander can run.
		return behaviortree.Failure
	}

	// If we haven't received *any* update (values/movement) for the current combat target recently,
	// the server may no longer consider it visible to us, or its position is stale because the mob
	// is wandering and we stopped receiving MonsterMove (e.g. due to our reported player pos making
	// the mob out of our update range on server, even if in reality it's near).
	// Drop it so the tree can re-evaluate and pick a target with fresh position data.
	if !target.LastSeen.IsZero() && time.Since(target.LastSeen) > 5*time.Second {
		b.logDecision("Combat target stale (no update >5s) GUID=%d Entry=%d, dropping to re-acquire", targetGUID, target.Entry)
		b.world.ClearTarget()
		b.world.ClearCombat()
		b.stopCurrentMove()
		return behaviortree.Failure
	}

	if target != nil && (target.Values[UnitNPCFlags] != 0 || !b.isHostileFaction(target.Values[UnitFieldFaction])) {
		b.sendAliveReasonChat("ABORT combat friendly GUID=%d Entry=%d npc=0x%x fac=%d", targetGUID, target.Entry, target.Values[UnitNPCFlags], target.Values[UnitFieldFaction])
		b.world.ClearTarget()
		b.world.ClearCombat()
		b.stopCurrentMove()
		return behaviortree.Failure
	}

	// Log to chat why we believe this target is alive (debug for dead creature attacks)
	deadF := (target.value(UnitFieldFlags) & UnitFlagDead) != 0
	curDist := target.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)
	// Use interpolated early for debug
	tx, ty, tz := target.InterpolatedPosition()
	if curDist <= 5.0 {
		px, py, pz := b.world.posX, b.world.posY, b.world.posZ
		mx, my, mz := tx, ty, tz
		b.sendAliveReasonChat("COMBAT GUID=%d Entry=%d: h=%d/%d IsAlive=%v deadFlag=%v flags=0x%x npc=0x%x fac=%d dist=%.1f player=(%.1f,%.1f,%.1f) mob=(%.1f,%.1f,%.1f)",
			targetGUID, target.Entry, target.Health(), target.MaxHealth(), target.IsAlive(), deadF,
			target.value(UnitFieldFlags), target.value(UnitNPCFlags), target.value(UnitFieldFaction), curDist, px, py, pz, mx, my, mz)
		b.log("ATTACK COMBAT GUID=%d Entry=%d dist=%.1f player=(%.1f,%.1f,%.1f) mob=(%.1f,%.1f,%.1f)", targetGUID, target.Entry, curDist, px, py, pz, mx, my, mz)
	} else {
		b.logDecision("COMBAT far from target GUID=%d dist=%.1f (still chasing)", targetGUID, curDist)
	}

	// Quick drop for low health no-progress (stale health on dead mob)
	if target.Health() > 0 && target.Health() < 20 {
		if !b.currentTargetSetAt.IsZero() && time.Since(b.currentTargetSetAt) > 3*time.Second {
			if b.engagedTargetHealth == 0 || target.Health() >= b.engagedTargetHealth {
				b.markKnownDead(targetGUID)
				b.logDecision("COMBAT quick unstick low health stale target")
				b.world.ClearTarget()
				b.world.ClearCombat()
				b.stopCurrentMove()
				return behaviortree.Failure
			}
		}
	}

	// Use interpolated position for moving targets (computed earlier for debug)
	dx := tx - b.world.posX
	dy := ty - b.world.posY
	dist2d := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	// Face and approach target if needed (follow to within 2 yards XY, re-follow if target moves).
	// Use 2D for the decision so Z differences (hills, interp) don't cause "stuck far but combat".
	if dist2d > 2.0 {
		// Face the target while closing the gap
		facing := float32(math.Atan2(float64(dy), float64(dx)))
		if math.Abs(float64(b.world.orientation-facing)) > 0.3 {
			b.world.SetFacing(facing)
		}
		b.moveToPoint(tx, ty, tz)
		return behaviortree.Running
	}

	// Stop if we were moving
	if b.isMoving {
		b.world.MoveStop()
		b.isMoving = false
		if b.moveController != nil {
			b.moveController.Stop(time.Now())
		}
		// Stationary heartbeat after stopping movement
		b.world.SendHeartbeat()
	}

	// Face target
	facing := float32(math.Atan2(float64(dy), float64(dx)))
	if math.Abs(float64(b.world.orientation-facing)) > 0.1 {
		b.world.SetFacing(facing)
	}

	// Continue auto-attack
	playerX, playerY, playerZ := b.world.posX, b.world.posY, b.world.posZ
	mobX, mobY, mobZ := tx, ty, tz
	if obj := b.world.GetObject(targetGUID); obj == nil || !obj.IsAlive() {
		b.log("ATTACK on missing/dead in combat GUID=%d player=(%.1f,%.1f,%.1f) mob=(%.1f,%.1f,%.1f)", targetGUID, playerX, playerY, playerZ, mobX, mobY, mobZ)
	}
	b.world.AttackSwing(targetGUID)

	// Use abilities based on class
	b.useCombatAbilities(targetGUID, target)

	return behaviortree.Running
}

func (b *Bot) useCombatAbilities(targetGUID uint64, target *WorldObject) {
	// Respect GCD (1.5 seconds)
	if time.Since(b.lastCastTime) < 1500*time.Millisecond {
		return
	}

	level := b.world.PlayerLevel()

	// Use Victory Rush if available (proc from killing blow)
	if b.lastVictoryRush && b.world.IsSpellReady(34428) {
		b.log("Casting Victory Rush (proc)")
		b.world.CastSpell(34428, targetGUID)
		b.lastCastTime = time.Now()
		b.lastVictoryRush = false
		return
	}

	spells := gamedata.GetSpellPriority(b.config.Class, level)

	for _, spellID := range spells {
		info, ok := gamedata.WarriorSpells[spellID]
		if !ok {
			continue
		}

		if info.Level > level {
			continue
		}

		// Skip Execute if target isn't low health (< 20%)
		if spellID == 5308 && target.Health() > target.MaxHealth()/5 {
			continue
		}

		// Check range
		dist := target.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)
		if info.Range > 0 && dist > info.Range {
			continue
		}
		if info.MinRange > 0 && dist < info.MinRange {
			continue
		}

		if b.world.IsSpellReady(spellID) {
			b.log("Casting %s (ID=%d) on target", info.Name, spellID)
			b.world.CastSpell(spellID, targetGUID)
			b.lastCastTime = time.Now()
			return // One ability per GCD
		}
	}
}

func (b *Bot) actionLoot() behaviortree.Status {
	if b.lastLootGUID == 0 {
		return behaviortree.Failure
	}
	guid := b.lastLootGUID
	obj := b.world.GetObject(guid)
	d := float32(999)
	if obj != nil {
		d = obj.DistanceTo(b.world.posX, b.world.posY, b.world.posZ)
	}
	// loot enter debug suppressed from console

	// Only loot if the corpse is very close (within melee range ~8).
	// If far we already decided not to set it, but double-check and clear here.
	if obj != nil {
		if d > 10.0 {
			// too far loot decision will be in chat via logDecision if needed
			b.lastLootGUID = 0
			b.stopCurrentMove()
			return behaviortree.Failure
		}
	} else {
		// gone loot to chat
		b.lastLootGUID = 0
		return behaviortree.Failure
	}

	b.lastLootGUID = 0
	b.logDecision("Looting corpse")
	b.world.Loot(guid)
	// The SMSG_LOOT_RESPONSE will trigger handleLootOpened which performs the actual looting + release.
	return behaviortree.Success
}

func (b *Bot) handleLootOpened(lootGUID uint64, items []LootItem) {
	// Auto-loot all items without blocking sleeps (non-blocking for the read loop)
	for _, item := range items {
		b.world.LootItem(item.Index)
	}
	b.world.LootMoney()
	b.world.LootRelease(lootGUID)
}

func (b *Bot) actionWander() behaviortree.Status {
	// console enter log removed (too noisy with 1000 bots)
	b.logDecision("Wandering to explore for mobs")

	// Re-check for targets while wandering. This lets us exit wander quickly when
	// mobs appear (the Sequence may have been resumed on the wander action).
	if best := b.findBestTarget(35); best != nil && best.IsAlive() {
		// (debug log removed)
		return behaviortree.Failure // let selector try find_and_fight again next tick
	}

	// If the controller is actively moving toward a wander point, keep going until it finishes.
	// The MovementController handles its own arrival detection and will stop when it reaches the destination.
	if b.isMoving {
		return behaviortree.Running
	}

	// Find a random nearby point using pathfinding.
	// Use a reasonably large radius so we actually explore the world instead of
	// orbiting a tiny local area. Random points + pathing + chaining gives
	// varied traversal instead of tight circles or repeated loops.
	if b.nav != nil {
		x, y, z, _, mapID := b.world.Position()

		// Snap to real ground before deciding on a random wander point.
		// Use small relative probe based on current Z (from path or pos) to stay on correct floor/level.
		// High fixed probe can pick upper floors in multi-level areas (e.g. under second floor).
		probeZ := z + 5.0
		if gh, ok := b.nav.GetHeight(mapID, x, y, probeZ); ok {
			delta := gh - z
			if delta > 1.0 {
				// would snap up significantly - keep current to avoid floor teleport
			} else if math.Abs(float64(delta)) > 0.5 {
				z = gh
			} else {
				z = gh
			}
		}

		radius := float32(70)
		if mapID == 1 || mapID == 530 { // Kalimdor or Blood Elf start (Eversong 530) - worse navmesh, tighter to avoid under/over map
			radius = 40
		}
		result, err := b.nav.FindRandomPath(mapID, navigation.Point3D{X: x, Y: y, Z: z}, radius)
		if err == nil && result.Found && len(result.Points) > 1 {
			pts := simplifyAndDensifyPath(result.Points, 3.0, 1.0)
			// Use the points directly from the random generator (they already include height correction
			// via internal FindPath/GetPolyHeight). This ensures random movement uses the generator's
			// corrected heights, and points are only on valid mmaps-connected areas (prevents climbing
			// rocks/areas the navmesh does not allow).
			b.ensureMovementController()
			if b.moveController != nil {
				// Snap first point to current for handoff (current already snapped above)
				if len(pts) > 0 {
					pts[0] = navigation.Point3D{X: x, Y: y, Z: z}
				}
				b.moveController.SetPath(pts, time.Now(), b.world.orientation, mapID)
				b.isMoving = true
			}
			return behaviortree.Running
		}
	}

	// Fallback (no nav): pick a varied random direction + decent distance (15-45yd).
	// Using high-res time bits for angle + varying length breaks repetitive circular
	// paths. Each re-wander (when close to previous dest) goes somewhere new.
	x, y, z := b.world.posX, b.world.posY, b.world.posZ
	seed := time.Now().UnixNano()
	angle := float64(seed%62831853) / 10000000.0 // good distribution
	dist := 15.0 + float64(seed%31)              // 15-45 yd steps
	newX := x + float32(math.Cos(angle))*float32(dist)
	newY := y + float32(math.Sin(angle))*float32(dist)
	b.moveToPoint(newX, newY, z)
	return behaviortree.Running
}

// ============================================================
// Movement system
// ============================================================

func (b *Bot) moveToPoint(x, y, z float32) {
	b.ensureMovementController()
	if b.moveController == nil {
		// Fallback: at least stop any old movement
		b.world.MoveStop()
		return
	}

	px, py, pz, po, mapID := b.world.Position()

	// Before any movement decisions, snap both current position and target to real
	// ground height using a small Z offset. Querying GetHeight from slightly above
	// the expected ground is required to get accurate terrain height.
	if b.nav != nil {
		// Snap current position to ground if needed (small corrections only; avoid jumping levels).
		probeZ := pz + 5.0
		if gh, ok := b.nav.GetHeight(mapID, px, py, probeZ); ok {
			delta := gh - pz
			if delta > 1.0 {
				// would snap up significantly (possible upper floor) - keep current Z
			} else if math.Abs(float64(delta)) > 0.5 {
				pz = gh
			} else {
				pz = gh
			}
		}
		targetProbe := z + 5.0
		if gh, ok := b.nav.GetHeight(mapID, x, y, targetProbe); ok {
			delta := gh - z
			if math.Abs(float64(delta)) > 2.0 {
				// large difference - trust the provided target Z (e.g. from random generator or mob)
				gh = z
			} else if math.Abs(float64(delta)) > 0.5 {
				z = gh
			} else {
				z = gh
			}
		}
	}

	// Clamp target Z for pursuit to avoid oscillation: bots trying to go much higher/lower
	// than current ground then snapping back. Limit delta to ~5 yards.
	deltaZ := z - pz
	if deltaZ > 5.0 {
		z = pz + 5.0
	} else if deltaZ < -5.0 {
		z = pz - 5.0
	}

	current := navigation.Point3D{X: px, Y: py, Z: pz}

	// If we have nav, compute a proper path (real pathfinding as required for movement).
	var pts []navigation.Point3D
	usedNavPath := false
	if b.nav != nil {
		result, err := b.nav.FindPath(mapID, current, navigation.Point3D{X: x, Y: y, Z: z})
		if err == nil && result != nil && result.Found && len(result.Points) > 1 {
			pts = simplifyAndDensifyPath(result.Points, 3.0, 1.0)
			// detect crazy path (common in Durotar bad navmesh)
			straight := current.DistanceTo2D(navigation.Point3D{X: x, Y: y, Z: z})
			plen := float32(0)
			for j := 1; j < len(pts); j++ {
				plen += pts[j-1].DistanceTo2D(pts[j])
			}
			if plen > straight*2.5 || len(pts) > 80 {
				b.logDecision("crazy path in Durotar-like terrain, using direct")
				pts = simplifyAndDensifyPath([]navigation.Point3D{current, {X: x, Y: y, Z: z}}, 3.0, 1.0)
			}
			usedNavPath = true
		} else {
			b.logDecision("No path found to target")
		}
	}

	if len(pts) == 0 {
		if b.nav != nil {
			b.logDecision("Nav present but no valid path to target - falling back to direct line (may clip trees/obstacles!)")
		}
		// Direct fallback
		pts = simplifyAndDensifyPath([]navigation.Point3D{current, {X: x, Y: y, Z: z}}, 3.0, 1.0)
		usedNavPath = false
	}

	// Ensure first point is exactly current for smooth handoff
	if len(pts) > 0 {
		pts[0] = current
	}

	// Only snap Z for direct fallback paths (nav paths already have corrected Z from generator).
	if !usedNavPath && b.nav != nil && mapID != 0 {
		for i := range pts {
			origZ := pts[i].Z
			hint := origZ + 5.0
			if gh, ok := b.nav.GetHeight(mapID, pts[i].X, pts[i].Y, hint); ok {
				if math.Abs(float64(gh-origZ)) > 2.0 {
					gh = origZ // avoid large correction in direct fallback
				}
				pts[i].Z = gh
			}
		}
	}

	// Hand off to the (now only) movement implementation.
	// The controller handles time-based following, 500ms HBs, and explicit turns at direction changes.
	b.moveController.SetPath(pts, time.Now(), po, mapID)
	b.isMoving = true

	// The controller will emit the appropriate START / HB packets.
}

// simplifyAndDensifyPath reduces zig-zags (simplify collinear) and adds intermediate points
// for smoother following on uneven terrain like Durotar hills. This helps make path following
// look less "crazy" when navmesh produces sparse or jagged paths.
func simplifyAndDensifyPath(pts []navigation.Point3D, maxStep, collinearTol float32) []navigation.Point3D {
	if len(pts) < 2 {
		return pts
	}
	// densify first (use 2D horizontal distance for step size; Z will be ground-snapped live)
	dense := []navigation.Point3D{pts[0]}
	for i := 1; i < len(pts); i++ {
		p0 := pts[i-1]
		p1 := pts[i]
		d := p0.DistanceTo2D(p1)
		if d > maxStep && d > 0.001 {
			n := int(d / maxStep)
			for k := 1; k <= n; k++ {
				t := float32(k) / float32(n+1)
				dense = append(dense, navigation.Point3D{
					X: p0.X + (p1.X-p0.X)*t,
					Y: p0.Y + (p1.Y-p0.Y)*t,
					Z: p0.Z + (p1.Z-p0.Z)*t, // interp; live snap will correct to ground
				})
			}
		}
		dense = append(dense, p1)
	}
	if len(dense) <= 2 {
		return dense
	}
	// simplify collinear-ish points (keep changes in direction or steep Z)
	simp := []navigation.Point3D{dense[0]}
	for i := 1; i < len(dense)-1; i++ {
		a := simp[len(simp)-1]
		b := dense[i]
		c := dense[i+1]
		abx := b.X - a.X
		aby := b.Y - a.Y
		abz := b.Z - a.Z
		bcx := c.X - b.X
		bcy := c.Y - b.Y
		bcz := c.Z - b.Z
		// 3D cross product mag
		crx := aby*bcz - abz*bcy
		cry := abz*bcx - abx*bcz
		crz := abx*bcy - aby*bcx
		cross := float32(math.Sqrt(float64(crx*crx + cry*cry + crz*crz)))
		if cross > collinearTol || math.Abs(float64(abz)) > 1.5 || math.Abs(float64(bcz)) > 1.5 {
			simp = append(simp, b)
		}
	}
	simp = append(simp, dense[len(dense)-1])
	return simp
}

// stopCurrentMove aborts any in-progress path without necessarily sending a stop packet.
// Called on kill of target etc. to prevent continuing to run toward a now-dead mob's last position.
func (b *Bot) stopCurrentMove() {
	b.isMoving = false
	if b.moveController != nil {
		b.moveController.Stop(time.Now())
	}
}

func (b *Bot) updateMovement() {
	if b.moveController == nil {
		return
	}

	b.moveController.Update(time.Now())
	b.isMoving = b.moveController.IsMoving()

	// Sync position from controller ONLY if it has actually started a path (travelDist > 0 or isMoving).
	// Otherwise we would overwrite the correct login position with struct-zero (0,0,0).
	if b.moveController.TravelDist() > 0 || b.isMoving {
		cx, cy, cz, co := b.moveController.CurrentPosition()
		b.world.UpdatePosition(cx, cy, cz, co)
	}
}

// ============================================================
// LuaEngine BotAPI implementation
// ============================================================

func (b *Bot) GetPosition() (x, y, z, o float32) {
	x, y, z, o, _ = b.world.Position()
	return
}

func (b *Bot) MoveTo(x, y, z float32) error {
	b.moveToPoint(x, y, z)
	return nil
}

func (b *Bot) StopMoving() error {
	b.stopCurrentMove()
	if err := b.world.MoveStop(); err != nil {
		return err
	}
	// Send a stationary heartbeat after stop so server has final position
	return b.world.SendHeartbeat()
}

// generateUniqueCharName produces a highly unique WoW-legal (alphabetic) name.
// Used when the requested name is taken so we can keep trying fresh ones.
func generateUniqueCharName(seed int) string {
	consonantStarts := []string{
		"Ar", "Br", "Cr", "Dr", "El", "Fr", "Gr", "Hr", "Ir", "Kr", "Lr", "Mr", "Nr", "Or", "Pr", "Rr", "Sr", "Tr", "Ur", "Vr", "Wr", "Zr",
		"Al", "Bl", "Cl", "Fl", "Gl", "Kl", "Ll", "Ml", "Pl", "Sl", "Tl", "Vl", "Wl", "Yl",
		"An", "Bn", "Cn", "Dn", "Fn", "Gn", "Kn", "Ln", "Mn", "Nn", "Pn", "Sn", "Tn", "Vn", "Wn", "Yn",
		"Ak", "Bk", "Ck", "Dk", "Fk", "Gk", "Hk", "Kk", "Lk", "Mk", "Nk", "Pk", "Sk", "Tk", "Vk", "Wk", "Yk",
		"Ag", "Bg", "Cg", "Dg", "Fg", "Gg", "Hg", "Kg", "Lg", "Mg", "Ng", "Pg", "Sg", "Tg", "Vg", "Wg", "Yg",
		"Ad", "Bd", "Cd", "Dd", "Fd", "Gd", "Hd", "Kd", "Ld", "Md", "Nd", "Pd", "Sd", "Td", "Vd", "Wd", "Yd",
		"Th", "Sh", "Ch", "Ph", "Wh", "Qu", "St", "Sp", "Sk", "Sm", "Sn", "Sw", "Tw", "Tr", "Dr", "Gr", "Kr", "Pr", "Br", "Fr", "Cl", "Fl", "Gl", "Pl", "Sl", "Bl",
	}
	midSyls := []string{
		"ar", "er", "ir", "or", "ur", "yr", "al", "el", "il", "ol", "ul", "yl",
		"an", "en", "in", "on", "un", "yn", "ak", "ek", "ik", "ok", "uk", "yk",
		"ag", "eg", "ig", "og", "ug", "yg", "ad", "ed", "id", "od", "ud", "yd",
		"ath", "eth", "ith", "oth", "uth", "yth", "ash", "esh", "ish", "osh", "ush", "ysh",
		"ra", "re", "ri", "ro", "ru", "ry", "la", "le", "li", "lo", "lu", "ly",
		"ma", "me", "mi", "mo", "mu", "my", "na", "ne", "ni", "no", "nu", "ny",
		"sa", "se", "si", "so", "su", "sy", "ta", "te", "ti", "to", "tu", "ty",
		"va", "ve", "vi", "vo", "vu", "vy", "za", "ze", "zi", "zo", "zu", "zy",
	}
	endSyls := []string{
		"ar", "er", "ir", "or", "ur", "ath", "eth", "ith", "oth", "uth",
		"an", "en", "in", "on", "un", "ak", "ek", "ik", "ok", "uk",
		"al", "el", "il", "ol", "ul", "ad", "ed", "id", "od", "ud",
		"as", "es", "is", "os", "us", "and", "end", "ind", "ond", "und",
		"ard", "erd", "ird", "ord", "urd", "ion", "eon", "ian", "aan", "oon",
	}

	rb := make([]byte, 2)
	rand.Read(rb)
	r1 := int(rb[0]) + seed
	r2 := int(rb[1]) + seed*7

	part1 := consonantStarts[r1%len(consonantStarts)]
	part2 := midSyls[r2%len(midSyls)]
	part3 := midSyls[(r1+r2)%len(midSyls)]
	part4 := endSyls[(r1*37+r2)%len(endSyls)]

	name := part1 + part2 + part3 + part4
	if len(name) > 12 {
		name = name[:12]
	}
	if len(name) < 3 {
		name += "ar"
	}
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
	}
	return name
}

func (b *Bot) AttackTarget(guid uint64) error {
	b.world.SetTarget(guid)
	return b.world.AttackSwing(guid)
}

func (b *Bot) StopAttack() error {
	return b.world.AttackStop()
}

func (b *Bot) SetTarget(guid uint64) error {
	return b.world.SetTarget(guid)
}

func (b *Bot) CastSpell(spellID uint32, targetGUID uint64) error {
	return b.world.CastSpell(spellID, targetGUID)
}

func (b *Bot) IsSpellReady(spellID uint32) bool {
	return b.world.IsSpellReady(spellID)
}

func (b *Bot) GetHealth() (current, max uint32) {
	return b.world.Health(), b.world.MaxHealth()
}

func (b *Bot) GetPower() (current, max uint32) {
	b.world.statsMu.RLock()
	defer b.world.statsMu.RUnlock()
	return b.world.power, b.world.maxPower
}

func (b *Bot) GetLevel() uint32 {
	return b.world.PlayerLevel()
}

func (b *Bot) InCombat() bool {
	return b.world.InCombat()
}

func (b *Bot) IsAlive() bool {
	h := b.world.Health()
	mh := b.world.MaxHealth()
	// If we haven't received health data yet, assume alive
	if h == 0 && mh == 0 {
		return true
	}
	if h == 0 {
		return false
	}
	// Also check death count - if deaths > 0 and health is still 0, we're dead
	return h > 0
}

func (b *Bot) GetTargetGUID() uint64 {
	return b.world.TargetGUID()
}

func (b *Bot) GetNearbyUnits(maxDist float32) []luaengine.UnitInfo {
	objects := b.world.GetNearbyUnits(maxDist)
	result := make([]luaengine.UnitInfo, 0, len(objects))
	for _, obj := range objects {
		result = append(result, b.worldObjToUnitInfo(obj))
	}
	return result
}

func (b *Bot) GetNearbyPlayers(maxDist float32) []luaengine.UnitInfo {
	objects := b.world.GetNearbyPlayers(maxDist)
	result := make([]luaengine.UnitInfo, 0, len(objects))
	for _, obj := range objects {
		result = append(result, b.worldObjToUnitInfo(obj))
	}
	return result
}

func (b *Bot) GetUnitInfo(guid uint64) *luaengine.UnitInfo {
	obj := b.world.GetObject(guid)
	if obj == nil {
		return nil
	}
	info := b.worldObjToUnitInfo(obj)
	return &info
}

func (b *Bot) worldObjToUnitInfo(obj *WorldObject) luaengine.UnitInfo {
	return luaengine.UnitInfo{
		GUID:      obj.GUID,
		Entry:     obj.Entry,
		Health:    obj.Health(),
		MaxHealth: obj.MaxHealth(),
		Level:     obj.Level(),
		PosX:      obj.PosX,
		PosY:      obj.PosY,
		PosZ:      obj.PosZ,
		IsAlive:   obj.IsAlive(),
		IsPlayer:  obj.IsPlayer,
		Distance:  obj.DistanceTo(b.world.posX, b.world.posY, b.world.posZ),
		Name:      obj.Name,
	}
}

func (b *Bot) SendChat(message string) error {
	return b.world.SendChatMessage(ChatMsgSay, LangCommon, message)
}

func (b *Bot) SendCommand(command string) error {
	return b.world.SendGMCommand(command)
}

func (b *Bot) Loot(guid uint64) error {
	return b.world.Loot(guid)
}

func (b *Bot) LootAll(guid uint64) error {
	b.world.Loot(guid)
	time.Sleep(500 * time.Millisecond)
	b.world.LootMoney()
	time.Sleep(200 * time.Millisecond)
	b.world.LootRelease(guid)
	return nil
}

func (b *Bot) Log(format string, args ...interface{}) {
	b.log(format, args...)
}

// markKnownDead marks this GUID as dead in *this bot's* private view of the world.
// This persists even if server packets keep sending positive health (stale cache from death not fully propagated to this connection).
func (b *Bot) markKnownDead(guid uint64) {
	b.knownDeadMu.Lock()
	if b.knownDead == nil {
		b.knownDead = make(map[uint64]bool)
	}
	b.knownDead[guid] = true
	b.knownDeadMu.Unlock()
	b.world.MarkObjectDead(guid) // also force live cache for this bot's worldclient
}

// isKnownDead returns true if this bot has decided the guid is dead in its view.
func (b *Bot) isKnownDead(guid uint64) bool {
	b.knownDeadMu.Lock()
	defer b.knownDeadMu.Unlock()
	return b.knownDead != nil && b.knownDead[guid]
}

// clearKnownDead if we see evidence it's alive (positive health update).
func (b *Bot) clearKnownDead(guid uint64) {
	b.knownDeadMu.Lock()
	if b.knownDead != nil {
		delete(b.knownDead, guid)
	}
	b.knownDeadMu.Unlock()
}

// ============================================================
// Utility methods
// ============================================================

// Stop signals the bot to stop running.
func (b *Bot) Stop() {
	select {
	case <-b.stopCh:
	default:
		close(b.stopCh)
	}
}

// LoadLuaScript loads a Lua script at runtime.
func (b *Bot) LoadLuaScript(code string) error {
	if b.lua == nil {
		return fmt.Errorf("lua engine not initialized")
	}
	return b.lua.DoString(code)
}

// Events returns the recorded events.
func (b *Bot) Events() []BotEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]BotEvent, len(b.events))
	copy(result, b.events)
	return result
}

func (b *Bot) setStatus(s BotStatus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status = s
}

// Status returns current status
func (b *Bot) Status() BotResult {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := BotResult{
		ID:     b.id,
		Status: b.status,
		Kills:  b.kills,
		Deaths: b.deaths,
	}
	if b.err != nil {
		result.Error = b.err.Error()
	}
	if b.world != nil {
		result.Level = b.world.PlayerLevel()
	}
	return result
}

func (b *Bot) fail(format string, args ...interface{}) BotResult {
	msg := fmt.Sprintf(format, args...)
	b.log("ERROR: %s", msg)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status = BotStatusError
	b.err = fmt.Errorf("%s", msg)
	if b.world != nil {
		b.world.Close()
	}
	return BotResult{
		ID:     b.id,
		Status: BotStatusError,
		Error:  msg,
	}
}

func (b *Bot) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[Bot %s] %s\n", b.id, msg)
}

// logDecision logs an important AI/behavior decision both to the console and
// (throttled) as an in-game /say so you can observe what the bot is "thinking"
// while watching it in the world.
func (b *Bot) logDecision(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)

	// Console is unreadable with hundreds/thousands of bots.
	// All decision logging goes to in-game /say chat only.
	// (use b.log(...) explicitly for anything you want in node console)
	if b.config.LogDecisionsToChat {
		if time.Since(b.lastDecisionChat) < 700*time.Millisecond {
			return
		}
		b.lastDecisionChat = time.Now()

		chat := "[AI] " + msg
		if len(chat) > 110 {
			chat = chat[:107] + "..."
		}
		_ = b.world.SendChatMessage(ChatMsgSay, LangCommon, chat)
	}
}

// sendAliveReasonChat sends a detailed "why we think this mob is alive" message to in-game chat.
// Uses its own throttle so you can see the health/flags/npc/faction info when engaging or fighting.
// Console output suppressed for high bot counts - only chat.
func (b *Bot) sendAliveReasonChat(format string, args ...interface{}) {
	if time.Since(b.lastAliveReasonChat) < 800*time.Millisecond {
		return
	}
	b.lastAliveReasonChat = time.Now()
	msg := fmt.Sprintf(format, args...)
	if len(msg) > 120 {
		msg = msg[:117] + "..."
	}
	_ = b.world.SendChatMessage(ChatMsgSay, LangCommon, "[ALIVE] "+msg)
	// no b.log - console is useless at scale
}

func (b *Bot) addEvent(eventType, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	b.log("[EVENT:%s] %s", eventType, msg)
	b.mu.Lock()
	b.events = append(b.events, BotEvent{
		Time:    time.Now(),
		Type:    eventType,
		Message: msg,
	})
	b.mu.Unlock()
}
