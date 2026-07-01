package wowsimclient

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"sync"
	"time"
)

// Opcodes (subset needed for the bot)
const (
	SmsgAuthChallenge uint16 = 0x01EC
	CmsgAuthSession   uint16 = 0x01ED
	SmsgAuthResponse  uint16 = 0x01EE

	CmsgCharEnum    uint16 = 0x0037
	SmsgCharEnum    uint16 = 0x003B
	CmsgCharCreate  uint16 = 0x0036
	SmsgCharCreate  uint16 = 0x003A
	CmsgCharDelete  uint16 = 0x0038
	SmsgCharDelete  uint16 = 0x003C
	CmsgPlayerLogin uint16 = 0x003D

	SmsgLoginVerifyWorld uint16 = 0x0236

	CmsgPing uint16 = 0x01DC
	SmsgPong uint16 = 0x01DD

	SmsgTimeSyncReq  uint16 = 0x0390
	CmsgTimeSyncResp uint16 = 0x0391

	CmsgMessageChat uint16 = 0x0095
	SmsgMessageChat uint16 = 0x0096

	CmsgTextEmote uint16 = 0x0104
	SmsgTextEmote uint16 = 0x0105
	SmsgEmote     uint16 = 0x0103

	MsgMoveJump             uint16 = 0x00BB
	MsgMoveFallLand         uint16 = 0x00C9
	MsgMoveHeartbeat        uint16 = 0x00EE
	MsgMoveStartForward     uint16 = 0x00B5
	MsgMoveStop             uint16 = 0x00B7
	MsgMoveStartBackward    uint16 = 0x00B6
	MsgMoveStartStrafeLeft  uint16 = 0x00B8
	MsgMoveStartStrafeRight uint16 = 0x00B9
	MsgMoveStopStrafe       uint16 = 0x00BA
	MsgMoveStartTurnLeft    uint16 = 0x00BC
	MsgMoveStartTurnRight   uint16 = 0x00BD
	MsgMoveStopTurn         uint16 = 0x00BE
	MsgMoveSetFacing        uint16 = 0x00DA
	MsgMoveTeleportAck      uint16 = 0x00C7
	SmsgMoveKnockBack       uint16 = 0x00EF
	SmsgMoveTeleport        uint16 = 0x00C5

	CmsgSetActiveMover uint16 = 0x026A
	CmsgLogoutRequest  uint16 = 0x004B
	SmsgLogoutResponse uint16 = 0x004C
	SmsgLogoutComplete uint16 = 0x004D

	SmsgWardenData uint16 = 0x02E6
	CmsgWardenData uint16 = 0x02E7

	CmsgReadyForAccountDataTimes uint16 = 0x04FF
	SmsgAccountDataTimes         uint16 = 0x0209
	CmsgRealmSplit               uint16 = 0x038C
	SmsgRealmSplit               uint16 = 0x038B

	SmsgAddonInfo     uint16 = 0x02EF
	SmsgTutorialFlags uint16 = 0x00FD
	SmsgCancelCombat  uint16 = 0x014E

	CmsgCompleteCinematic   uint16 = 0x00FC
	CmsgNextCinematicCamera uint16 = 0x00FB

	// Combat opcodes
	CmsgAttackSwing         uint16 = 0x0141
	CmsgAttackStop          uint16 = 0x0142
	SmsgAttackStart         uint16 = 0x0143
	SmsgAttackStop          uint16 = 0x0144
	SmsgAttackerStateUpdate uint16 = 0x014A

	// Spell opcodes
	CmsgCastSpell     uint16 = 0x012E
	SmsgSpellStart    uint16 = 0x0131
	SmsgSpellGo       uint16 = 0x0132
	SmsgSpellFailure  uint16 = 0x0133
	SmsgSpellCooldown uint16 = 0x0134
	SmsgInitialSpells uint16 = 0x012A
	SmsgCooldownEvent uint16 = 0x0135
	SmsgClearCooldown uint16 = 0x01DE
	CmsgCancelCast    uint16 = 0x012F
	CmsgCancelAura    uint16 = 0x0133

	// Update object opcodes
	SmsgUpdateObject         uint16 = 0x00A9
	SmsgDestroyObject        uint16 = 0x00AA
	SmsgCompressedUpdate     uint16 = 0x01F6
	SmsgMonsterMove          uint16 = 0x00DD
	SmsgMonsterMoveTransport uint16 = 0x02AE
	SmsgCompressedMoves      uint16 = 0x02FB
	SmsgMultipleMoves        uint16 = 0x051E

	// Loot opcodes
	CmsgLoot                uint16 = 0x015D
	CmsgLootMoney           uint16 = 0x015E
	CmsgLootRelease         uint16 = 0x015F
	SmsgLootResponse        uint16 = 0x0160
	CmsgAutostoreLootItem   uint16 = 0x0108
	SmsgLootRemoved         uint16 = 0x0162
	SmsgLootReleaseResponse uint16 = 0x0161

	// Item / inventory opcodes
	CmsgAutoequipItem uint16 = 0x010A

	// Quest opcodes
	CmsgQuestgiverHello         uint16 = 0x0184
	SmsgQuestgiverQuestList     uint16 = 0x0185
	CmsgQuestgiverAcceptQuest   uint16 = 0x0189
	CmsgQuestgiverCompleteQuest uint16 = 0x018A
	CmsgQuestgiverChooseReward  uint16 = 0x018E
	SmsgQuestgiverQuestDetails  uint16 = 0x0188
	SmsgQuestgiverRequestItems  uint16 = 0x018B
	SmsgQuestgiverOfferReward   uint16 = 0x018D
	SmsgQuestupdateComplete     uint16 = 0x01A8

	// Group opcodes
	CmsgGroupInvite        uint16 = 0x006E
	SmsgGroupInvite        uint16 = 0x006F
	CmsgGroupAccept        uint16 = 0x0072
	CmsgGroupSetLeader     uint16 = 0x0078
	SmsgGroupList          uint16 = 0x007D
	SmsgPartyCommandResult uint16 = 0x007F

	// Level up
	SmsgLevelupInfo uint16 = 0x01D4

	// Death handling
	CmsgRepopRequest  uint16 = 0x015A
	CmsgReclaimCorpse uint16 = 0x01D2

	// Target / selection
	CmsgSetSelection uint16 = 0x013D

	// Misc
	SmsgNewWorld        uint16 = 0x003E
	SmsgTransferPending uint16 = 0x003F
	SmsgAiReaction      uint16 = 0x013C
	SmsgPowerUpdate     uint16 = 0x0480

	// Speed change
	SmsgForceRunSpeedChange    uint16 = 0x00E2
	CmsgForceRunSpeedChangeAck uint16 = 0x00E3

	// Name query
	CmsgNameQuery             uint16 = 0x0050
	SmsgNameQueryResponse     uint16 = 0x0051
	CmsgCreatureQuery         uint16 = 0x0060
	SmsgCreatureQueryResponse uint16 = 0x0061

	// Set sheathed
	CmsgSetSheathed uint16 = 0x01E0
)

// Chat message types
const (
	ChatMsgSay   uint32 = 0x01
	ChatMsgYell  uint32 = 0x06
	ChatMsgEmote uint32 = 0x08
)

// Laugh emote ID
const (
	TextEmoteLaugh uint32 = 0x3C // 60 = LAUGH
)

// Language
const (
	LangUniversal uint32 = 0x00
	LangOrcish    uint32 = 0x01
	LangCommon    uint32 = 0x07
)

// Movement speeds from AzerothCore Unit.cpp (yards per second)
const (
	BaseSpeedWalk     float32 = 2.5
	BaseSpeedRun      float32 = 7.0
	BaseSpeedRunBack  float32 = 4.5
	BaseSpeedSwim     float32 = 4.722222
	BaseSpeedSwimBack float32 = 2.5
	BaseSpeedFlight   float32 = 7.0
)

// Movement flags
const (
	MoveFlagNone        uint32 = 0x00000000
	MoveFlagForward     uint32 = 0x00000001
	MoveFlagBackward    uint32 = 0x00000002
	MoveFlagStrafeLeft  uint32 = 0x00000004
	MoveFlagStrafeRight uint32 = 0x00000008
	MoveFlagTurnLeft    uint32 = 0x00000010
	MoveFlagTurnRight   uint32 = 0x00000020
	MoveFlagFalling     uint32 = 0x00001000
	MoveFlagSwimming    uint32 = 0x00200000
	MoveFlagFlying      uint32 = 0x02000000
)

// ObjectType constants
const (
	ObjectTypeObject    uint8 = 0
	ObjectTypeItem      uint8 = 1
	ObjectTypeContainer uint8 = 2
	ObjectTypeUnit      uint8 = 3
	ObjectTypePlayer    uint8 = 4
	ObjectTypeGameObj   uint8 = 5
	ObjectTypeDynObj    uint8 = 6
	ObjectTypeCorpse    uint8 = 7
)

// Update types
const (
	UpdateTypeValues            uint8 = 0
	UpdateTypeMovement          uint8 = 1
	UpdateTypeCreateObject      uint8 = 2
	UpdateTypeCreateObject2     uint8 = 3
	UpdateTypeOutOfRangeObjects uint8 = 4
	UpdateTypeNearObjects       uint8 = 5
)

// Unit fields from AzerothCore UpdateFields.h (OBJECT_END = 0x0006)
const (
	ObjectFieldGUID          = 0x0000 // 2 uint32s
	ObjectFieldType          = 0x0002
	UnitFieldEntry           = 0x0003 // OBJECT_FIELD_ENTRY
	UnitFieldTarget          = 0x0012 // OBJECT_END + 0x000C = 0x0012 (2 uint32s = GUID)
	UnitFieldBytes0          = 0x0017 // OBJECT_END + 0x0011 = 0x0017 (race, class, gender, powertype)
	UnitFieldHealth          = 0x0018 // OBJECT_END + 0x0012
	UnitFieldPower1          = 0x0019 // OBJECT_END + 0x0013 (mana/rage/energy)
	UnitFieldMaxHealth       = 0x0020 // OBJECT_END + 0x001A
	UnitFieldMaxPower1       = 0x0021 // OBJECT_END + 0x001B
	UnitFieldLevel           = 0x0036 // OBJECT_END + 0x0030
	UnitFieldFaction         = 0x0037 // OBJECT_END + 0x0031 (faction template)
	UnitFieldFlags           = 0x003B // OBJECT_END + 0x0035
	UnitFieldFlags2          = 0x003C // OBJECT_END + 0x0036
	UnitFieldDisplayID       = 0x0043 // OBJECT_END + 0x003D
	UnitFieldNativeDisplayID = 0x0044 // OBJECT_END + 0x003E
	UnitDynamicFlags         = 0x004F // OBJECT_END + 0x0049
	UnitNPCFlags             = 0x0052 // OBJECT_END + 0x004C
)

// From AC SharedDefines.h - dyn flags for dead/lootable corpses
const (
	UnitDynflagLootable = 0x0001
	UnitDynflagDead     = 0x0020
)

// UnitFlags
const (
	UnitFlagInCombat      uint32 = 0x00080000
	UnitFlagNotAttackable uint32 = 0x00000002
	UnitFlagDisarmed      uint32 = 0x00200000
	UnitFlagPacified      uint32 = 0x00020000
	UnitFlagStunned       uint32 = 0x00040000
	UnitFlagDead          uint32 = 0x20000000
)

// CharEnumEntry holds character data from SMSG_CHAR_ENUM
type CharEnumEntry struct {
	GUID  uint64
	Name  string
	Race  uint8
	Class uint8
	Level uint8
}

// WorldObject represents a tracked object in the game world
type WorldObject struct {
	GUID   uint64
	TypeID uint8 // ObjectType*
	Entry  uint32
	Values map[uint16]uint32

	// valuesMu protects concurrent access to Values (and derived) from the packet read goroutine
	// and the bot AI goroutine(s). Prevents data races on map and stale health/flags views
	// (e.g. seeing positive health on actually dead mobs killed by others).
	valuesMu sync.RWMutex

	// Position data
	PosX, PosY, PosZ float32
	Orientation      float32
	MapID            uint32

	// Last time we received a position update for this object (MonsterMove or movement block).
	// Used to detect stale positions for targets (e.g. mob wandered but we stopped receiving updates).
	LastPosUpdate time.Time

	// Last time we received *any* update for this object (values, movement, etc.).
	// Indicates the server still considers it visible to us.
	LastSeen time.Time

	// Movement interpolation
	DestX, DestY, DestZ    float32
	IsMoving               bool
	MoveStartTime          time.Time
	MoveDuration           time.Duration
	StartX, StartY, StartZ float32

	// Derived convenience fields
	Name     string
	IsPlayer bool
}

// Clone returns a deep copy of the WorldObject. This gives callers (e.g. AI logic)
// their own private version of the world state snapshot, reducing races with
// concurrent packet updates to the live cache.
func (o *WorldObject) Clone() *WorldObject {
	if o == nil {
		return nil
	}
	o.valuesMu.RLock()
	vals := make(map[uint16]uint32, len(o.Values))
	for k, v := range o.Values {
		vals[k] = v
	}
	o.valuesMu.RUnlock()

	return &WorldObject{
		GUID:          o.GUID,
		TypeID:        o.TypeID,
		Entry:         o.Entry,
		Values:        vals,
		PosX:          o.PosX,
		PosY:          o.PosY,
		PosZ:          o.PosZ,
		Orientation:   o.Orientation,
		MapID:         o.MapID,
		DestX:         o.DestX,
		DestY:         o.DestY,
		DestZ:         o.DestZ,
		IsMoving:      o.IsMoving,
		MoveStartTime: o.MoveStartTime,
		MoveDuration:  o.MoveDuration,
		StartX:        o.StartX,
		StartY:        o.StartY,
		StartZ:        o.StartZ,
		Name:          o.Name,
		IsPlayer:      o.IsPlayer,
	}
}

// value returns a field under lock to avoid data races with packet updates.
func (o *WorldObject) value(field uint16) uint32 {
	o.valuesMu.RLock()
	defer o.valuesMu.RUnlock()
	return o.Values[field]
}

// setValue sets under lock.
func (o *WorldObject) setValue(field uint16, v uint32) {
	o.valuesMu.Lock()
	defer o.valuesMu.Unlock()
	o.Values[field] = v
}

// Health returns the current health of the object, or 0 if unknown.
func (o *WorldObject) Health() uint32 {
	return o.value(UnitFieldHealth)
}

// MaxHealth returns the maximum health of the object, or 0 if unknown.
func (o *WorldObject) MaxHealth() uint32 {
	return o.value(UnitFieldMaxHealth)
}

// Level returns the level of the unit, or 0 if unknown.
func (o *WorldObject) Level() uint32 {
	return o.value(UnitFieldLevel)
}

