package wowsimclient

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// Opcodes (subset needed for the bot)
const (
	SmsgAuthChallenge uint16 = 0x01EC
	CmsgAuthSession   uint16 = 0x01ED
	SmsgAuthResponse   uint16 = 0x01EE

	CmsgCharEnum    uint16 = 0x0037
	SmsgCharEnum    uint16 = 0x003B
	CmsgCharCreate  uint16 = 0x0036
	SmsgCharCreate  uint16 = 0x003A
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

	MsgMoveJump       uint16 = 0x00BB
	MsgMoveFallLand   uint16 = 0x00C9
	MsgMoveHeartbeat  uint16 = 0x00EE
	MsgMoveStartForward uint16 = 0x00B5
	MsgMoveStop       uint16 = 0x00B7

	CmsgSetActiveMover   uint16 = 0x026A
	CmsgLogoutRequest    uint16 = 0x004B
	SmsgLogoutResponse   uint16 = 0x004C
	SmsgLogoutComplete   uint16 = 0x004D

	SmsgWardenData uint16 = 0x02E6
	CmsgWardenData uint16 = 0x02E7

	CmsgReadyForAccountDataTimes uint16 = 0x04FF
	SmsgAccountDataTimes         uint16 = 0x0209
	CmsgRealmSplit               uint16 = 0x038C
	SmsgRealmSplit               uint16 = 0x038B

	SmsgAddonInfo        uint16 = 0x02EF
	SmsgTutorialFlags    uint16 = 0x00FD
	SmsgCancelCombat     uint16 = 0x014E

	CmsgCompleteCinematic uint16 = 0x00FC
	CmsgNextCinematicCamera uint16 = 0x00FB
)

