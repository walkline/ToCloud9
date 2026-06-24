package wowsimclient

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// BotConfig holds configuration for a single bot instance
type BotConfig struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	AuthServer    string `json:"auth_server"`
	CharacterName string `json:"character_name"`
	RealmIndex    int    `json:"realm_index"` // 0-based index into realm list

	// Character creation defaults (used if character doesn't exist)
	Race      uint8 `json:"race"`       // default: 5 (undead)
	Class     uint8 `json:"class"`      // default: 1 (warrior)
	Gender    uint8 `json:"gender"`     // default: 0 (male)
}

// BotStatus represents the current state of a bot
type BotStatus string

const (
	BotStatusIdle          BotStatus = "idle"
	BotStatusAuthenticating BotStatus = "authenticating"
	BotStatusConnecting    BotStatus = "connecting"
	BotStatusInWorld       BotStatus = "in_world"
	BotStatusDone          BotStatus = "done"
	BotStatusError         BotStatus = "error"
)

// BotResult holds the result of a bot run
type BotResult struct {
	ID     string    `json:"id"`
	Status BotStatus `json:"status"`
	Error  string    `json:"error,omitempty"`
}

// Bot implements the WoW client bot logic
type Bot struct {
	id     string
	config BotConfig
	status BotStatus
	err    error
	mu     sync.Mutex

	world *WorldClient
}

// NewBot creates a new bot
func NewBot(id string, config BotConfig) *Bot {
	// Apply defaults
	if config.Race == 0 {
		config.Race = 5 // Undead
	}
	if config.Class == 0 {
		config.Class = 1 // Warrior
	}

	return &Bot{
		id:     id,
		config: config,
		status: BotStatusIdle,
	}
}

// Run executes the full bot flow
func (b *Bot) Run() BotResult {
	b.setStatus(BotStatusAuthenticating)
	b.log("Starting bot for %s@%s, char: %s", b.config.Username, b.config.AuthServer, b.config.CharacterName)

	// Step 1: Authenticate with authserver
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

	// Set up character list handler
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

	// Start the world client (reads auth challenge, sends auth session, sets up encryption)
	worldErrCh := make(chan error, 1)
	go func() {
		worldErrCh <- b.world.Run()
	}()

	// Wait a bit for auth response
	time.Sleep(500 * time.Millisecond)

	// Step 3: Request character list
	b.world.SendReadyForAccountDataTimes()
	b.world.SendRealmSplit()
	if err := b.world.RequestCharList(); err != nil {
		return b.fail("request char list failed: %v", err)
	}

	// Wait for character list
	var chars []CharEnumEntry
	select {
	case chars = <-charListCh:
	case <-time.After(10 * time.Second):
		return b.fail("timeout waiting for character list")
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
		b.log("Character %s not found, creating...", b.config.CharacterName)
		if err := b.world.CreateCharacter(
			b.config.CharacterName,
			b.config.Race,
			b.config.Class,
			b.config.Gender,
			0, 0, 0, 0, 0, 0, // skin, face, hairStyle, hairColor, facialHair, outfitID
		); err != nil {
			return b.fail("create character failed: %v", err)
		}

		// Wait for create result
		// CHAR_CREATE_SUCCESS = 0x2F (47)
		select {
		case result := <-charCreateCh:
			if result != 0x2F {
				return b.fail("character creation failed with code 0x%X (%d)", result, result)
			}
		case <-time.After(10 * time.Second):
			return b.fail("timeout waiting for character creation")
		}

		b.log("Character created, requesting updated char list")

		// Request updated char list
		b.world.SendReadyForAccountDataTimes()
		b.world.SendRealmSplit()
		if err := b.world.RequestCharList(); err != nil {
			return b.fail("request char list after create failed: %v", err)
		}

		select {
		case chars = <-charListCh:
		case <-time.After(10 * time.Second):
			return b.fail("timeout waiting for char list after create")
		}

		for _, ch := range chars {
			if strings.EqualFold(ch.Name, b.config.CharacterName) {
				charGUID = ch.GUID
				found = true
				b.log("Found created character %s (GUID: %d)", ch.Name, ch.GUID)
				break
			}
		}

		if !found {
			return b.fail("character not found after creation")
		}
	}

	// Step 5: Login with character
	b.log("Logging in with character GUID %d", charGUID)
	if err := b.world.LoginCharacter(charGUID); err != nil {
		return b.fail("login character failed: %v", err)
	}

	// Wait for login to complete (SMSG_LOGIN_VERIFY_WORLD)
	select {
	case <-b.world.loginDone:
	case <-time.After(30 * time.Second):
		return b.fail("timeout waiting for world login")
	case err := <-worldErrCh:
		return b.fail("world connection died during login: %v", err)
	}

	b.setStatus(BotStatusInWorld)
	b.log("Character is in world!")

	// Small delay to let initial world data arrive
	time.Sleep(3 * time.Second)

	// Set active mover (the server expects this)
	b.world.SetActiveMover(charGUID)

	time.Sleep(1 * time.Second)

	// Step 6: Perform actions

	// Send a chat message
	b.log("Sending chat message")
	if err := b.world.SendChatMessage(ChatMsgSay, LangCommon, "Hello from load test bot!"); err != nil {
		b.log("Send chat message error: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Laugh emote
	b.log("Sending laugh emote")
	if err := b.world.SendTextEmote(TextEmoteLaugh, 0); err != nil {
		b.log("Send emote error: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Jump
	b.log("Jumping")
	if err := b.world.SendJump(); err != nil {
		b.log("Send jump error: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Land after jump (clear falling flag)
	if err := b.world.SendHeartbeat(); err != nil {
		b.log("Send heartbeat error: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Step 7: Logout
	b.log("Logging out")
	if err := b.world.SendLogout(); err != nil {
		b.log("Send logout error: %v", err)
	}

	if err := b.world.WaitForLogout(30 * time.Second); err != nil {
		b.log("Logout error: %v (continuing anyway)", err)
	}

	b.world.Close()
	b.setStatus(BotStatusDone)
	b.log("Bot finished successfully")

	return BotResult{
		ID:     b.id,
		Status: BotStatusDone,
	}
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
	}
	if b.err != nil {
		result.Error = b.err.Error()
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