// IsAlive returns true if the object appears to be alive.
func (o *WorldObject) IsAlive() bool {
	flags := o.value(UnitFieldFlags)
	if flags&UnitFlagDead != 0 {
		return false
	}
	dynFlags := o.value(UnitDynamicFlags)
	if dynFlags&(UnitDynflagDead|UnitDynflagLootable) != 0 {
		return false
	}
	h := o.Health()
	mh := o.MaxHealth()
	// If we know the max health (object data received) and current is 0, it's dead
	if mh > 0 && h == 0 {
		return false
	}
	if h > 0 {
		return true
	}
	return flags&UnitFlagDead == 0
}

// IsUnit returns true if the object type is unit (NPC) or player.
func (o *WorldObject) IsUnit() bool {
	return o.TypeID == ObjectTypeUnit || o.TypeID == ObjectTypePlayer
}

// InterpolatedPosition returns the estimated current position for a moving object.
func (o *WorldObject) InterpolatedPosition() (float32, float32, float32) {
	if !o.IsMoving || o.MoveDuration <= 0 {
		return o.PosX, o.PosY, o.PosZ
	}
	elapsed := time.Since(o.MoveStartTime)
	t := float32(elapsed.Seconds()) / float32(o.MoveDuration.Seconds())
	if t >= 1.0 {
		o.PosX = o.DestX
		o.PosY = o.DestY
		o.PosZ = o.DestZ
		o.IsMoving = false
		return o.PosX, o.PosY, o.PosZ
	}
	ix := o.StartX + (o.DestX-o.StartX)*t
	iy := o.StartY + (o.DestY-o.StartY)*t
	iz := o.StartZ + (o.DestZ-o.StartZ)*t
	return ix, iy, iz
}

// DistanceTo computes 3D distance to another position, using interpolated position for moving objects.
func (o *WorldObject) DistanceTo(x, y, z float32) float32 {
	ox, oy, oz := o.InterpolatedPosition()
	dx := ox - x
	dy := oy - y
	dz := oz - z
	return float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
}

// SpellCooldown tracks a spell's cooldown expiry
type SpellCooldown struct {
	SpellID   uint32
	ExpiresAt time.Time
}

// KnownSpell represents a spell the bot knows
type KnownSpell struct {
	SpellID uint32
	Active  bool
}

// LootItem represents an item available for looting
type LootItem struct {
	Index    uint8
	ItemID   uint32
	Quantity uint32
}

// WorldClient handles the world server protocol
type WorldClient struct {
	conn       net.Conn
	readBuf    *bufio.Reader // buffered for fewer syscalls on readPacket (critical for 500+ bots)
	username   string
	sessionKey []byte

	encryptServer *rc4.Cipher
	encryptClient *rc4.Cipher
	encrypted     bool

	sendMu sync.Mutex

	charGUID                      uint64
	timeSyncCounter               uint32
	posX, posY, posZ, orientation float32
	mapID                         uint32
	moveSpeed                     float32
	currentMoveFlags              uint32 // last used for movement (to use in acks etc)

	// Cached packed GUID to avoid repeated allocation in hot send path
	packedGUID []byte
	decompBuf  []byte // reusable for compressed updates

	loginDone  chan struct{}
	logoutDone chan struct{}

	stopChan chan struct{}
	stopped  bool

	lastError error

	// For movement debug logging (use observer client to log other player's packets)
	movDebugMu   sync.Mutex
	lastMovDebug map[uint64]struct {
		ts      uint32
		x, y, z float32
		wall    time.Time
	}

	// Object tracking
	objectsMu sync.RWMutex
	objects   map[uint64]*WorldObject

	// Known spells
	spellsMu    sync.RWMutex
	knownSpells map[uint32]*KnownSpell

	// Cooldowns
	cooldownsMu sync.RWMutex
	cooldowns   map[uint32]*SpellCooldown

	// Combat state
	combatMu      sync.RWMutex
	inCombat      bool
	targetGUID    uint64
	attackingGUID uint64

	// Player stats
	statsMu   sync.RWMutex
	health    uint32
	maxHealth uint32
	power     uint32
	maxPower  uint32
	level     uint32

	// Callbacks
	logFunc            func(format string, args ...interface{})
	OnCharList         func(chars []CharEnumEntry)
	OnCharCreateResult func(data []byte)
	OnChatMessage      func(senderName, message string, msgType uint8)
	OnLevelUp          func(newLevel uint32)
	OnDeath            func()
	OnKill             func(victimGUID uint64)
	OnObjectUpdate     func(guid uint64, obj *WorldObject)
	OnObjectRemove     func(guid uint64)
	OnLootOpened       func(lootGUID uint64, items []LootItem)
	OnSpellCastResult  func(spellID uint32, success bool)
	OnCombatStart      func(attackerGUID, victimGUID uint64)
	OnCombatStop       func()
	OnInvalidTarget    func(victimGUID uint64) // server told us attack on this is invalid (dead, friendly, etc.)
	OnPacket           func(opcode uint16, data []byte)
}

// NewWorldClient creates a world client
func NewWorldClient(username string, sessionKey []byte, logFunc func(string, ...interface{})) *WorldClient {
	return &WorldClient{
		username:    strings.ToUpper(username),
		sessionKey:  sessionKey,
		loginDone:   make(chan struct{}),
		logoutDone:  make(chan struct{}),
		stopChan:    make(chan struct{}),
		logFunc:     logFunc,
		objects:     make(map[uint64]*WorldObject),
		knownSpells: make(map[uint32]*KnownSpell),
		cooldowns:   make(map[uint32]*SpellCooldown),
		moveSpeed:   BaseSpeedRun,
		lastMovDebug: make(map[uint64]struct {
			ts      uint32
			x, y, z float32
			wall    time.Time
		}),
	}
}

// Connect connects to the world server
func (w *WorldClient) Connect(worldAddr string) error {
	var err error
	w.conn, err = net.DialTimeout("tcp", worldAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("connect to worldserver: %w", err)
	}

	// give a rest for my router
	worldAddr = strings.Replace(worldAddr, "192.168.178.110", "127.0.0.1", -1)

	// Use buffered reader to drastically reduce syscalls for header+payload reads.
	// 16KB is good for bursts of SMSG_COMPRESSED_UPDATE with many entities.
	w.readBuf = bufio.NewReaderSize(w.conn, 16384)

	// Larger kernel read buffer helps when we do get a syscall; bufio will
	// still be the main win for amortizing them.
	if tcp, ok := w.conn.(*net.TCPConn); ok {
		tcp.SetReadBuffer(65536)
	}
	return nil
}

// Run starts reading and processing packets. Blocks until connection closes.
func (w *WorldClient) Run() error {
	// Read SMSG_AUTH_CHALLENGE
	if err := w.handleAuthChallenge(); err != nil {
		w.conn.Close()
		return fmt.Errorf("auth challenge: %w", err)
	}

	// Run the packet reading loop (blocks until connection closes or error)
	w.readLoop()
	return w.lastError
}

func (w *WorldClient) handleAuthChallenge() error {
	// Read server header (unencrypted): size(2 big-endian) + opcode(2 little-endian)
	// Use readBuf to avoid syscall per small read.
	if w.conn != nil {
		_ = w.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	}
	header := make([]byte, 4)
	if _, err := io.ReadFull(w.readBuf, header); err != nil {
		return err
	}

	size := binary.BigEndian.Uint16(header[0:2]) - 2
	opcode := binary.LittleEndian.Uint16(header[2:4])

	if opcode != SmsgAuthChallenge {
		return fmt.Errorf("expected SMSG_AUTH_CHALLENGE (0x%X), got 0x%X", SmsgAuthChallenge, opcode)
	}

	data := make([]byte, size)
	if _, err := io.ReadFull(w.readBuf, data); err != nil {
		return err
	}

	// Parse: uint32(1) + authSeed(4 bytes) + randomBytes(32 bytes)
	r := bytes.NewReader(data)
	var one uint32
	binary.Read(r, binary.LittleEndian, &one)

	authSeed := make([]byte, 4)
	r.Read(authSeed)

	// Generate client seed
	clientSeed := make([]byte, 4)
	rand.Read(clientSeed)

	// Compute digest: SHA1(username, t(zeros), clientSeed, authSeed, sessionKey)
	t := []byte{0, 0, 0, 0}
	h := sha1.New()
	h.Write([]byte(w.username))
	h.Write(t)
	h.Write(clientSeed)
	h.Write(authSeed)
	h.Write(w.sessionKey)
	digest := h.Sum(nil)

	// Build CMSG_AUTH_SESSION
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint32(12340)) // build
	binary.Write(buf, binary.LittleEndian, uint32(0))     // loginServerID
	buf.Write(append([]byte(w.username), 0))              // null-terminated username
	binary.Write(buf, binary.LittleEndian, uint32(0))     // loginServerType
	buf.Write(clientSeed)
	binary.Write(buf, binary.LittleEndian, uint32(0)) // regionID
	binary.Write(buf, binary.LittleEndian, uint32(0)) // battlegroundID
	binary.Write(buf, binary.LittleEndian, uint32(1)) // realmID
	binary.Write(buf, binary.LittleEndian, uint64(0)) // dosResponse
	buf.Write(digest)

	// Addon info (minimal - just count=0 compressed)
	addonData := buildMinimalAddonInfo()
	buf.Write(addonData)

	w.sendPacketUnencrypted(CmsgAuthSession, buf.Bytes())

	// Set up encryption
	w.setupEncryption()

	return nil
}

func buildMinimalAddonInfo() []byte {
	// Addon count = 0, compressed with zlib
	// Raw: uint32(0) = 4 bytes saying 0 addons
	// We need to zlib compress: size(uint32) + compressed data
	// Actually let's just send the compressed addon data
	// The server reads: uint32(uncompressed_size) + zlib(addon_data)
	// If size is 0, server skips it
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint32(0)) // decompressed size = 0
	return buf.Bytes()
}

func (w *WorldClient) setupEncryption() {
	serverEncryptionKey := []byte{0xCC, 0x98, 0xAE, 0x04, 0xE8, 0x97, 0xEA, 0xCA, 0x12, 0xDD, 0xC0, 0x93, 0x42, 0x91, 0x53, 0x57}
	clientEncryptionKey := []byte{0xC2, 0xB3, 0x72, 0x3C, 0xC6, 0xAE, 0xD9, 0xB5, 0x34, 0x3C, 0x53, 0xEE, 0x2F, 0x43, 0x67, 0xCE}

	// Server key is used by server to encrypt, so we use it to decrypt
	sMac := hmac.New(sha1.New, serverEncryptionKey)
	sMac.Write(w.sessionKey)
	sKey := sMac.Sum(nil)

	// Client key is used by client to encrypt
	cMac := hmac.New(sha1.New, clientEncryptionKey)
	cMac.Write(w.sessionKey)
	cKey := cMac.Sum(nil)

	w.encryptServer, _ = rc4.NewCipher(sKey)
	w.encryptClient, _ = rc4.NewCipher(cKey)

	// Drop first 1024 bytes (ARC4-drop1024)
	drop := make([]byte, 1024)
	w.encryptServer.XORKeyStream(drop, drop)
	drop = make([]byte, 1024)
	w.encryptClient.XORKeyStream(drop, drop)

	w.encrypted = true
}

func (w *WorldClient) readLoop() {
	defer func() {
		if !w.stopped {
			w.stopped = true
			close(w.stopChan)
		}
	}()

	for {
		opcode, data, err := w.readPacket()
		if err != nil {
			if !w.stopped {
				w.lastError = err
			}
			return
		}

		w.handlePacket(opcode, data)
	}
}

func (w *WorldClient) readPacket() (uint16, []byte, error) {
	// Use buffered reader for vastly fewer syscalls. Each ReadFull here
	// is now a fast buffer op in the common case; the OS read happens
	// in larger chunks (16KB).
	if w.readBuf == nil {
		w.readBuf = bufio.NewReaderSize(w.conn, 16384)
	}

	// Set a read deadline so we don't block forever on dead/half-open connections.
	// The AI loop sends pings every 30s; give enough headroom for lag.
	if w.conn != nil {
		_ = w.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	}

	var hdr [5]byte // max header size we ever need (large encrypted)

	if w.encrypted {
		// Encrypted header: 4 or 5 bytes.
		if _, err := io.ReadFull(w.readBuf, hdr[:4]); err != nil {
			return 0, nil, fmt.Errorf("read header: %w", err)
		}
		w.encryptServer.XORKeyStream(hdr[:4], hdr[:4])

		isLarge := hdr[0]&0x80 != 0
		var size uint32
		var opcode uint16
		if isLarge {
			if _, err := io.ReadFull(w.readBuf, hdr[4:5]); err != nil {
				return 0, nil, err
			}
			w.encryptServer.XORKeyStream(hdr[4:5], hdr[4:5])
			size = (uint32(hdr[0]&0x7F) << 16) | (uint32(hdr[1]) << 8) | uint32(hdr[2])
			opcode = binary.LittleEndian.Uint16(hdr[3:5])
		} else {
			size = (uint32(hdr[0]) << 8) | uint32(hdr[1])
			opcode = binary.LittleEndian.Uint16(hdr[2:4])
		}

		if size < 2 {
			return opcode, nil, nil
		}
		payloadSize := int(size) - 2
		if payloadSize > 10*1024*1024 {
			return 0, nil, fmt.Errorf("packet too large: %d", payloadSize)
		}
		if payloadSize == 0 {
			return opcode, nil, nil
		}

		// Extend deadline for the (potentially large) payload.
		if w.conn != nil {
			_ = w.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		}
		data := make([]byte, payloadSize)
		if _, err := io.ReadFull(w.readBuf, data); err != nil {
			return 0, nil, err
		}
		return opcode, data, nil
	}

	// Unencrypted (only auth phase)
	if w.conn != nil {
		_ = w.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	}
	if _, err := io.ReadFull(w.readBuf, hdr[:1]); err != nil {
		return 0, nil, fmt.Errorf("read first byte: %w", err)
	}

	isLarge := hdr[0]&0x80 != 0
	if isLarge {
		if _, err := io.ReadFull(w.readBuf, hdr[1:5]); err != nil { // 4 more bytes
			return 0, nil, err
		}
		size := (uint32(hdr[0]&0x7F) << 16) | (uint32(hdr[1]) << 8) | uint32(hdr[2])
		opcode := binary.LittleEndian.Uint16(hdr[3:5])

		if size < 2 {
			return opcode, nil, nil
		}
		payloadSize := int(size) - 2
		if payloadSize == 0 {
			return opcode, nil, nil
		}
		data := make([]byte, payloadSize)
		if _, err := io.ReadFull(w.readBuf, data); err != nil {
			return 0, nil, err
		}
		return opcode, data, nil
	}

	// Normal unencrypted header
	if _, err := io.ReadFull(w.readBuf, hdr[1:4]); err != nil { // 1 size + 2 opcode
		return 0, nil, err
	}
	size := (uint32(hdr[0]) << 8) | uint32(hdr[1])
	opcode := binary.LittleEndian.Uint16(hdr[2:4])

	if size < 2 {
		return opcode, nil, nil
	}
	payloadSize := int(size) - 2
	if payloadSize == 0 {
		return opcode, nil, nil
	}
	if w.conn != nil {
		_ = w.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	}
	data := make([]byte, payloadSize)
	if _, err := io.ReadFull(w.readBuf, data); err != nil {
		return 0, nil, err
	}
	return opcode, data, nil
}