// Chat message types
const (
	ChatMsgSay   uint32 = 0x00
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

// CharEnumEntry holds character data from SMSG_CHAR_ENUM
type CharEnumEntry struct {
	GUID  uint64
	Name  string
	Race  uint8
	Class uint8
	Level uint8
}

// WorldClient handles the world server protocol
type WorldClient struct {
	conn       net.Conn
	username   string
	sessionKey []byte

	encryptServer *rc4.Cipher // for decrypting server -> client headers
	encryptClient *rc4.Cipher // for encrypting client -> server headers
	encrypted     bool

	sendMu sync.Mutex

	charGUID        uint64
	timeSyncCounter uint32
	posX, posY, posZ, orientation float32

	loginDone chan struct{}
	logoutDone chan struct{}

	stopChan chan struct{}
	stopped  bool

	lastError error

	// Callbacks
	logFunc          func(format string, args ...interface{})
	OnCharList       func(chars []CharEnumEntry)
	OnCharCreateResult func(data []byte)
}

// NewWorldClient creates a world client
func NewWorldClient(username string, sessionKey []byte, logFunc func(string, ...interface{})) *WorldClient {
	return &WorldClient{
		username:   strings.ToUpper(username),
		sessionKey: sessionKey,
		loginDone:  make(chan struct{}),
		logoutDone: make(chan struct{}),
		stopChan:   make(chan struct{}),
		logFunc:    logFunc,
	}
}

// Connect connects to the world server
func (w *WorldClient) Connect(worldAddr string) error {
	var err error
	w.conn, err = net.DialTimeout("tcp", worldAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("connect to worldserver: %w", err)
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
	header := make([]byte, 4)
	if _, err := io.ReadFull(w.conn, header); err != nil {
		return err
	}

	size := binary.BigEndian.Uint16(header[0:2]) - 2
	opcode := binary.LittleEndian.Uint16(header[2:4])

	if opcode != SmsgAuthChallenge {
		return fmt.Errorf("expected SMSG_AUTH_CHALLENGE (0x%X), got 0x%X", SmsgAuthChallenge, opcode)
	}

	data := make([]byte, size)
	if _, err := io.ReadFull(w.conn, data); err != nil {
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
	buf.Write(append([]byte(w.username), 0))               // null-terminated username
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
	// Server sends either:
	// Normal:  size(2, big-endian) + opcode(2, little-endian) = 4 bytes header
	// Large:   size(3, big-endian with 0x80 flag) + opcode(2, little-endian) = 5 bytes header
	// The first byte's high bit (0x80) indicates a large packet.

	// Read first byte to determine header size
	firstByte := make([]byte, 1)
	if _, err := io.ReadFull(w.conn, firstByte); err != nil {
		return 0, nil, fmt.Errorf("read first byte: %w", err)
	}

	isLarge := false
	var headerSize int
	if w.encrypted {
		// We need to decrypt, but we have to read more bytes first
		// Read the rest of the header
		// For normal: 3 more bytes (total 4)
		// We won't know if it's large until we decrypt the first byte
		// So read 3 more bytes tentatively
		rest := make([]byte, 3)
		if _, err := io.ReadFull(w.conn, rest); err != nil {
			return 0, nil, fmt.Errorf("read rest of header: %w", err)
		}

		header := append(firstByte, rest...)
		w.encryptServer.XORKeyStream(header, header)

		if header[0]&0x80 != 0 {
			// Large packet - need one more byte
			isLarge = true
			extra := make([]byte, 1)
			if _, err := io.ReadFull(w.conn, extra); err != nil {
				return 0, nil, err
			}
			w.encryptServer.XORKeyStream(extra, extra)
			header = append(header, extra...)
			headerSize = 5
		} else {
			headerSize = 4
		}

		var size uint32
		var opcode uint16
		if isLarge {
			size = (uint32(header[0]&0x7F) << 16) | (uint32(header[1]) << 8) | uint32(header[2])
			opcode = binary.LittleEndian.Uint16(header[3:5])
		} else {
			size = (uint32(header[0]) << 8) | uint32(header[1])
			opcode = binary.LittleEndian.Uint16(header[2:4])
		}

		_ = headerSize

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

		data := make([]byte, payloadSize)
		if _, err := io.ReadFull(w.conn, data); err != nil {
			return 0, nil, err
		}

		return opcode, data, nil
	}

	// Unencrypted path
	if firstByte[0]&0x80 != 0 {
		isLarge = true
	}

	if isLarge {
		rest := make([]byte, 4) // 2 more size bytes + 2 opcode bytes
		if _, err := io.ReadFull(w.conn, rest); err != nil {
			return 0, nil, err
		}
		size := (uint32(firstByte[0]&0x7F) << 16) | (uint32(rest[0]) << 8) | uint32(rest[1])
		opcode := binary.LittleEndian.Uint16(rest[2:4])

		if size < 2 {
			return opcode, nil, nil
		}
		payloadSize := int(size) - 2
		if payloadSize == 0 {
			return opcode, nil, nil
		}
		data := make([]byte, payloadSize)
		if _, err := io.ReadFull(w.conn, data); err != nil {
			return 0, nil, err
		}
		return opcode, data, nil
	}

	// Normal unencrypted
	rest := make([]byte, 3) // 1 more size byte + 2 opcode bytes
	if _, err := io.ReadFull(w.conn, rest); err != nil {
		return 0, nil, err
	}
	size := (uint32(firstByte[0]) << 8) | uint32(rest[0])
	opcode := binary.LittleEndian.Uint16(rest[1:3])

	if size < 2 {
		return opcode, nil, nil
	}
	payloadSize := int(size) - 2
	if payloadSize == 0 {
		return opcode, nil, nil
	}
	data := make([]byte, payloadSize)
	if _, err := io.ReadFull(w.conn, data); err != nil {
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

	if w.encrypted {
		w.encryptClient.XORKeyStream(header, header)
	}

	w.sendMu.Lock()
	defer w.sendMu.Unlock()

	_, err := w.conn.Write(append(header, data...))
	return err
}

func (w *WorldClient) handlePacket(opcode uint16, data []byte) {
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
		// Pong received, nothing to do
	case SmsgWardenData:
		// Ignore warden
	case SmsgLogoutResponse:
		w.handleLogoutResponse(data)
	case SmsgLogoutComplete:
		w.handleLogoutComplete()
	case SmsgCancelCombat:
		// ignore
	default:
		// Ignore all other packets
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
		w.log("Login verified - map %d, pos (%.1f, %.1f, %.1f)", mapID, w.posX, w.posY, w.posZ)
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

// LoginCharacter sends CMSG_PLAYER_LOGIN
func (w *WorldClient) LoginCharacter(guid uint64) error {
	w.charGUID = guid
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
	binary.Write(buf, binary.LittleEndian, uint32(0x00002000)) // movementFlags: MOVEMENTFLAG_FALLING
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
	binary.Write(buf, binary.LittleEndian, float32(7.96))  // zspeed
	binary.Write(buf, binary.LittleEndian, float32(0.0))   // sinAngle
	binary.Write(buf, binary.LittleEndian, float32(1.0))   // cosAngle
	binary.Write(buf, binary.LittleEndian, float32(0.0))   // xyspeed
	return w.sendPacket(MsgMoveJump, buf.Bytes())
}

// SendHeartbeat sends a movement heartbeat
func (w *WorldClient) SendHeartbeat() error {
	buf := new(bytes.Buffer)
	writePackedGUID(buf, w.charGUID)
	binary.Write(buf, binary.LittleEndian, uint32(0))          // movementFlags: none
	binary.Write(buf, binary.LittleEndian, uint16(0))          // movementFlags2
	binary.Write(buf, binary.LittleEndian, uint32(getMSTime()))
	binary.Write(buf, binary.LittleEndian, w.posX)
	binary.Write(buf, binary.LittleEndian, w.posY)
	binary.Write(buf, binary.LittleEndian, w.posZ)
	binary.Write(buf, binary.LittleEndian, w.orientation)
	binary.Write(buf, binary.LittleEndian, uint32(0)) // fallTime
	return w.sendPacket(MsgMoveHeartbeat, buf.Bytes())
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

// Close closes the connection
func (w *WorldClient) Close() {
	w.sendMu.Lock()
	defer w.sendMu.Unlock()
	if w.conn != nil {
		w.conn.Close()
	}
}

// StopChan returns the channel that is closed when the connection stops
func (w *WorldClient) StopChan() <-chan struct{} {
	return w.stopChan
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