func (w *WorldClient) sendPacketUnencrypted(opcode uint16, data []byte) error {
	header := make([]byte, 6)
	binary.BigEndian.PutUint16(header[0:2], uint16(len(data)+4))
	binary.LittleEndian.PutUint32(header[2:6], uint32(opcode))

	w.sendMu.Lock()
	defer w.sendMu.Unlock()

	_, err := w.conn.Write(append(header, data...))
	return err
}

func (w *WorldClient) sendPacket(opcode uint16, data []byte) error {
	// Client sends: size(2, big-endian) + opcode(4, little-endian)
	header := make([]byte, 6)
	binary.BigEndian.PutUint16(header[0:2], uint16(len(data)+4))
	binary.LittleEndian.PutUint32(header[2:6], uint32(opcode))

	w.sendMu.Lock()
	defer w.sendMu.Unlock()

	if w.encrypted {
		w.encryptClient.XORKeyStream(header, header)
	}

	packet := append(header, data...)
	_, err := w.conn.Write(packet)
	return err
}

func (w *WorldClient) handlePacket(opcode uint16, data []byte) {
	// Generic callback for raw packet access
	if w.OnPacket != nil {
		w.OnPacket(opcode, data)
	}

	switch opcode {
	case SmsgAuthResponse:
		w.handleAuthResponse(data)
	case SmsgCharEnum:
		w.handleCharEnum(data)
	case SmsgCharCreate:
		if w.OnCharCreateResult != nil {
			w.OnCharCreateResult(data)
		}
	case SmsgLoginVerifyWorld:
		w.handleLoginVerifyWorld(data)
	case SmsgTimeSyncReq:
		w.handleTimeSyncReq(data)
	case SmsgPong:
		// Pong received
	case SmsgWardenData:
		// Ignore warden
	case SmsgLogoutResponse:
		w.handleLogoutResponse(data)
	case SmsgLogoutComplete:
		w.handleLogoutComplete()
	case SmsgCancelCombat:
		w.handleCancelCombat()

	// Combat
	case SmsgAttackStart:
		w.handleAttackStart(data)
	case SmsgAttackStop:
		w.handleAttackStop(data)
	case SmsgAttackerStateUpdate:
		w.handleAttackerStateUpdate(data)

	// Spells
	case SmsgInitialSpells:
		w.handleInitialSpells(data)
	case SmsgSpellGo:
		w.handleSpellGo(data)
	case SmsgSpellFailure:
		w.handleSpellFailure(data)
	case SmsgSpellCooldown:
		w.handleSpellCooldown(data)
	case SmsgCooldownEvent:
		w.handleCooldownEvent(data)
	case SmsgClearCooldown:
		w.handleClearCooldown(data)

	// Object updates
	case SmsgUpdateObject:
		w.handleUpdateObject(data)
	case SmsgCompressedUpdate:
		w.handleCompressedUpdateObject(data)
	case SmsgDestroyObject:
		w.handleDestroyObject(data)
	case SmsgMonsterMove:
		w.handleMonsterMove(data)
	case SmsgMonsterMoveTransport:
		w.handleMonsterMoveTransport(data)
	case SmsgCompressedMoves:
		w.handleCompressedMoves(data)
	case SmsgMultipleMoves:
		w.handleMultipleMoves(data)
	case MsgMoveTeleportAck:
		w.handleMoveTeleportAck(data)
	case SmsgMoveKnockBack:
		w.handleMoveKnockBack(data)
	case SmsgMoveTeleport:
		w.handleMoveTeleport(data)

	// Direct movement packets (for players and possibly relayed for some units/creatures)
	case MsgMoveStartForward, MsgMoveStop, MsgMoveHeartbeat, MsgMoveSetFacing,
		MsgMoveStartBackward, MsgMoveJump, MsgMoveFallLand,
		MsgMoveStartStrafeLeft, MsgMoveStartStrafeRight, MsgMoveStopStrafe,
		MsgMoveStartTurnLeft, MsgMoveStartTurnRight, MsgMoveStopTurn:
		w.handleMovementPacket(opcode, data)

	// Loot
	case SmsgLootResponse:
		w.handleLootResponse(data)

	// Chat
	case SmsgMessageChat:
		w.handleChatMessage(data)

	// Level up
	case SmsgLevelupInfo:
		w.handleLevelUp(data)

	// Power
	case SmsgPowerUpdate:
		w.handlePowerUpdate(data)

	case SmsgNewWorld:
		w.handleNewWorld(data)
	case SmsgTransferPending:
		w.log("Transfer pending received")

	// Speed
	case SmsgForceRunSpeedChange:
		w.handleForceSpeedChange(data)

	default:
		// Ignore most unhandled opcodes
	}
}

func (w *WorldClient) handleAuthResponse(data []byte) {
	if len(data) == 0 {
		w.log("Auth response: empty data")
		return
	}

	result := data[0]
	if result != 12 { // AUTH_OK = 12
		w.log("Auth response failed with code %d", result)
		w.lastError = fmt.Errorf("world auth failed with code %d", result)
		return
	}

	w.log("World auth successful")
}

func (w *WorldClient) handleCharEnum(data []byte) {
	if len(data) == 0 {
		return
	}

	r := bytes.NewReader(data)
	count, _ := r.ReadByte()

	w.log("Character enum: %d characters", count)

	var chars []CharEnumEntry
	for i := byte(0); i < count; i++ {
		entry := w.parseCharEnumEntry(r)
		if entry != nil {
			chars = append(chars, *entry)
		}
	}

	if w.OnCharList != nil {
		w.OnCharList(chars)
	}
}

func (w *WorldClient) parseCharEnumEntry(r *bytes.Reader) *CharEnumEntry {
	entry := &CharEnumEntry{}

	if err := binary.Read(r, binary.LittleEndian, &entry.GUID); err != nil {
		return nil
	}

	// Read null-terminated name
	var nameBytes []byte
	for {
		b, err := r.ReadByte()
		if err != nil || b == 0 {
			break
		}
		nameBytes = append(nameBytes, b)
	}
	entry.Name = string(nameBytes)

	// race(1) + class(1) + gender(1) + skin(1) + face(1) + hairStyle(1) + hairColor(1) + facialHair(1) + level(1)
	var race, class, gender, skin, face, hairStyle, hairColor, facialHair, level uint8
	binary.Read(r, binary.LittleEndian, &race)
	binary.Read(r, binary.LittleEndian, &class)
	binary.Read(r, binary.LittleEndian, &gender)
	binary.Read(r, binary.LittleEndian, &skin)
	binary.Read(r, binary.LittleEndian, &face)
	binary.Read(r, binary.LittleEndian, &hairStyle)
	binary.Read(r, binary.LittleEndian, &hairColor)
	binary.Read(r, binary.LittleEndian, &facialHair)
	binary.Read(r, binary.LittleEndian, &level)

	entry.Race = race
	entry.Class = class
	entry.Level = level

	// zone(4) + map(4) + x(4) + y(4) + z(4)
	skip := make([]byte, 20)
	r.Read(skip)

	// guildID(4)
	skip = make([]byte, 4)
	r.Read(skip)

	// charFlags(4)
	skip = make([]byte, 4)
	r.Read(skip)

	// customizationFlags(4) (at_login flags)
	skip = make([]byte, 4)
	r.Read(skip)

	// firstLogin(1)
	r.ReadByte()

	// petDisplayID(4) + petLevel(4) + petFamily(4)
	skip = make([]byte, 12)
	r.Read(skip)

	// Equipment: 23 slots * (displayID(4) + inventoryType(1) + enchantAura(4)) = 23 * 9 = 207
	skip = make([]byte, 23*9)
	r.Read(skip)

	return entry
}

func (w *WorldClient) handleLoginVerifyWorld(data []byte) {
	// Parse: mapID(4) + posX(4) + posY(4) + posZ(4) + orientation(4)
	if len(data) >= 20 {
		r := bytes.NewReader(data)
		var mapID uint32
		binary.Read(r, binary.LittleEndian, &mapID)
		binary.Read(r, binary.LittleEndian, &w.posX)
		binary.Read(r, binary.LittleEndian, &w.posY)
		binary.Read(r, binary.LittleEndian, &w.posZ)
		binary.Read(r, binary.LittleEndian, &w.orientation)
		w.mapID = mapID
		// (debug log removed)
	} else {
		w.log("Login verified - character is in world!")
	}
	select {
	case <-w.loginDone:
	default:
		close(w.loginDone)
	}
}

func (w *WorldClient) handleTimeSyncReq(data []byte) {
	if len(data) < 4 {
		return
	}
	counter := binary.LittleEndian.Uint32(data[0:4])

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, counter)
	binary.Write(buf, binary.LittleEndian, uint32(getMSTime()))

	w.sendPacket(CmsgTimeSyncResp, buf.Bytes())
}

func (w *WorldClient) handleLogoutResponse(data []byte) {
	// SMSG_LOGOUT_RESPONSE: reason(4) + instant(1)
	if len(data) >= 5 {
		reason := binary.LittleEndian.Uint32(data[0:4])
		if reason != 0 {
			w.log("Logout denied with reason %d", reason)
		}
	}
}

func (w *WorldClient) handleLogoutComplete() {
	w.log("Logout complete")
	w.stopped = true
	select {
	case <-w.logoutDone:
	default:
		close(w.logoutDone)
	}
}

// Public methods for bot actions

// RequestCharList sends CMSG_CHAR_ENUM
func (w *WorldClient) RequestCharList() error {
	return w.sendPacket(CmsgCharEnum, nil)
}

// SendReadyForAccountDataTimes sends CMSG_READY_FOR_ACCOUNT_DATA_TIMES
func (w *WorldClient) SendReadyForAccountDataTimes() error {
	return w.sendPacket(CmsgReadyForAccountDataTimes, nil)
}

// SendRealmSplit sends CMSG_REALM_SPLIT
func (w *WorldClient) SendRealmSplit() error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint32(0xFFFFFFFF))
	return w.sendPacket(CmsgRealmSplit, buf.Bytes())
}

// CreateCharacter sends CMSG_CHAR_CREATE
func (w *WorldClient) CreateCharacter(name string, race, class, gender, skin, face, hairStyle, hairColor, facialHair, outfitID uint8) error {
	buf := new(bytes.Buffer)
	buf.Write(append([]byte(name), 0))
	buf.WriteByte(race)
	buf.WriteByte(class)
	buf.WriteByte(gender)
	buf.WriteByte(skin)
	buf.WriteByte(face)
	buf.WriteByte(hairStyle)
	buf.WriteByte(hairColor)
	buf.WriteByte(facialHair)
	buf.WriteByte(outfitID)
	return w.sendPacket(CmsgCharCreate, buf.Bytes())
}

// DeleteCharacter sends CMSG_CHAR_DELETE for the given GUID.
// Call RequestCharList before/after as needed to refresh the list.
// This should only be used when explicitly allowed (orchestrator or --delete-existing-chars).
func (w *WorldClient) DeleteCharacter(guid uint64) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, guid)
	return w.sendPacket(CmsgCharDelete, buf.Bytes())
}

// LoginCharacter sends CMSG_PLAYER_LOGIN
func (w *WorldClient) LoginCharacter(guid uint64) error {
	w.charGUID = guid
	// Precompute packed GUID once (hot path optimization for sends)
	w.packedGUID = make([]byte, 0, 9)
	packGUID := make([]byte, 9)
	packGUID[0] = 0
	size := 1
	g := guid
	for i := uint8(0); g != 0; i++ {
		if g&0xFF != 0 {
			packGUID[0] |= 1 << i
			packGUID[size] = byte(g & 0xFF)
			size++
		}
		g >>= 8
	}
	w.packedGUID = append(w.packedGUID, packGUID[:size]...)
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, guid)
	return w.sendPacket(CmsgPlayerLogin, buf.Bytes())
}

// SetActiveMover sends CMSG_SET_ACTIVE_MOVER
func (w *WorldClient) SetActiveMover(guid uint64) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, guid)
	return w.sendPacket(CmsgSetActiveMover, buf.Bytes())
}

// SendChatMessage sends a chat message
func (w *WorldClient) SendChatMessage(msgType, lang uint32, message string) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, msgType)
	binary.Write(buf, binary.LittleEndian, lang)
	buf.Write(append([]byte(message), 0))
	return w.sendPacket(CmsgMessageChat, buf.Bytes())
}

// SendTextEmote sends a text emote (e.g., laugh)
func (w *WorldClient) SendTextEmote(emoteID uint32, targetGUID uint64) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, emoteID)
	binary.Write(buf, binary.LittleEndian, uint32(0xFFFFFFFF)) // emote num (not used)
	binary.Write(buf, binary.LittleEndian, targetGUID)
	return w.sendPacket(CmsgTextEmote, buf.Bytes())
}

// SendJump sends a jump movement packet
func (w *WorldClient) SendJump() error {
	buf := new(bytes.Buffer)
	writePackedGUID(buf, w.charGUID)
	binary.Write(buf, binary.LittleEndian, uint32(0x00001000)) // movementFlags: MOVEMENTFLAG_FALLING
	binary.Write(buf, binary.LittleEndian, uint16(0))          // movementFlags2
	binary.Write(buf, binary.LittleEndian, uint32(getMSTime()))
	// Position x, y, z, orientation - use stored position if available
	binary.Write(buf, binary.LittleEndian, w.posX)
	binary.Write(buf, binary.LittleEndian, w.posY)
	binary.Write(buf, binary.LittleEndian, w.posZ)
	binary.Write(buf, binary.LittleEndian, w.orientation)
	// fallTime
	binary.Write(buf, binary.LittleEndian, uint32(0))
	// Jump data (present because MOVEMENTFLAG_FALLING is set)
	binary.Write(buf, binary.LittleEndian, float32(7.96)) // zspeed
	binary.Write(buf, binary.LittleEndian, float32(0.0))  // sinAngle
	binary.Write(buf, binary.LittleEndian, float32(1.0))  // cosAngle
	binary.Write(buf, binary.LittleEndian, float32(0.0))  // xyspeed
	return w.sendPacket(MsgMoveJump, buf.Bytes())
}

// SendHeartbeat sends a movement heartbeat (stationary, no movement flags)
func (w *WorldClient) SendHeartbeat() error {
	w.currentMoveFlags = 0
	ts := getMSTime()
	buf := new(bytes.Buffer)
	if len(w.packedGUID) > 0 {
		buf.Write(w.packedGUID)
	} else {
		writePackedGUID(buf, w.charGUID)
	}
	binary.Write(buf, binary.LittleEndian, uint32(0)) // movementFlags: none
	binary.Write(buf, binary.LittleEndian, uint16(0)) // movementFlags2
	binary.Write(buf, binary.LittleEndian, ts)
	binary.Write(buf, binary.LittleEndian, w.posX)
	binary.Write(buf, binary.LittleEndian, w.posY)
	binary.Write(buf, binary.LittleEndian, w.posZ)
	binary.Write(buf, binary.LittleEndian, w.orientation)
	binary.Write(buf, binary.LittleEndian, uint32(0)) // fallTime
	pkt := buf.Bytes()
	return w.sendPacket(MsgMoveHeartbeat, pkt)
}

// SendMovementHeartbeat sends a movement heartbeat while moving (with forward flag)
func (w *WorldClient) SendMovementHeartbeat() error {
	w.currentMoveFlags = MoveFlagForward
	ts := getMSTime()
	buf := new(bytes.Buffer)
	if len(w.packedGUID) > 0 {
		buf.Write(w.packedGUID)
	} else {
		writePackedGUID(buf, w.charGUID)
	}
	binary.Write(buf, binary.LittleEndian, uint32(MoveFlagForward)) // movementFlags: forward
	binary.Write(buf, binary.LittleEndian, uint16(0))               // movementFlags2
	binary.Write(buf, binary.LittleEndian, ts)
	binary.Write(buf, binary.LittleEndian, w.posX)
	binary.Write(buf, binary.LittleEndian, w.posY)
	binary.Write(buf, binary.LittleEndian, w.posZ)
	binary.Write(buf, binary.LittleEndian, w.orientation)
	binary.Write(buf, binary.LittleEndian, uint32(0)) // fallTime
	pkt := buf.Bytes()
	//w.log("[MOV] send 0x%04X flags=0x%08X ts=%d pos=(%.3f,%.3f,%.3f) o=%.3f", MsgMoveHeartbeat, MoveFlagForward, ts, w.posX, w.posY, w.posZ, w.orientation)
	return w.sendPacket(MsgMoveHeartbeat, pkt)
}

// SendMovementHeartbeatWithJump sends a forward heartbeat that also has MOVEMENTFLAG_FALLING
// and includes the jump/fall trajectory data (zspeed, sin/cos angle, xyspeed).
// This matches packets seen in manual movement logs for the initial move while a small drop/fall is occurring.
func (w *WorldClient) SendMovementHeartbeatWithJump(fallTime uint32, zspeed, sinAngle, cosAngle, xyspeed float32) error {
	flags := MoveFlagForward | MoveFlagFalling
	w.currentMoveFlags = flags
	ts := getMSTime()
	buf := new(bytes.Buffer)
	if len(w.packedGUID) > 0 {
		buf.Write(w.packedGUID)
	} else {
		writePackedGUID(buf, w.charGUID)
	}
	binary.Write(buf, binary.LittleEndian, uint32(flags))
	binary.Write(buf, binary.LittleEndian, uint16(0)) // flags2
	binary.Write(buf, binary.LittleEndian, ts)
	binary.Write(buf, binary.LittleEndian, w.posX)
	binary.Write(buf, binary.LittleEndian, w.posY)
	binary.Write(buf, binary.LittleEndian, w.posZ)
	binary.Write(buf, binary.LittleEndian, w.orientation)
	binary.Write(buf, binary.LittleEndian, fallTime)
	// Jump / falling data (present because MOVEMENTFLAG_FALLING is set)
	binary.Write(buf, binary.LittleEndian, zspeed)
	binary.Write(buf, binary.LittleEndian, sinAngle)
	binary.Write(buf, binary.LittleEndian, cosAngle)
	binary.Write(buf, binary.LittleEndian, xyspeed)
	return w.sendPacket(MsgMoveHeartbeat, buf.Bytes())
}

// writeMovementBody writes the movement info body (flags + pos etc) without leading packed GUID.
// Used for ACK packets which have their own prefix (guid + counter).
func (w *WorldClient) writeMovementBody(buf *bytes.Buffer, moveFlags uint32) {
	binary.Write(buf, binary.LittleEndian, moveFlags)
	binary.Write(buf, binary.LittleEndian, uint16(0)) // flags2
	binary.Write(buf, binary.LittleEndian, uint32(getMSTime()))
	binary.Write(buf, binary.LittleEndian, w.posX)
	binary.Write(buf, binary.LittleEndian, w.posY)
	binary.Write(buf, binary.LittleEndian, w.posZ)
	binary.Write(buf, binary.LittleEndian, w.orientation)
	binary.Write(buf, binary.LittleEndian, uint32(0)) // fallTime
	// For basic ground movement we omit transport/pitch/jump/spline extras.
}

func (w *WorldClient) sendForceSpeedAck(ackOpcode uint16, counter uint32, speed float32) {
	buf := new(bytes.Buffer)
	writePackedGUID(buf, w.charGUID)
	binary.Write(buf, binary.LittleEndian, counter)
	w.writeMovementBody(buf, w.currentMoveFlags)
	binary.Write(buf, binary.LittleEndian, speed)
	w.sendPacket(ackOpcode, buf.Bytes())
}

func (w *WorldClient) handleForceSpeedChange(data []byte) {
	if len(data) < 10 {
		return
	}
	r := bytes.NewReader(data)
	_, err := readPackedGUID(r)
	if err != nil {
		return
	}
	var counter uint32
	binary.Read(r, binary.LittleEndian, &counter)

	// For RUN there is an extra uint8(0) before the float.
	// Read enough for u8 + f32 or just f32.
	var b [5]byte
	if n, err := io.ReadFull(r, b[:]); err == nil && n >= 5 {
		newspeed := math.Float32frombits(binary.LittleEndian.Uint32(b[1:5]))
		w.moveSpeed = newspeed
		w.log("Force speed change: speed=%.4f counter=%d", newspeed, counter)
		w.sendForceSpeedAck(CmsgForceRunSpeedChangeAck, counter, newspeed)
	} else if n, err := io.ReadFull(r, b[:4]); err == nil && n >= 4 {
		newspeed := math.Float32frombits(binary.LittleEndian.Uint32(b[:4]))
		w.moveSpeed = newspeed
		w.log("Force speed change: speed=%.4f counter=%d", newspeed, counter)
		w.sendForceSpeedAck(CmsgForceRunSpeedChangeAck, counter, newspeed)
	}
}

// SendPing sends CMSG_PING
func (w *WorldClient) SendPing(seq uint32) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, seq)
	binary.Write(buf, binary.LittleEndian, uint32(0)) // latency
	return w.sendPacket(CmsgPing, buf.Bytes())
}

// SendLogout sends CMSG_LOGOUT_REQUEST
func (w *WorldClient) SendLogout() error {
	return w.sendPacket(CmsgLogoutRequest, nil)
}

// WaitForLogout waits for the logout to complete
func (w *WorldClient) WaitForLogout(timeout time.Duration) error {
	select {
	case <-w.logoutDone:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("logout timeout")
	case <-w.stopChan:
		return w.lastError
	}
}

// CompleteCinematic sends CMSG_COMPLETE_CINEMATIC
func (w *WorldClient) CompleteCinematic() error {
	return w.sendPacket(CmsgCompleteCinematic, nil)
}

// NextCinematicCamera sends CMSG_NEXT_CINEMATIC_CAMERA
func (w *WorldClient) NextCinematicCamera() error {
	return w.sendPacket(CmsgNextCinematicCamera, nil)
}

// RepopRequest sends CMSG_REPOP_REQUEST (release spirit after death)
func (w *WorldClient) RepopRequest() error {
	w.log("Releasing spirit (CMSG_REPOP_REQUEST)")
	return w.sendPacket(CmsgRepopRequest, []byte{0}) // 1 byte expected by server
}

// ReclaimCorpse sends CMSG_RECLAIM_CORPSE (resurrect at corpse)
func (w *WorldClient) ReclaimCorpse() error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, w.charGUID)
	w.log("Reclaiming corpse (CMSG_RECLAIM_CORPSE)")
	return w.sendPacket(CmsgReclaimCorpse, buf.Bytes())
}

// Close closes the connection and unblocks any pending reads.
func (w *WorldClient) Close() {
	w.sendMu.Lock()
	defer w.sendMu.Unlock()
	w.stopped = true
	if w.conn != nil {
		// Closing the conn will unblock any ReadFull in readPacket/readLoop.
		_ = w.conn.Close()
	}
}

// StopChan returns the channel that is closed when the connection stops
func (w *WorldClient) StopChan() <-chan struct{} {
	return w.stopChan
}

// Stop requests the client to stop. It also closes the underlying connection
// so that a blocked readPacket/readLoop unblocks promptly.
func (w *WorldClient) Stop() {
	w.stopped = true
	w.Close()
}

func (w *WorldClient) log(format string, args ...interface{}) {
	if w.logFunc != nil {
		w.logFunc(format, args...)
	}
}

func getMSTime() uint32 {
	return uint32(time.Now().UnixMilli() & 0xFFFFFFFF)
}

func crandRead(b []byte) {
	rand.Read(b)
}

// writePackedGUID writes a packed GUID to the buffer
func writePackedGUID(buf *bytes.Buffer, guid uint64) {
	packGUID := make([]byte, 9)
	packGUID[0] = 0
	size := 1

	for i := uint8(0); guid != 0; i++ {
		if guid&0xFF != 0 {
			packGUID[0] |= 1 << i
			packGUID[size] = byte(guid & 0xFF)
			size++
		}
		guid >>= 8
	}

	buf.Write(packGUID[:size])
}

// readPackedGUID reads a packed GUID from a reader
func readPackedGUID(r io.Reader) (uint64, error) {
	var maskBuf [1]byte
	if _, err := io.ReadFull(r, maskBuf[:]); err != nil {
		return 0, err
	}
	mask := maskBuf[0]
	if mask == 0 {
		return 0, nil
	}

	var guid uint64
	var b [1]byte
	for i := uint8(0); i < 8; i++ {
		if mask&(1<<i) != 0 {
			if _, err := io.ReadFull(r, b[:]); err != nil {
				return 0, err
			}
			guid |= uint64(b[0]) << (i * 8)
		}
	}
	return guid, nil
}

// ============================================================
// Combat methods
// ============================================================

// AttackSwing sends CMSG_ATTACKSWING to start melee attacking a target
func (w *WorldClient) AttackSwing(targetGUID uint64) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, targetGUID)
	w.combatMu.Lock()
	w.attackingGUID = targetGUID
	w.combatMu.Unlock()
	return w.sendPacket(CmsgAttackSwing, buf.Bytes())
}

// AttackStop sends CMSG_ATTACKSTOP to stop attacking
func (w *WorldClient) AttackStop() error {
	w.combatMu.Lock()
	w.attackingGUID = 0
	w.inCombat = false
	w.combatMu.Unlock()
	return w.sendPacket(CmsgAttackStop, nil)
}

// SetTarget sends CMSG_SET_SELECTION to set the current target
func (w *WorldClient) SetTarget(targetGUID uint64) error {
	w.combatMu.Lock()
	w.targetGUID = targetGUID
	w.combatMu.Unlock()
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, targetGUID)
	return w.sendPacket(CmsgSetSelection, buf.Bytes())
}

// CastSpell sends CMSG_CAST_SPELL for the given spell targeting a unit
func (w *WorldClient) CastSpell(spellID uint32, targetGUID uint64) error {
	buf := new(bytes.Buffer)
	buf.WriteByte(0) // castCount
	binary.Write(buf, binary.LittleEndian, spellID)
	buf.WriteByte(0) // castFlags

	// Target flags: unit target
	if targetGUID != 0 {
		binary.Write(buf, binary.LittleEndian, uint32(0x0002)) // TARGET_FLAG_UNIT
		writePackedGUID(buf, targetGUID)
	} else {
		binary.Write(buf, binary.LittleEndian, uint32(0x0000)) // TARGET_FLAG_SELF
	}

	return w.sendPacket(CmsgCastSpell, buf.Bytes())
}

// CastSpellAtPosition sends a spell targeted at a position
func (w *WorldClient) CastSpellAtPosition(spellID uint32, x, y, z float32) error {
	buf := new(bytes.Buffer)
	buf.WriteByte(0)
	binary.Write(buf, binary.LittleEndian, spellID)
	buf.WriteByte(0)
	binary.Write(buf, binary.LittleEndian, uint32(0x0020)) // TARGET_FLAG_DEST_LOCATION
	binary.Write(buf, binary.LittleEndian, x)
	binary.Write(buf, binary.LittleEndian, y)
	binary.Write(buf, binary.LittleEndian, z)
	return w.sendPacket(CmsgCastSpell, buf.Bytes())
}

// Loot sends CMSG_LOOT to loot a unit/object
func (w *WorldClient) Loot(guid uint64) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, guid)
	return w.sendPacket(CmsgLoot, buf.Bytes())
}

// LootMoney sends CMSG_LOOT_MONEY to collect money from loot
func (w *WorldClient) LootMoney() error {
	return w.sendPacket(CmsgLootMoney, nil)
}

// LootItem sends CMSG_AUTOSTORE_LOOT_ITEM to take a loot item by slot
func (w *WorldClient) LootItem(slot uint8) error {
	buf := new(bytes.Buffer)
	buf.WriteByte(slot)
	return w.sendPacket(CmsgAutostoreLootItem, buf.Bytes())
}

// LootRelease sends CMSG_LOOT_RELEASE to close the loot window
func (w *WorldClient) LootRelease(guid uint64) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, guid)
	return w.sendPacket(CmsgLootRelease, buf.Bytes())
}

// SendGMCommand sends a chat message that is a GM command (e.g., ".gm on")
func (w *WorldClient) SendGMCommand(command string) error {
	w.log("Sending GM command: %s (opcode=0x%04X encrypted=%v)", command, CmsgMessageChat, w.encrypted)
	err := w.SendChatMessage(ChatMsgSay, LangCommon, command)
	if err != nil {
		w.log("GM command send error: %v", err)
	}
	return err
}

// GroupInvite sends CMSG_GROUP_INVITE to invite a player by name
func (w *WorldClient) GroupInvite(playerName string) error {
	buf := new(bytes.Buffer)
	buf.Write(append([]byte(playerName), 0))
	return w.sendPacket(CmsgGroupInvite, buf.Bytes())
}

// GroupAccept sends CMSG_GROUP_ACCEPT to accept a group invitation
func (w *WorldClient) GroupAccept() error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint32(0))
	return w.sendPacket(CmsgGroupAccept, buf.Bytes())
}

// NameQuery sends CMSG_NAME_QUERY to look up a player name
func (w *WorldClient) NameQuery(guid uint64) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, guid)
	return w.sendPacket(CmsgNameQuery, buf.Bytes())
}

// CreatureQuery sends CMSG_CREATURE_QUERY to look up a creature name
func (w *WorldClient) CreatureQuery(entry uint32, guid uint64) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, entry)
	binary.Write(buf, binary.LittleEndian, guid)
	return w.sendPacket(CmsgCreatureQuery, buf.Bytes())
}

// SetSheathed sets sheath state: 0=unsheathed, 1=melee, 2=ranged
func (w *WorldClient) SetSheathed(state uint32) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, state)
	return w.sendPacket(CmsgSetSheathed, buf.Bytes())
}

// QuestgiverHello sends CMSG_QUESTGIVER_HELLO
func (w *WorldClient) QuestgiverHello(npcGUID uint64) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, npcGUID)
	return w.sendPacket(CmsgQuestgiverHello, buf.Bytes())
}

// QuestgiverAcceptQuest sends CMSG_QUESTGIVER_ACCEPT_QUEST
func (w *WorldClient) QuestgiverAcceptQuest(npcGUID uint64, questID uint32) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, npcGUID)
	binary.Write(buf, binary.LittleEndian, questID)
	binary.Write(buf, binary.LittleEndian, uint32(0))
	return w.sendPacket(CmsgQuestgiverAcceptQuest, buf.Bytes())
}

// QuestgiverCompleteQuest sends CMSG_QUESTGIVER_COMPLETE_QUEST
func (w *WorldClient) QuestgiverCompleteQuest(npcGUID uint64, questID uint32) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, npcGUID)
	binary.Write(buf, binary.LittleEndian, questID)
	return w.sendPacket(CmsgQuestgiverCompleteQuest, buf.Bytes())
}

// QuestgiverChooseReward sends CMSG_QUESTGIVER_CHOOSE_REWARD
func (w *WorldClient) QuestgiverChooseReward(npcGUID uint64, questID uint32, reward uint32) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, npcGUID)
	binary.Write(buf, binary.LittleEndian, questID)
	binary.Write(buf, binary.LittleEndian, reward)
	return w.sendPacket(CmsgQuestgiverChooseReward, buf.Bytes())
}

// ============================================================
// Movement methods
// ============================================================

// MoveForward starts forward movement toward a destination.
// The caller is responsible for sending MsgMoveStop when arriving.
func (w *WorldClient) MoveForward() error {
	return w.sendMovement(MsgMoveStartForward, MoveFlagForward)
}

// MoveStop stops all movement
func (w *WorldClient) MoveStop() error {
	return w.sendMovement(MsgMoveStop, MoveFlagNone)
}

// SetFacing sets the character's facing orientation
func (w *WorldClient) SetFacing(orientation float32) error {
	w.orientation = orientation
	return w.sendMovement(MsgMoveSetFacing, MoveFlagNone)
}

// SetFacingMoving sets facing while moving (includes forward flag, like real client)
func (w *WorldClient) SetFacingMoving(orientation float32) error {
	w.orientation = orientation
	return w.sendMovement(MsgMoveSetFacing, MoveFlagForward)
}

func (w *WorldClient) sendMovement(opcode uint16, moveFlags uint32) error {
	w.currentMoveFlags = moveFlags
	ts := getMSTime()
	buf := new(bytes.Buffer)
	if len(w.packedGUID) > 0 {
		buf.Write(w.packedGUID)
	} else {
		writePackedGUID(buf, w.charGUID)
	}
	binary.Write(buf, binary.LittleEndian, moveFlags)
	binary.Write(buf, binary.LittleEndian, uint16(0)) // movementFlags2
	binary.Write(buf, binary.LittleEndian, ts)
	binary.Write(buf, binary.LittleEndian, w.posX)
	binary.Write(buf, binary.LittleEndian, w.posY)
	binary.Write(buf, binary.LittleEndian, w.posZ)
	binary.Write(buf, binary.LittleEndian, w.orientation)
	binary.Write(buf, binary.LittleEndian, uint32(0)) // fallTime
	pkt := buf.Bytes()
	return w.sendPacket(opcode, pkt)
}

// UpdatePosition locally updates the stored position and sends a heartbeat.
func (w *WorldClient) UpdatePosition(x, y, z, o float32) {
	w.posX = x
	w.posY = y
	w.posZ = z
	w.orientation = o
}

// Position returns the current position of the character.
func (w *WorldClient) Position() (x, y, z, o float32, mapID uint32) {
	return w.posX, w.posY, w.posZ, w.orientation, w.mapID
}

// CharGUID returns the GUID of the logged-in character.
func (w *WorldClient) CharGUID() uint64 {
	return w.charGUID
}

// ============================================================
// Query methods
// ============================================================

// GetObject returns a tracked object by GUID, or nil if not found.
// It returns a clone so the caller gets its own private snapshot of the world state.
func (w *WorldClient) GetObject(guid uint64) *WorldObject {
	w.objectsMu.RLock()
	obj := w.objects[guid]
	w.objectsMu.RUnlock()
	if obj == nil {
		return nil
	}
	return obj.Clone()
}

// GetNearbyUnits returns all tracked units (NPCs) within maxDist yards.
// Returns clones so each bot/AI gets its own private version of the world state
// (helps with races and different layers/views per connection).
func (w *WorldClient) GetNearbyUnits(maxDist float32) []*WorldObject {
	w.objectsMu.RLock()
	var raw []*WorldObject
	for _, obj := range w.objects {
		if obj.TypeID == ObjectTypeUnit && obj.DistanceTo(w.posX, w.posY, w.posZ) <= maxDist {
			raw = append(raw, obj)
		}
	}
	w.objectsMu.RUnlock()

	result := make([]*WorldObject, len(raw))
	for i, obj := range raw {
		result[i] = obj.Clone()
	}
	return result
}

// GetNearbyPlayers returns all tracked players within maxDist yards.
// Returns clones for private per-bot world view.
func (w *WorldClient) GetNearbyPlayers(maxDist float32) []*WorldObject {
	w.objectsMu.RLock()
	var raw []*WorldObject
	for _, obj := range w.objects {
		if obj.TypeID == ObjectTypePlayer && obj.GUID != w.charGUID && obj.DistanceTo(w.posX, w.posY, w.posZ) <= maxDist {
			raw = append(raw, obj)
		}
	}
	w.objectsMu.RUnlock()

	result := make([]*WorldObject, len(raw))
	for i, obj := range raw {
		result[i] = obj.Clone()
	}
	return result
}

// InCombat returns whether the bot thinks it is in combat.
func (w *WorldClient) InCombat() bool {
	w.combatMu.RLock()
	defer w.combatMu.RUnlock()
	return w.inCombat
}

// TargetGUID returns the current target GUID.
func (w *WorldClient) TargetGUID() uint64 {
	w.combatMu.RLock()
	defer w.combatMu.RUnlock()
	return w.targetGUID
}

// ClearTarget clears the current target selection.
func (w *WorldClient) ClearTarget() {
	w.combatMu.Lock()
	w.targetGUID = 0
	w.combatMu.Unlock()
}

// ClearCombat clears the combat state.
func (w *WorldClient) ClearCombat() {
	w.combatMu.Lock()
	w.inCombat = false
	w.attackingGUID = 0
	w.combatMu.Unlock()
}

// MarkObjectDead forces health=0 for the given GUID in the object cache.
// Used on kill prediction/confirmation so IsAlive() and BT decisions see death immediately
// instead of waiting for the next values update (prevents chasing/looking at dead).
func (w *WorldClient) MarkObjectDead(guid uint64) {
	w.objectsMu.Lock()
	if obj := w.objects[guid]; obj != nil {
		obj.setValue(UnitFieldHealth, 0)
		obj.setValue(UnitFieldFlags, obj.value(UnitFieldFlags)|UnitFlagDead)
		obj.setValue(UnitDynamicFlags, obj.value(UnitDynamicFlags)|UnitDynflagDead)
	}
	w.objectsMu.Unlock()
}

// ApplyLocalDamage applies damage seen in AttackerStateUpdate packets to the local object cache.
// This helps when the server doesn't promptly send UNIT_FIELD_HEALTH=0 updates for mobs killed
// by other players/bots (common in multi-bot scenarios). We locally simulate the health reduction
// so IsAlive() and targeting see death faster, avoiding the "8/55 on dead mob" problem.
func (w *WorldClient) ApplyLocalDamage(guid uint64, damage uint32) {
	if damage == 0 {
		return
	}
	w.objectsMu.Lock()
	if obj := w.objects[guid]; obj != nil {
		h := obj.value(UnitFieldHealth)
		if h > damage {
			obj.setValue(UnitFieldHealth, h-damage)
		} else {
			obj.setValue(UnitFieldHealth, 0)
			obj.setValue(UnitFieldFlags, obj.value(UnitFieldFlags)|UnitFlagDead)
			obj.setValue(UnitDynamicFlags, obj.value(UnitDynamicFlags)|UnitDynflagDead)
		}
	}
	w.objectsMu.Unlock()
}

// Health returns current player health.
func (w *WorldClient) Health() uint32 {
	w.statsMu.RLock()
	defer w.statsMu.RUnlock()
	return w.health
}

// MaxHealth returns max player health.
func (w *WorldClient) MaxHealth() uint32 {
	w.statsMu.RLock()
	defer w.statsMu.RUnlock()
	return w.maxHealth
}

// PlayerLevel returns the bot's current level.
func (w *WorldClient) PlayerLevel() uint32 {
	w.statsMu.RLock()
	defer w.statsMu.RUnlock()
	return w.level
}

// IsSpellReady returns true if the spell is known and off cooldown.
func (w *WorldClient) IsSpellReady(spellID uint32) bool {
	w.spellsMu.RLock()
	sp, known := w.knownSpells[spellID]
	w.spellsMu.RUnlock()
	if !known || !sp.Active {
		return false
	}

	w.cooldownsMu.RLock()
	cd, onCooldown := w.cooldowns[spellID]
	w.cooldownsMu.RUnlock()
	if onCooldown && time.Now().Before(cd.ExpiresAt) {
		return false
	}

	return true
}

// KnowsSpell returns true if the bot has learned the given spell.
func (w *WorldClient) KnowsSpell(spellID uint32) bool {
	w.spellsMu.RLock()
	defer w.spellsMu.RUnlock()
	sp, ok := w.knownSpells[spellID]
	return ok && sp.Active
}

// MoveSpeed returns the current movement speed.
func (w *WorldClient) MoveSpeed() float32 {
	return w.moveSpeed
}

// ============================================================
// Packet handlers for combat / spells / objects
// ============================================================

func (w *WorldClient) handleCancelCombat() {
	w.combatMu.Lock()
	w.inCombat = false
	w.combatMu.Unlock()
	if w.OnCombatStop != nil {
		w.OnCombatStop()
	}
}

func (w *WorldClient) handleAttackStart(data []byte) {
	if len(data) < 16 {
		return
	}
	attacker := binary.LittleEndian.Uint64(data[0:8])
	victim := binary.LittleEndian.Uint64(data[8:16])
	w.log("SMSG_ATTACK_START: attacker=%d victim=%d myGUID=%d", attacker, victim, w.charGUID)
	if attacker == w.charGUID || victim == w.charGUID {
		w.combatMu.Lock()
		w.inCombat = true
		if victim == w.charGUID && w.targetGUID == 0 {
			w.targetGUID = attacker
		}
		w.combatMu.Unlock()
	}
	if w.OnCombatStart != nil {
		w.OnCombatStart(attacker, victim)
	}
}

func (w *WorldClient) handleAttackStop(data []byte) {
	if len(data) < 4 {
		return
	}
	r := bytes.NewReader(data)
	attackerGUID, _ := readPackedGUID(r)
	victimGUID, _ := readPackedGUID(r)

	if attackerGUID == w.charGUID {
		w.combatMu.Lock()
		w.attackingGUID = 0
		w.combatMu.Unlock()

		// Server told us to stop attacking this victim. This is authoritative feedback
		// that the target is no longer valid (dead, friendly, out of range, etc.).
		// Mark it invalid in our cache immediately and clear if it was our target.
		if victimGUID != 0 {
			w.MarkObjectDead(victimGUID)
			if w.targetGUID == victimGUID {
				w.ClearTarget()
				w.ClearCombat()
			}
			w.combatMu.Lock()
			w.inCombat = false
			w.combatMu.Unlock()
			if w.OnInvalidTarget != nil {
				w.OnInvalidTarget(victimGUID)
			}
		}
	}

	// Also mark if we see stop for our current target (even if not attacker? rare but safe)
	if victimGUID != 0 && w.targetGUID == victimGUID {
		w.MarkObjectDead(victimGUID)
	}
}

func (w *WorldClient) handleAttackerStateUpdate(data []byte) {
	if len(data) < 20 {
		return
	}
	r := bytes.NewReader(data)
	var hitInfo uint32
	binary.Read(r, binary.LittleEndian, &hitInfo)
	attackerGUID, _ := readPackedGUID(r)
	victimGUID, _ := readPackedGUID(r)
	var totalDamage uint32
	binary.Read(r, binary.LittleEndian, &totalDamage)

	// Track our outgoing damage and check for kills (pre-damage view)
	if attackerGUID == w.charGUID && totalDamage > 0 {
		victim := w.GetObject(victimGUID)
		if victim != nil {
			h := victim.Health()
			w.log("Dealt %d damage to GUID %d Entry=%d (HP: %d)", totalDamage, victimGUID, victim.Entry, h)
			if h > 0 && totalDamage >= h {
				w.log("Killed target GUID %d Entry=%d", victimGUID, victim.Entry)
				if w.OnKill != nil {
					w.OnKill(victimGUID)
				}
			}
		} else {
			w.log("Dealt %d damage to GUID %d (no object data)", totalDamage, victimGUID)
		}
	}

	// Apply seen damage locally to the victim's object cache *after* our prediction.
	// Critical for multi-bot: when others kill a mob, server may not send health=0 promptly
	// to this connection. Local damage application makes our IsAlive()/targeting see 0 health.
	if victimGUID != 0 && totalDamage > 0 {
		w.ApplyLocalDamage(victimGUID, totalDamage)
	}

	// If local application brought health to 0, force dead mark on cache so IsAlive sees it immediately.
	if victimGUID != 0 {
		v := w.GetObject(victimGUID)
		if v != nil && v.Health() == 0 {
			w.MarkObjectDead(victimGUID)
		}
	}

	// Check if we are the victim - set combat state and track damage
	if victimGUID == w.charGUID && totalDamage > 0 {
		// Always set combat flag when taking damage
		w.combatMu.Lock()
		w.inCombat = true
		if w.targetGUID == 0 {
			w.targetGUID = attackerGUID
		}
		w.combatMu.Unlock()

		w.statsMu.Lock()
		w.log("Took %d damage from attacker GUID %d (HP: %d/%d)", totalDamage, attackerGUID, w.health, w.maxHealth)
		if w.health > 0 && totalDamage >= w.health {
			w.health = 0
			w.statsMu.Unlock()
			w.log("Bot has died! (killed by GUID %d)", attackerGUID)
			if w.OnDeath != nil {
				w.OnDeath()
			}
		} else if w.health > totalDamage {
			w.health -= totalDamage
			w.statsMu.Unlock()
		} else {
			w.statsMu.Unlock()
		}
	}
}

func (w *WorldClient) handleInitialSpells(data []byte) {
	if len(data) < 3 {
		return
	}
	r := bytes.NewReader(data)
	r.ReadByte() // talentSpec
	var spellCount uint16
	binary.Read(r, binary.LittleEndian, &spellCount)

	w.spellsMu.Lock()
	for i := uint16(0); i < spellCount; i++ {
		var spellID uint32
		binary.Read(r, binary.LittleEndian, &spellID)
		var unk uint16
		binary.Read(r, binary.LittleEndian, &unk) // slot index or flags

		w.knownSpells[spellID] = &KnownSpell{SpellID: spellID, Active: unk == 0}
	}
	w.spellsMu.Unlock()
	w.log("Received %d initial spells", spellCount)
}

func (w *WorldClient) handleSpellGo(data []byte) {
	if len(data) < 4 {
		return
	}
	r := bytes.NewReader(data)
	casterGUID, _ := readPackedGUID(r)
	_, _ = readPackedGUID(r) // casterUnit
	var castID uint8
	binary.Read(r, binary.LittleEndian, &castID)
	var spellID uint32
	binary.Read(r, binary.LittleEndian, &spellID)

	if casterGUID == w.charGUID {
		if w.OnSpellCastResult != nil {
			w.OnSpellCastResult(spellID, true)
		}
	}
}

func (w *WorldClient) handleSpellFailure(data []byte) {
	if len(data) < 10 {
		return
	}
	r := bytes.NewReader(data)
	casterGUID, _ := readPackedGUID(r)
	var castID uint8
	binary.Read(r, binary.LittleEndian, &castID)
	var spellID uint32
	binary.Read(r, binary.LittleEndian, &spellID)
	var reason uint8
	binary.Read(r, binary.LittleEndian, &reason)

	if casterGUID == w.charGUID {
		w.log("Spell %d FAILED (reason=%d)", spellID, reason)
		if w.OnSpellCastResult != nil {
			w.OnSpellCastResult(spellID, false)
		}
	}
}

func (w *WorldClient) handleSpellCooldown(data []byte) {
	if len(data) < 12 {
		return
	}
	r := bytes.NewReader(data)
	var guid uint64
	binary.Read(r, binary.LittleEndian, &guid)

	if guid != w.charGUID {
		return
	}

	w.cooldownsMu.Lock()
	for r.Len() >= 8 {
		var spellID uint32
		var cdTime uint32
		binary.Read(r, binary.LittleEndian, &spellID)
		binary.Read(r, binary.LittleEndian, &cdTime)

		w.cooldowns[spellID] = &SpellCooldown{
			SpellID:   spellID,
			ExpiresAt: time.Now().Add(time.Duration(cdTime) * time.Millisecond),
		}
	}
	w.cooldownsMu.Unlock()
}

func (w *WorldClient) handleCooldownEvent(data []byte) {
	if len(data) < 12 {
		return
	}
	var spellID uint32
	var guid uint64
	r := bytes.NewReader(data)
	binary.Read(r, binary.LittleEndian, &spellID)
	binary.Read(r, binary.LittleEndian, &guid)

	if guid == w.charGUID {
		w.cooldownsMu.Lock()
		delete(w.cooldowns, spellID)
		w.cooldownsMu.Unlock()
	}
}

func (w *WorldClient) handleClearCooldown(data []byte) {
	if len(data) < 12 {
		return
	}
	var spellID uint32
	var guid uint64
	r := bytes.NewReader(data)
	binary.Read(r, binary.LittleEndian, &spellID)
	binary.Read(r, binary.LittleEndian, &guid)

	if guid == w.charGUID {
		w.cooldownsMu.Lock()
		delete(w.cooldowns, spellID)
		w.cooldownsMu.Unlock()
	}
}

func (w *WorldClient) handleLootResponse(data []byte) {
	if len(data) < 14 {
		return
	}
	r := bytes.NewReader(data)
	var lootGUID uint64
	binary.Read(r, binary.LittleEndian, &lootGUID)

	var lootType uint8
	binary.Read(r, binary.LittleEndian, &lootType)
	var gold uint32
	binary.Read(r, binary.LittleEndian, &gold)
	var itemCount uint8
	binary.Read(r, binary.LittleEndian, &itemCount)

	items := make([]LootItem, 0, itemCount)
	for i := uint8(0); i < itemCount; i++ {
		item := LootItem{}
		binary.Read(r, binary.LittleEndian, &item.Index)
		binary.Read(r, binary.LittleEndian, &item.ItemID)
		binary.Read(r, binary.LittleEndian, &item.Quantity)

		// Skip: displayID(4) + randomSuffix(4) + randomPropertyID(4) + slotType(1)
		skip := make([]byte, 13)
		r.Read(skip)

		items = append(items, item)
	}

	if w.OnLootOpened != nil {
		w.OnLootOpened(lootGUID, items)
	}
}

func (w *WorldClient) handleChatMessage(data []byte) {
	if len(data) < 8 {
		return
	}
	r := bytes.NewReader(data)
	var msgType uint8
	binary.Read(r, binary.LittleEndian, &msgType)
	var lang uint32
	binary.Read(r, binary.LittleEndian, &lang)

	// Read sender GUID
	var senderGUID uint64
	binary.Read(r, binary.LittleEndian, &senderGUID)
	var unk uint32
	binary.Read(r, binary.LittleEndian, &unk)

	// For SAY, YELL, etc. the format is:
	// targetGUID(8) + msgLen(4) + msg(null-terminated) + chatTag(1)
	switch msgType {
	case 0x00, 0x01, 0x06: // SAY, PARTY, YELL
		var targetGUID uint64
		binary.Read(r, binary.LittleEndian, &targetGUID)
		var msgLen uint32
		binary.Read(r, binary.LittleEndian, &msgLen)
		if msgLen > 0 && msgLen < 4096 {
			msgBytes := make([]byte, msgLen)
			r.Read(msgBytes)
			msg := strings.TrimRight(string(msgBytes), "\x00")
			senderName := ""
			obj := w.GetObject(senderGUID)
			if obj != nil {
				senderName = obj.Name
			}
			if w.OnChatMessage != nil {
				w.OnChatMessage(senderName, msg, msgType)
			}
		}
	case 0xFF: // CHAT_MSG_SYSTEM (system messages)
		// System messages have different format
		var targetGUID uint64
		binary.Read(r, binary.LittleEndian, &targetGUID)
		var msgLen uint32
		binary.Read(r, binary.LittleEndian, &msgLen)
		if msgLen > 0 && msgLen < 4096 {
			msgBytes := make([]byte, msgLen)
			r.Read(msgBytes)
			msg := strings.TrimRight(string(msgBytes), "\x00")
			w.log("System: %s", msg)
		}
	}
}

func (w *WorldClient) handleLevelUp(data []byte) {
	if len(data) < 4 {
		return
	}
	newLevel := binary.LittleEndian.Uint32(data[0:4])
	w.statsMu.Lock()
	w.level = newLevel
	w.statsMu.Unlock()
	w.log("LEVEL UP! Now level %d", newLevel)
	if w.OnLevelUp != nil {
		w.OnLevelUp(newLevel)
	}
}

func (w *WorldClient) handlePowerUpdate(data []byte) {
	if len(data) < 12 {
		return
	}
	r := bytes.NewReader(data)
	guid, _ := readPackedGUID(r)
	if guid != w.charGUID {
		return
	}
	var powerType uint8
	binary.Read(r, binary.LittleEndian, &powerType)
	var value uint32
	binary.Read(r, binary.LittleEndian, &value)

	if powerType == 0 { // mana
		w.statsMu.Lock()
		w.power = value
		w.statsMu.Unlock()
	}
}

// ============================================================
// SMSG_UPDATE_OBJECT handling
// ============================================================

func (w *WorldClient) handleCompressedUpdateObject(data []byte) {
	if len(data) < 4 {
		return
	}
	decompressedSize := binary.LittleEndian.Uint32(data[0:4])
	if decompressedSize > 10*1024*1024 {
		return
	}

	zr, err := zlib.NewReader(bytes.NewReader(data[4:]))
	if err != nil {
		return
	}
	defer zr.Close()

	if cap(w.decompBuf) < int(decompressedSize) {
		w.decompBuf = make([]byte, decompressedSize)
	}
	decompressed := w.decompBuf[:decompressedSize]
	n, err := io.ReadFull(zr, decompressed)
	if err != nil && n == 0 {
		return
	}

	w.handleUpdateObject(decompressed[:n])
}

func (w *WorldClient) handleUpdateObject(data []byte) {
	if len(data) < 4 {
		return
	}
	r := bytes.NewReader(data)
	var blockCount uint32
	binary.Read(r, binary.LittleEndian, &blockCount)

	for i := uint32(0); i < blockCount && r.Len() > 0; i++ {
		var updateType uint8
		if err := binary.Read(r, binary.LittleEndian, &updateType); err != nil {
			return
		}

		switch updateType {
		case UpdateTypeValues:
			guid, err := readPackedGUID(r)
			if err != nil {
				return
			}
			if guid == w.charGUID {
				w.readValuesUpdate(r, guid)
			} else if w.GetObject(guid) != nil {
				// only process values updates for objects we are tracking (units/NPCs for targeting/health)
				// other players' updates are skipped to save CPU when many players visible
				w.readValuesUpdate(r, guid)
			} else {
				w.skipValuesUpdate(r)
			}

		case UpdateTypeMovement:
			// Movement only update for existing object (e.g. position/orient for units)
			guid, err := readPackedGUID(r)
			if err != nil {
				return
			}
			w.objectsMu.RLock()
			master := w.objects[guid]
			w.objectsMu.RUnlock()
			if master != nil {
				w.readMovementUpdate(r, master)
			} else {
				w.skipMovementUpdate(r)
			}

		case UpdateTypeCreateObject, UpdateTypeCreateObject2:
			guid, err := readPackedGUID(r)
			if err != nil {
				return
			}
			var objTypeID uint8
			if err := binary.Read(r, binary.LittleEndian, &objTypeID); err != nil {
				return
			}

			if objTypeID == ObjectTypePlayer && guid != w.charGUID {
				// For other players (the main source of packet volume with 500 in zone),
				// skip parsing entirely. We only need to consume bytes to stay in sync.
				// We still track self + NPCs/units for AI targeting.
				w.skipMovementUpdate(r)
				w.skipValuesUpdate(r)
			} else {
				obj := w.getOrCreateObject(guid)
				obj.TypeID = objTypeID
				obj.IsPlayer = objTypeID == ObjectTypePlayer

				w.readMovementUpdate(r, obj)
				w.readValuesUpdate(r, guid)
			}

		case UpdateTypeOutOfRangeObjects:
			var count uint32
			binary.Read(r, binary.LittleEndian, &count)
			for j := uint32(0); j < count; j++ {
				guid, err := readPackedGUID(r)
				if err != nil {
					return
				}
				w.removeObject(guid)
			}

		case UpdateTypeNearObjects:
			// list of near objects, consume guids to keep sync (format may vary)
			var count uint32
			binary.Read(r, binary.LittleEndian, &count)
			for j := uint32(0); j < count; j++ {
				_, _ = readPackedGUID(r)
			}

		default:
			// Unknown update type, bail
			return
		}
	}
}

func (w *WorldClient) readMovementUpdate(r *bytes.Reader, obj *WorldObject) {
	var updateFlags uint16
	if err := binary.Read(r, binary.LittleEndian, &updateFlags); err != nil {
		return
	}

	// UPDATEFLAG_LIVING = 0x20
	if updateFlags&0x20 != 0 {
		var moveFlags uint32
		binary.Read(r, binary.LittleEndian, &moveFlags)
		var moveFlags2 uint16
		binary.Read(r, binary.LittleEndian, &moveFlags2)
		var timestamp uint32
		binary.Read(r, binary.LittleEndian, &timestamp)

		binary.Read(r, binary.LittleEndian, &obj.PosX)
		binary.Read(r, binary.LittleEndian, &obj.PosY)
		binary.Read(r, binary.LittleEndian, &obj.PosZ)
		binary.Read(r, binary.LittleEndian, &obj.Orientation)

		obj.LastPosUpdate = time.Now()
		obj.LastSeen = time.Now()

		// Transport GUID (MOVEMENTFLAG_ONTRANSPORT = 0x00000200)
		if moveFlags&0x00000200 != 0 {
			tGUID, _ := readPackedGUID(r)
			_ = tGUID
			// transX(4) + transY(4) + transZ(4) + transO(4) + transTime(4) + transSeat(1)
			r.Seek(21, io.SeekCurrent)
			// MOVEMENTFLAG2_INTERPOLATED_MOVEMENT = 0x0400
			if moveFlags2&0x0400 != 0 {
				var extraTime uint32
				binary.Read(r, binary.LittleEndian, &extraTime)
			}
		}

		// MOVEMENTFLAG_SWIMMING (0x00200000) or MOVEMENTFLAG_FLYING (0x02000000) or MOVEMENTFLAG2_ALWAYS_ALLOW_PITCHING (0x0020)
		if moveFlags&(0x00200000|0x02000000) != 0 || moveFlags2&0x0020 != 0 {
			var pitch float32
			binary.Read(r, binary.LittleEndian, &pitch)
		}

		var fallTime uint32
		binary.Read(r, binary.LittleEndian, &fallTime)

		// MOVEMENTFLAG_FALLING (0x00001000)
		if moveFlags&0x00001000 != 0 {
			r.Seek(16, io.SeekCurrent) // zSpeed(4) + sinAngle(4) + cosAngle(4) + xySpeed(4)
		}

		// MOVEMENTFLAG_SPLINE_ELEVATION
		if moveFlags&0x04000000 != 0 {
			var splineElev float32
			binary.Read(r, binary.LittleEndian, &splineElev)
		}

		// Movement speeds: walk, run, runBack, swim, swimBack, flight, flightBack, turnRate, pitchRate
		if obj.GUID == w.charGUID {
			speeds := make([]float32, 9)
			for si := 0; si < 9; si++ {
				binary.Read(r, binary.LittleEndian, &speeds[si])
			}
			w.moveSpeed = speeds[1] // run speed
		} else {
			r.Seek(36, io.SeekCurrent)
		}

		// Spline data (MOVEMENTFLAG_SPLINE_ENABLED = 0x08000000)
		if moveFlags&0x08000000 != 0 {
			if obj.TypeID == ObjectTypeUnit && obj.GUID != w.charGUID {
				w.parseCreateSplineData(r, obj)
			} else {
				w.skipSplineData(r)
			}
		}
	} else if updateFlags&0x0100 != 0 {
		// UPDATEFLAG_POSITION (0x0100) - non-living objects with position updates
		tGUID, _ := readPackedGUID(r) // transport GUID (packed, may be just 0x00)
		_ = tGUID
		binary.Read(r, binary.LittleEndian, &obj.PosX)
		binary.Read(r, binary.LittleEndian, &obj.PosY)
		binary.Read(r, binary.LittleEndian, &obj.PosZ)
		obj.LastPosUpdate = time.Now()
		obj.LastSeen = time.Now()
		// transport offsets (or duplicate of position if no transport)
		var tx, ty, tz float32
		binary.Read(r, binary.LittleEndian, &tx)
		binary.Read(r, binary.LittleEndian, &ty)
		binary.Read(r, binary.LittleEndian, &tz)
		binary.Read(r, binary.LittleEndian, &obj.Orientation)
		var facing float32
		binary.Read(r, binary.LittleEndian, &facing)
	} else if updateFlags&0x0040 != 0 {
		// UPDATEFLAG_STATIONARY_POSITION (0x0040)
		binary.Read(r, binary.LittleEndian, &obj.PosX)
		binary.Read(r, binary.LittleEndian, &obj.PosY)
		binary.Read(r, binary.LittleEndian, &obj.PosZ)
		obj.LastPosUpdate = time.Now()
		obj.LastSeen = time.Now()
		binary.Read(r, binary.LittleEndian, &obj.Orientation)
	}

	// UPDATEFLAG_UNKNOWN (0x0008)
	if updateFlags&0x0008 != 0 {
		var unk uint32
		binary.Read(r, binary.LittleEndian, &unk)
	}

	// UPDATEFLAG_LOWGUID (0x0010)
	if updateFlags&0x0010 != 0 {
		var lowGUID uint32
		binary.Read(r, binary.LittleEndian, &lowGUID)
	}

	// UPDATEFLAG_HAS_TARGET (0x0004)
	if updateFlags&0x0004 != 0 {
		_, _ = readPackedGUID(r)
	}

	// UPDATEFLAG_TRANSPORT (0x0002)
	if updateFlags&0x0002 != 0 {
		var transportTime uint32
		binary.Read(r, binary.LittleEndian, &transportTime)
	}

	// UPDATEFLAG_VEHICLE (0x0080)
	if updateFlags&0x0080 != 0 {
		var vehicleID uint32
		binary.Read(r, binary.LittleEndian, &vehicleID)
		var vehicleOrient float32
		binary.Read(r, binary.LittleEndian, &vehicleOrient)
	}

	// UPDATEFLAG_ROTATION (0x0200) for gameobjects
	if updateFlags&0x0200 != 0 {
		var rotation int64
		binary.Read(r, binary.LittleEndian, &rotation)
	}
}

func (w *WorldClient) skipSplineData(r *bytes.Reader) {
	var splineFlags uint32
	binary.Read(r, binary.LittleEndian, &splineFlags)

	// SPLINEFLAG_FINAL_ANGLE = 0x00040000
	if splineFlags&0x00040000 != 0 {
		var angle float32
		binary.Read(r, binary.LittleEndian, &angle)
	} else if splineFlags&0x00020000 != 0 {
		// SPLINEFLAG_FINAL_TARGET
		var targetGUID uint64
		binary.Read(r, binary.LittleEndian, &targetGUID)
	} else if splineFlags&0x00010000 != 0 {
		// SPLINEFLAG_FINAL_POINT
		r.Seek(12, io.SeekCurrent)
	}

	var timePassed uint32
	binary.Read(r, binary.LittleEndian, &timePassed)
	var duration uint32
	binary.Read(r, binary.LittleEndian, &duration)
	var splineID uint32
	binary.Read(r, binary.LittleEndian, &splineID)

	// 3.3.3 additions
	var unk1 float32
	binary.Read(r, binary.LittleEndian, &unk1)
	var unk2 float32
	binary.Read(r, binary.LittleEndian, &unk2)
	var unk3 uint32
	binary.Read(r, binary.LittleEndian, &unk3)
	var unk4 uint32
	binary.Read(r, binary.LittleEndian, &unk4)

	var splineCount uint32
	binary.Read(r, binary.LittleEndian, &splineCount)

	// Read waypoints - use seek to avoid allocs
	r.Seek(int64(splineCount)*12, io.SeekCurrent)

	// Spline mode
	var splineMode uint8
	binary.Read(r, binary.LittleEndian, &splineMode)

	// Final point
	r.Seek(12, io.SeekCurrent)
}

// parseCreateSplineData parses the spline data from UPDATE_OBJECT create movement block for units
// and sets the movement state for interpolation. This ensures we capture initial movement
// for creatures created while moving, so their positions are up-to-date rather than frozen
// at create time (which was causing targeting of outdated locations).
func (w *WorldClient) parseCreateSplineData(r *bytes.Reader, obj *WorldObject) {
	var splineFlags uint32
	if err := binary.Read(r, binary.LittleEndian, &splineFlags); err != nil {
		return
	}

	// Handle final facing (angle, target, or point)
	if splineFlags&0x00040000 != 0 { // FINAL_ANGLE
		var angle float32
		binary.Read(r, binary.LittleEndian, &angle)
	} else if splineFlags&0x00020000 != 0 { // FINAL_TARGET
		var tgt uint64
		binary.Read(r, binary.LittleEndian, &tgt)
	} else if splineFlags&0x00010000 != 0 { // FINAL_POINT
		var fx, fy, fz float32
		binary.Read(r, binary.LittleEndian, &fx)
		binary.Read(r, binary.LittleEndian, &fy)
		binary.Read(r, binary.LittleEndian, &fz)
	}

	var timePassed, duration, splineID uint32
	binary.Read(r, binary.LittleEndian, &timePassed)
	binary.Read(r, binary.LittleEndian, &duration)
	binary.Read(r, binary.LittleEndian, &splineID)

	// 3.3.3 fields (duration_mod, duration_mod_next, vertical_accel, effect_start_time)
	var durationMod1, durationMod2, verticalAccel float32
	var effectStartTime uint32
	binary.Read(r, binary.LittleEndian, &durationMod1)
	binary.Read(r, binary.LittleEndian, &durationMod2)
	binary.Read(r, binary.LittleEndian, &verticalAccel)
	binary.Read(r, binary.LittleEndian, &effectStartTime)

	var nodeCount uint32
	binary.Read(r, binary.LittleEndian, &nodeCount)

	var lastX, lastY, lastZ float32
	origNodeCount := nodeCount
	if nodeCount > 1024 {
		// Guard against bogus huge nodeCount from misparsed data (CPU/reader exhaustion risk)
		nodeCount = 1024
	}

	if nodeCount > 0 {
		// We only need the last point for destination. Seek over all but the last to avoid CPU on bogus huge counts.
		if nodeCount > 1 {
			r.Seek(int64((nodeCount-1)*12), io.SeekCurrent)
		}
		binary.Read(r, binary.LittleEndian, &lastX)
		binary.Read(r, binary.LittleEndian, &lastY)
		binary.Read(r, binary.LittleEndian, &lastZ)
	}

	var mode uint8
	binary.Read(r, binary.LittleEndian, &mode)

	// final dest
	var fdX, fdY, fdZ float32
	binary.Read(r, binary.LittleEndian, &fdX)
	binary.Read(r, binary.LittleEndian, &fdY)
	binary.Read(r, binary.LittleEndian, &fdZ)

	// Use the final dest if present, else last node
	destX, destY, destZ := fdX, fdY, fdZ
	if origNodeCount > 0 && destX == 0 && destY == 0 && destZ == 0 {
		destX, destY, destZ = lastX, lastY, lastZ
	}

	obj.StartX = obj.PosX
	obj.StartY = obj.PosY
	obj.StartZ = obj.PosZ
	obj.DestX = destX
	obj.DestY = destY
	obj.DestZ = destZ
	obj.IsMoving = (origNodeCount > 0 || splineFlags != 0)
	obj.MoveStartTime = time.Now().Add(-time.Duration(timePassed) * time.Millisecond)
	obj.MoveDuration = time.Duration(duration) * time.Millisecond
}

// skipMovementUpdate consumes a movement block without allocating or storing anything.
// Used for other players/objects when we only need to keep the protocol stream in sync.
func (w *WorldClient) skipMovementUpdate(r *bytes.Reader) {
	var updateFlags uint16
	if err := binary.Read(r, binary.LittleEndian, &updateFlags); err != nil {
		return
	}

	if updateFlags&0x20 != 0 {
		// LIVING
		var moveFlags uint32
		binary.Read(r, binary.LittleEndian, &moveFlags)
		var moveFlags2 uint16
		binary.Read(r, binary.LittleEndian, &moveFlags2)
		var timestamp uint32
		binary.Read(r, binary.LittleEndian, &timestamp)

		r.Seek(16, io.SeekCurrent) // x y z o

		if moveFlags&0x00000200 != 0 {
			_, _ = readPackedGUID(r)
			r.Seek(21, io.SeekCurrent)
			if moveFlags2&0x0400 != 0 {
				var extra uint32
				binary.Read(r, binary.LittleEndian, &extra)
			}
		}
		if moveFlags&(0x00200000|0x02000000) != 0 || moveFlags2&0x0020 != 0 {
			var pitch float32
			binary.Read(r, binary.LittleEndian, &pitch)
		}
		var fallTime uint32
		binary.Read(r, binary.LittleEndian, &fallTime)
		if moveFlags&0x00001000 != 0 {
			r.Seek(16, io.SeekCurrent)
		}
		if moveFlags&0x04000000 != 0 {
			var se float32
			binary.Read(r, binary.LittleEndian, &se)
		}
		// 9 speeds
		r.Seek(36, io.SeekCurrent)
		if moveFlags&0x08000000 != 0 {
			w.skipSplineData(r)
		}
	} else if updateFlags&0x0100 != 0 {
		// UPDATEFLAG_POSITION
		_, _ = readPackedGUID(r)
		// x y z tx ty tz o facing (8 floats)
		r.Seek(32, io.SeekCurrent)
	} else if updateFlags&0x0040 != 0 {
		// STATIONARY_POSITION
		r.Seek(16, io.SeekCurrent)
	}

	if updateFlags&0x0008 != 0 {
		var unk uint32
		binary.Read(r, binary.LittleEndian, &unk)
	}
	if updateFlags&0x0010 != 0 {
		var lg uint32
		binary.Read(r, binary.LittleEndian, &lg)
	}
	if updateFlags&0x0004 != 0 {
		_, _ = readPackedGUID(r)
	}
	if updateFlags&0x0002 != 0 {
		_, _ = readPackedGUID(r)
	}
	if updateFlags&0x0200 != 0 {
		var rot int64
		binary.Read(r, binary.LittleEndian, &rot)
	}
}

// skipValuesUpdate consumes a values update block (mask + values) without storing or callbacks.
func (w *WorldClient) skipValuesUpdate(r *bytes.Reader) {
	var blockCount uint8
	if err := binary.Read(r, binary.LittleEndian, &blockCount); err != nil || blockCount == 0 {
		return
	}
	var mask [32]uint32
	totalValues := 0
	for i := uint8(0); i < blockCount && i < 32; i++ {
		binary.Read(r, binary.LittleEndian, &mask[i])
		for b := 0; b < 32; b++ {
			if mask[i]&(1<<uint(b)) != 0 {
				totalValues++
			}
		}
	}
	if totalValues > 0 {
		r.Seek(int64(totalValues)*4, io.SeekCurrent)
	}
}

func (w *WorldClient) readValuesUpdate(r *bytes.Reader, guid uint64) {
	// Read update mask
	var blockCount uint8
	if err := binary.Read(r, binary.LittleEndian, &blockCount); err != nil {
		return
	}

	if blockCount == 0 {
		return
	}

	mask := make([]uint32, blockCount)
	for i := uint8(0); i < blockCount; i++ {
		binary.Read(r, binary.LittleEndian, &mask[i])
	}

	obj := w.getOrCreateObject(guid)
	obj.LastSeen = time.Now()

	for i := uint8(0); i < blockCount; i++ {
		for bit := uint8(0); bit < 32; bit++ {
			if mask[i]&(1<<bit) != 0 {
				fieldIndex := uint16(i)*32 + uint16(bit)
				var value uint32
				binary.Read(r, binary.LittleEndian, &value)

				obj.setValue(fieldIndex, value)

				// Update derived fields
				switch fieldIndex {
				case UnitFieldEntry:
					obj.Entry = value
				case UnitDynamicFlags:
					if (value & (UnitDynflagDead | UnitDynflagLootable)) != 0 {
						// Server told us it's a dead/lootable corpse - force health 0 in our cache
						// even if a health value wasn't sent or is stale positive.
						obj.setValue(UnitFieldHealth, 0)
					}
				case UnitFieldHealth:
					if guid == w.charGUID {
						w.statsMu.Lock()
						oldHealth := w.health
						w.health = value
						w.statsMu.Unlock()
						if value == 0 && oldHealth > 0 {
							w.log("Bot has died! (health went to 0 in update)")
							if w.OnDeath != nil {
								w.OnDeath()
							}
						}
					}
				case UnitFieldMaxHealth:
					if guid == w.charGUID {
						w.statsMu.Lock()
						w.maxHealth = value
						w.statsMu.Unlock()
					}
				case UnitFieldPower1:
					if guid == w.charGUID {
						w.statsMu.Lock()
						w.power = value
						w.statsMu.Unlock()
					}
				case UnitFieldMaxPower1:
					if guid == w.charGUID {
						w.statsMu.Lock()
						w.maxPower = value
						w.statsMu.Unlock()
					}
				case UnitFieldLevel:
					if guid == w.charGUID {
						w.statsMu.Lock()
						w.level = value
						w.statsMu.Unlock()
					}
				}
			}
		}
	}

	if w.OnObjectUpdate != nil {
		w.OnObjectUpdate(guid, obj)
	}
}

func (w *WorldClient) handleDestroyObject(data []byte) {
	if len(data) < 8 {
		return
	}
	guid := binary.LittleEndian.Uint64(data[0:8])
	w.removeObject(guid)
}

func (w *WorldClient) handleMonsterMove(data []byte) {
	if len(data) < 16 {
		return
	}
	r := bytes.NewReader(data)
	guid, err := readPackedGUID(r)
	if err != nil {
		return
	}

	// uint8 flag (sets/unsets MOVEMENTFLAG2_UNK7 0x40)
	var unk8 uint8
	if err := binary.Read(r, binary.LittleEndian, &unk8); err != nil {
		return
	}

	// Current position
	var posX, posY, posZ float32
	binary.Read(r, binary.LittleEndian, &posX)
	binary.Read(r, binary.LittleEndian, &posY)
	binary.Read(r, binary.LittleEndian, &posZ)

	w.objectsMu.RLock()
	obj, ok := w.objects[guid]
	w.objectsMu.RUnlock()
	if !ok || obj == nil {
		return
	}

	// Always update to the current position from the packet
	obj.PosX = posX
	obj.PosY = posY
	obj.PosZ = posZ
	obj.LastPosUpdate = time.Now()
	obj.LastSeen = time.Now()

	// splineId (uint32)
	var splineID uint32
	if err := binary.Read(r, binary.LittleEndian, &splineID); err != nil {
		return
	}

	// MonsterMoveType (uint8): 0=Normal, 1=Stop, 2=FacingSpot, 3=FacingTarget, 4=FacingAngle
	var moveType uint8
	if err := binary.Read(r, binary.LittleEndian, &moveType); err != nil {
		return
	}

	if moveType == 1 { // MonsterMoveStop
		obj.IsMoving = false
		// snap to the stop pos
		obj.StartX = posX
		obj.StartY = posY
		obj.StartZ = posZ
		obj.DestX = posX
		obj.DestY = posY
		obj.DestZ = posZ
		return
	}

	// Skip facing data depending on move type
	switch moveType {
	case 2: // FacingSpot: 3 floats
		var fx, fy, fz float32
		binary.Read(r, binary.LittleEndian, &fx)
		binary.Read(r, binary.LittleEndian, &fy)
		binary.Read(r, binary.LittleEndian, &fz)
	case 3: // FacingTarget: uint64
		var ftarget uint64
		binary.Read(r, binary.LittleEndian, &ftarget)
	case 4: // FacingAngle: float32
		var fangle float32
		binary.Read(r, binary.LittleEndian, &fangle)
	}

	// splineFlags (uint32)
	var splineFlags uint32
	if err := binary.Read(r, binary.LittleEndian, &splineFlags); err != nil {
		return
	}

	// animation (if flag 0x00000100 - Animation)
	if splineFlags&0x00000100 != 0 {
		var animID uint8
		var animStartTime uint32
		binary.Read(r, binary.LittleEndian, &animID)
		binary.Read(r, binary.LittleEndian, &animStartTime)
	}

	// duration (uint32)
	var duration uint32
	if err := binary.Read(r, binary.LittleEndian, &duration); err != nil {
		return
	}

	// parabolic (if flag 0x00000200 - Parabolic)
	if splineFlags&0x00000200 != 0 {
		var verticalAccel float32
		var effectStartTime uint32
		binary.Read(r, binary.LittleEndian, &verticalAccel)
		binary.Read(r, binary.LittleEndian, &effectStartTime)
	}

	// Read waypoints to get destination
	var waypointCount uint32
	if err := binary.Read(r, binary.LittleEndian, &waypointCount); err != nil {
		return
	}

	if waypointCount == 0 {
		return
	}

	// For CatmullRom (flag 0x00000008), waypoints are full Vector3 positions
	// For linear paths, first point after count is the destination, rest are packed
	if splineFlags&0x00000008 != 0 {
		// CatmullRom: read all waypoints, last one is destination
		var lastX, lastY, lastZ float32
		for i := uint32(0); i < waypointCount; i++ {
			binary.Read(r, binary.LittleEndian, &lastX)
			binary.Read(r, binary.LittleEndian, &lastY)
			binary.Read(r, binary.LittleEndian, &lastZ)
		}
		obj.StartX = obj.PosX
		obj.StartY = obj.PosY
		obj.StartZ = obj.PosZ
		obj.DestX = lastX
		obj.DestY = lastY
		obj.DestZ = lastZ
		obj.IsMoving = true
		obj.MoveStartTime = time.Now()
		obj.MoveDuration = time.Duration(duration) * time.Millisecond
	} else {
		// Linear: destination is the first Vector3 after the count
		var destX, destY, destZ float32
		binary.Read(r, binary.LittleEndian, &destX)
		binary.Read(r, binary.LittleEndian, &destY)
		binary.Read(r, binary.LittleEndian, &destZ)
		obj.StartX = obj.PosX
		obj.StartY = obj.PosY
		obj.StartZ = obj.PosZ
		obj.DestX = destX
		obj.DestY = destY
		obj.DestZ = destZ
		obj.IsMoving = true
		obj.MoveStartTime = time.Now()
		obj.MoveDuration = time.Duration(duration) * time.Millisecond
	}

}

func (w *WorldClient) handleMonsterMoveTransport(data []byte) {
	// SMSG_MONSTER_MOVE_TRANSPORT has transport prefix: after opcode, typically unitGUID? + transGUID packed + seat + then standard monster move data (unk + pos + ...)
	// Since not main case per user, parse prefix and then update pos using similar logic to avoid missing pos updates.
	r := bytes.NewReader(data)
	unitGUID, err := readPackedGUID(r)
	if err != nil {
		return
	}
	transGUID, _ := readPackedGUID(r)
	var seat int8
	binary.Read(r, binary.LittleEndian, &seat)
	_ = transGUID
	_ = seat

	// now at unk8 + pos etc, same as after guid in regular
	var unk8 uint8
	if err := binary.Read(r, binary.LittleEndian, &unk8); err != nil {
		return
	}

	var posX, posY, posZ float32
	binary.Read(r, binary.LittleEndian, &posX)
	binary.Read(r, binary.LittleEndian, &posY)
	binary.Read(r, binary.LittleEndian, &posZ)

	w.objectsMu.RLock()
	obj, ok := w.objects[unitGUID]
	w.objectsMu.RUnlock()
	if !ok || obj == nil {
		return
	}
	obj.PosX = posX
	obj.PosY = posY
	obj.PosZ = posZ
	obj.LastPosUpdate = time.Now()
	obj.LastSeen = time.Now()

	// for full spline, would continue parse, but for pos correctness, pos is set. Skip rest for now to keep sync.
	// To fully support, could call similar parse code, but skip for this.
}

func (w *WorldClient) handleCompressedMoves(data []byte) {
	// (debug log removed)
	// TODO: decompress and parse multiple move packets.
}

func (w *WorldClient) handleMultipleMoves(data []byte) {
	// (debug log removed)
	// Parse multiple movement infos.
}

func (w *WorldClient) handleMovementPacket(opcode uint16, data []byte) {
	if len(data) < 8 {
		return
	}
	r := bytes.NewReader(data)
	guid, err := readPackedGUID(r)
	if err != nil {
		return
	}
	w.objectsMu.RLock()
	obj, ok := w.objects[guid]
	w.objectsMu.RUnlock()
	if !ok || obj == nil {
		// Not tracking, but consume to keep in sync? For now, try to parse pos anyway for debug.
		// Skip to pos.
		var moveFlags uint32
		binary.Read(r, binary.LittleEndian, &moveFlags)
		var moveFlags2 uint16
		binary.Read(r, binary.LittleEndian, &moveFlags2)
		var ts uint32
		binary.Read(r, binary.LittleEndian, &ts)
		var x, y, z, o float32
		binary.Read(r, binary.LittleEndian, &x)
		binary.Read(r, binary.LittleEndian, &y)
		binary.Read(r, binary.LittleEndian, &z)
		binary.Read(r, binary.LittleEndian, &o)
		// (debug log removed)
		return
	}
	// Parse basic movement info to update pos
	var moveFlags uint32
	binary.Read(r, binary.LittleEndian, &moveFlags)
	var moveFlags2 uint16
	binary.Read(r, binary.LittleEndian, &moveFlags2)
	var ts uint32
	binary.Read(r, binary.LittleEndian, &ts)
	var x, y, z, o float32
	binary.Read(r, binary.LittleEndian, &x)
	binary.Read(r, binary.LittleEndian, &y)
	binary.Read(r, binary.LittleEndian, &z)
	binary.Read(r, binary.LittleEndian, &o)
	obj.PosX = x
	obj.PosY = y
	obj.PosZ = z
	obj.Orientation = o
	obj.LastPosUpdate = time.Now()
	obj.LastSeen = time.Now()
	obj.IsMoving = (moveFlags&0x1000) != 0 || (moveFlags&0x4000) != 0 // forward or backward rough
	// (debug log removed)
}

func (w *WorldClient) handleMoveKnockBack(data []byte) {
	if len(data) < 8 {
		return
	}
	r := bytes.NewReader(data)
	guid, _ := readPackedGUID(r)
	w.objectsMu.RLock()
	obj, ok := w.objects[guid]
	w.objectsMu.RUnlock()
	if !ok || obj == nil {
		return
	}
	// packet has more: falltime, x y z speed horiz/vert etc. For pos update, we can try to read new pos if present in structure.
	// Basic: skip some, read possible pos fields. For now, log to see.
	// (debug log removed)
	// To properly update, would parse the knock target pos, but AC may follow with monster move.
	// For this, at least mark.
}

func (w *WorldClient) handleMoveTeleport(data []byte) {
	if len(data) < 8 {
		return
	}
	r := bytes.NewReader(data)
	guid, _ := readPackedGUID(r)
	w.objectsMu.RLock()
	obj, ok := w.objects[guid]
	w.objectsMu.RUnlock()
	if !ok || obj == nil {
		return
	}
	// similar to ack, has movement block with new pos
	var moveFlags uint32
	binary.Read(r, binary.LittleEndian, &moveFlags)
	var moveFlags2 uint16
	binary.Read(r, binary.LittleEndian, &moveFlags2)
	var ts uint32
	binary.Read(r, binary.LittleEndian, &ts)
	var nx, ny, nz, no float32
	binary.Read(r, binary.LittleEndian, &nx)
	binary.Read(r, binary.LittleEndian, &ny)
	binary.Read(r, binary.LittleEndian, &nz)
	binary.Read(r, binary.LittleEndian, &no)
	obj.PosX = nx
	obj.PosY = ny
	obj.PosZ = nz
	obj.Orientation = no
	obj.LastPosUpdate = time.Now()
	obj.LastSeen = time.Now()
	obj.IsMoving = false
	// (debug log removed)
}

func (w *WorldClient) handleNewWorld(data []byte) {
	if len(data) < 20 {
		return
	}
	r := bytes.NewReader(data)
	var mapID uint32
	binary.Read(r, binary.LittleEndian, &mapID)
	var newX, newY, newZ, newO float32
	binary.Read(r, binary.LittleEndian, &newX)
	binary.Read(r, binary.LittleEndian, &newY)
	binary.Read(r, binary.LittleEndian, &newZ)
	binary.Read(r, binary.LittleEndian, &newO)

	// (debug log removed)

	w.mapID = mapID
	w.posX = newX
	w.posY = newY
	w.posZ = newZ
	w.orientation = newO

	// Clear all objects since we changed maps
	w.objectsMu.Lock()
	w.objects = make(map[uint64]*WorldObject)
	w.objectsMu.Unlock()

	// Send MSG_MOVE_WORLDPORT_ACK
	w.sendPacket(0x00DC, nil) // CMSG_WORLD_PORT_RESPONSE = 0x00DC
}

func (w *WorldClient) handleMoveTeleportAck(data []byte) {
	if len(data) < 10 {
		return
	}
	r := bytes.NewReader(data)
	guid, _ := readPackedGUID(r)
	if guid != w.charGUID {
		return
	}

	// Read counter
	var counter uint32
	binary.Read(r, binary.LittleEndian, &counter)

	// Read movement block: moveFlags(4) + moveFlags2(2) + time(4) + posX(4) + posY(4) + posZ(4) + orientation(4)
	var moveFlags uint32
	binary.Read(r, binary.LittleEndian, &moveFlags)
	var moveFlags2 uint16
	binary.Read(r, binary.LittleEndian, &moveFlags2)
	var moveTime uint32
	binary.Read(r, binary.LittleEndian, &moveTime)
	var newX, newY, newZ, newO float32
	binary.Read(r, binary.LittleEndian, &newX)
	binary.Read(r, binary.LittleEndian, &newY)
	binary.Read(r, binary.LittleEndian, &newZ)
	binary.Read(r, binary.LittleEndian, &newO)

	// (debug log removed)

	w.posX = newX
	w.posY = newY
	w.posZ = newZ
	w.orientation = newO

	// Clear all objects since we teleported
	w.objectsMu.Lock()
	w.objects = make(map[uint64]*WorldObject)
	w.objectsMu.Unlock()

	// Send acknowledgment: packed GUID + uint32 flags + uint32 time
	ackBuf := new(bytes.Buffer)
	writePackedGUID(ackBuf, w.charGUID)
	binary.Write(ackBuf, binary.LittleEndian, uint32(0)) // flags
	binary.Write(ackBuf, binary.LittleEndian, uint32(0)) // time
	w.sendPacket(MsgMoveTeleportAck, ackBuf.Bytes())

	// Send heartbeat at new position
	w.SendHeartbeat()
}

func (w *WorldClient) getOrCreateObject(guid uint64) *WorldObject {
	w.objectsMu.Lock()
	defer w.objectsMu.Unlock()
	obj, ok := w.objects[guid]
	if !ok {
		obj = &WorldObject{
			GUID:   guid,
			Values: make(map[uint16]uint32),
		}
		w.objects[guid] = obj
	}
	return obj
}

func (w *WorldClient) removeObject(guid uint64) {
	w.objectsMu.Lock()
	delete(w.objects, guid)
	w.objectsMu.Unlock()
	if w.OnObjectRemove != nil {
		w.OnObjectRemove(guid)
	}
}

// SendPacketRaw exposes the encrypted packet sender for advanced usage.
func (w *WorldClient) SendPacketRaw(opcode uint16, data []byte) error {
	return w.sendPacket(opcode, data)
}

// logMovementPacket logs details of received movement packets (primarily from other players).
// Used to debug smoothness by running an observer client alongside a moving one.
// It logs opcode, GUID, flags, timestamp, position, orientation so deltas/speeds can be analyzed.
func (w *WorldClient) logMovementPacket(opcode uint16, data []byte) {
	if len(data) < 4 {
		return
	}

	r := bytes.NewReader(data)
	guid, err := readPackedGUID(r)
	if err != nil {
		w.log("[MOV] recv 0x%04X bad packed GUID: %v raw=% X", opcode, err, data)
		return
	}

	// Only care about other players' movement for observer analysis (skip self echoes if any)
	if guid == w.charGUID {
		return
	}

	var flags uint32
	var flags2 uint16
	var mtime uint32
	var px, py, pz, po float32

	// Common prefix for most movement packets: flags(4) + flags2(2) + time(4) + x y z o (16)
	if err := binary.Read(r, binary.LittleEndian, &flags); err != nil {
		//w.log("[MOV] recv 0x%04X guid=%d bad flags", opcode, guid)
		return
	}
	binary.Read(r, binary.LittleEndian, &flags2)
	binary.Read(r, binary.LittleEndian, &mtime)
	binary.Read(r, binary.LittleEndian, &px)
	binary.Read(r, binary.LittleEndian, &py)
	binary.Read(r, binary.LittleEndian, &pz)
	binary.Read(r, binary.LittleEndian, &po)

	// Log key info for smoothness analysis: packet ts, pos, to compute inter-packet deltas/speed/jitter from observer POV
	nowWall := time.Now()
	w.movDebugMu.Lock()
	prev, had := w.lastMovDebug[guid]
	deltaT := float64(0)
	deltaD := float64(0)
	estSpeed := float64(0)
	if had {
		deltaT = nowWall.Sub(prev.wall).Seconds()
		dx := px - prev.x
		dy := py - prev.y
		dz := pz - prev.z
		deltaD = math.Sqrt(float64(dx*dx + dy*dy + dz*dz))
		if deltaT > 0 {
			estSpeed = deltaD / deltaT
		}
	}
	w.lastMovDebug[guid] = struct {
		ts      uint32
		x, y, z float32
		wall    time.Time
	}{ts: mtime, x: px, y: py, z: pz, wall: nowWall}
	w.movDebugMu.Unlock()

	w.log("[MOV] recv 0x%04X guid=%d flags=0x%08X f2=0x%04X ts=%d pos=(%.3f,%.3f,%.3f) o=%.3f len=%d dt=%.3fs dd=%.3f speed=%.2f",
		opcode, guid, flags, flags2, mtime, px, py, pz, po, len(data), deltaT, deltaD, estSpeed)
}
