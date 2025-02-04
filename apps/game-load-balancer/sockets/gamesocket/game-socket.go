package gamesocket

import (
	"compress/zlib"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/game-load-balancer/crypto"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/packet"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/repo"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/session"
	"github.com/walkline/ToCloud9/apps/game-load-balancer/sockets"
	"github.com/walkline/ToCloud9/shared/slices"
)

// useEncryption used to disable encryption during testing
var useEncryption = true

// GameSocket socket between game client and load balancer
type GameSocket struct {
	logger zerolog.Logger
	conn   net.Conn

	packetsReader *sockets.PacketsReader
	encryption    *crypto.Arc

	sendChan chan *packet.Packet
	readChan chan *packet.Packet

	ctx context.Context

	accountRepo repo.AccountRepo

	sessionParams session.GameSessionParams
	session       *session.GameSession

	authSeed  []byte
	accountID uint32
}

func NewGameSocket(
	c net.Conn,
	accountRepo repo.AccountRepo,
	params session.GameSessionParams,
) sockets.Socket {
	return &GameSocket{
		conn:          c,
		packetsReader: sockets.NewPacketsReader(c, 4, packet.SourceGameClient),
		sendChan:      make(chan *packet.Packet, 10),
		readChan:      make(chan *packet.Packet, 10),
		logger:        log.Logger,
		accountRepo:   accountRepo,
		sessionParams: params,
	}
}

// Handshake sends handshake request
func (s *GameSocket) Handshake() error {
	s.authSeed = make([]byte, 4)
	_, err := rand.Read(s.authSeed)
	if err != nil {
		return err
	}

	randomBytes := make([]byte, 32)
	_, err = rand.Read(randomBytes)
	if err != nil {
		return err
	}

	// sending auth challenge
	p := packet.NewWriterWithSize(packet.SMsgAuthChallenge, uint32(len(s.authSeed)+len(randomBytes)+4))
	p.Uint32(1)
	p.Bytes(s.authSeed)
	p.Bytes(randomBytes)
	s.Send(p)
	return nil
}

// ListenAndProcess listen for incoming messages and starts to handle them from here
// BLOCKS WHILE CONNECTION IS OPEN
func (s *GameSocket) ListenAndProcess(ctx context.Context) error {
	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.ctx = newCtx

	go func() {
		for {
			select {
			case p, open := <-s.sendChan:
				if !open {
					return
				}

				err := s.sendOriginalPacket(p)
				if err != nil {
					s.logger.Error().Err(err).Msg("can't send packet to the game client")
				}
			case <-s.ctx.Done():
				if s.session == nil {
					close(s.sendChan)
				}
				return
			}
		}
	}()

	err := s.Handshake()
	if err != nil {
		return err
	}

	for s.packetsReader.Next() {
		p := s.packetsReader.Packet()
		if e := s.logger.Debug(); e.Enabled() {
			s.logger.Debug().
				Str("opcode", fmt.Sprintf("%s (0x%X)", p.Opcode.String(), uint16(p.Opcode))).
				Uint32("size", p.Size).
				Msg("📦 Game=>Balancer")
		}

		err = s.processPacket(p)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("opcode", fmt.Sprintf("%s (0x%X)", p.Opcode.String(), p.Opcode)).
				Uint32("size", p.Size).
				Str("payload", fmt.Sprintf("%v", p.Data)).
				Msg("Failed to process packet")
		}
	}

	return nil
}

// Close socket
func (s *GameSocket) Close() {
	_ = s.conn.Close() // do we need to handle this somehow?
}

// ReadChannel returns channel with packets from game client
func (s *GameSocket) ReadChannel() <-chan *packet.Packet {
	return s.readChan
}

// WriteChannel returns channel that consumes packets that needs to be sent to the game client
func (s *GameSocket) WriteChannel() chan<- *packet.Packet {
	return s.sendChan
}

// Send sends writer to the game client
func (s *GameSocket) Send(p *packet.Writer) {
	s.sendChan <- &packet.Packet{
		Opcode: p.Opcode,
		Size:   uint32(p.Payload.Len()),
		Data:   p.Payload.Bytes(),
	}
}

// SendPacket sends packet to the game client
func (s *GameSocket) SendPacket(p *packet.Packet) {
	s.sendChan <- p
}

// AuthSession handles CMsgAuthSession and creates session object that takes control over the packets.
func (s *GameSocket) AuthSession(p *packet.Packet) error {
	reader := p.Reader()

	/*build*/
	_ = reader.Uint32()
	/*loginServerID*/ _ = reader.Uint32()
	account := reader.String()
	/*loginServerType*/ _ = reader.Uint32()

	localChallenge := make([]uint8, 4)
	reader.Read(&localChallenge)

	/*regionID*/
	_ = reader.Uint32()
	/*battlegroundID*/ _ = reader.Uint32()
	/*realmID*/ _ = reader.Uint32()
	/*dosResponse*/ _ = reader.Uint64()

	theirDigest := make([]byte, 20)
	reader.Read(&theirDigest)

	addonInfo := make([]byte, reader.Left())
	reader.Read(&addonInfo)

	if reader.Error() != nil {
		return fmt.Errorf("can't process AuthSession, err: %w", reader.Error())
	}

	accountObj, err := s.accountRepo.AccountByUserName(context.TODO(), account)
	if err != nil {
		return err
	}

	if useEncryption {
		t := []byte{0, 0, 0, 0}
		ourDigest := sha1.Sum(slices.AppendBytes([]byte(account), t, localChallenge, s.authSeed, accountObj.SessionKeyAuth))
		if !slices.SameBytes(ourDigest[:], theirDigest) {
			return fmt.Errorf("authentication failed, account: %s (%d)", account, accountObj.ID)
		}
	}

	s.accountID = accountObj.ID
	s.logger = s.logger.With().Uint32("account", accountObj.ID).Logger()

	if useEncryption {
		var err error
		s.encryption, err = crypto.NewArc(accountObj.SessionKeyAuth)
		if err != nil {
			return fmt.Errorf("can't create encryptor, err: %w", err)
		}
	}

	s.packetsReader.EnableEncryption(s.encryption)

	resp := packet.NewWriterWithSize(packet.SMsgAuthResponse, 1+4+1+4+1)
	resp.Uint8(12)
	resp.Uint32(0) // BillingTimeRemaining
	resp.Uint8(0)  // BillingPlanFlags
	resp.Uint32(0) // BillingTimeRested
	resp.Uint8(2)  // 0 - normal, 1 - TBC, 2 - WOTLK, must be set in database manually for each account
	s.Send(resp)

	err = s.handleAddons(addonInfo)
	if err != nil {
		return fmt.Errorf("can't handle addons, err: %w", err)
	}

	resp = packet.NewWriterWithSize(packet.SMsgTutorialFlags, 8*4)
	for i := 0; i < 8; i++ {
		resp.Uint32(0xFFFFFFFF)
	}
	s.Send(resp)

	s.session = session.NewGameSession(s.ctx, &s.logger, s, s.accountID, p, s.sessionParams)
	go s.session.HandlePackets(s.ctx)

	return nil
}

func (s *GameSocket) Address() string {
	return s.conn.RemoteAddr().String()
}

func (s *GameSocket) handleAddons(b []byte) error {
	r := packet.NewReaderWithData(b)
	size := r.Uint32()
	if size == 0 {
		return nil
	}

	if size > 0xFFFFF {
		return fmt.Errorf("addon info is to big, size: %d", size)
	}

	size -= 4

	decodeReader, err := zlib.NewReader(r.RawReader())
	if err != nil {
		return fmt.Errorf("can't decode addon info, err: %w", err)
	}

	decodedData := make([]byte, size)
	n, err := decodeReader.Read(decodedData)
	if err != nil {
		return fmt.Errorf("can't decode addon info, err: %w", err)
	}

	if n != int(size) {
		return fmt.Errorf("different decode data size, exp: %d, got: %d", size, n)
	}

	r = packet.NewReaderWithData(decodedData)
	addonsCount := r.Uint32()

	//addonPubKey := []byte{
	//	0xC3, 0x5B, 0x50, 0x84, 0xB9, 0x3E, 0x32, 0x42, 0x8C, 0xD0, 0xC7, 0x48, 0xFA, 0x0E, 0x5D, 0x54,
	//	0x5A, 0xA3, 0x0E, 0x14, 0xBA, 0x9E, 0x0D, 0xB9, 0x5D, 0x8B, 0xEE, 0xB6, 0x84, 0x93, 0x45, 0x75,
	//	0xFF, 0x31, 0xFE, 0x2F, 0x64, 0x3F, 0x3D, 0x6D, 0x07, 0xD9, 0x44, 0x9B, 0x40, 0x85, 0x59, 0x34,
	//	0x4E, 0x10, 0xE1, 0xE7, 0x43, 0x69, 0xEF, 0x7C, 0x16, 0xFC, 0xB4, 0xED, 0x1B, 0x95, 0x28, 0xA8,
	//	0x23, 0x76, 0x51, 0x31, 0x57, 0x30, 0x2B, 0x79, 0x08, 0x50, 0x10, 0x1C, 0x4A, 0x1A, 0x2C, 0xC8,
	//	0x8B, 0x8F, 0x05, 0x2D, 0x22, 0x3D, 0xDB, 0x5A, 0x24, 0x7A, 0x0F, 0x13, 0x50, 0x37, 0x8F, 0x5A,
	//	0xCC, 0x9E, 0x04, 0x44, 0x0E, 0x87, 0x01, 0xD4, 0xA3, 0x15, 0x94, 0x16, 0x34, 0xC6, 0xC2, 0xC3,
	//	0xFB, 0x49, 0xFE, 0xE1, 0xF9, 0xDA, 0x8C, 0x50, 0x3C, 0xBE, 0x2C, 0xBB, 0x57, 0xED, 0x46, 0xB9,
	//	0xAD, 0x8B, 0xC6, 0xDF, 0x0E, 0xD6, 0x0F, 0xBE, 0x80, 0xB3, 0x8B, 0x1E, 0x77, 0xCF, 0xAD, 0x22,
	//	0xCF, 0xB7, 0x4B, 0xCF, 0xFB, 0xF0, 0x6B, 0x11, 0x45, 0x2D, 0x7A, 0x81, 0x18, 0xF2, 0x92, 0x7E,
	//	0x98, 0x56, 0x5D, 0x5E, 0x69, 0x72, 0x0A, 0x0D, 0x03, 0x0A, 0x85, 0xA2, 0x85, 0x9C, 0xCB, 0xFB,
	//	0x56, 0x6E, 0x8F, 0x44, 0xBB, 0x8F, 0x02, 0x22, 0x68, 0x63, 0x97, 0xBC, 0x85, 0xBA, 0xA8, 0xF7,
	//	0xB5, 0x40, 0x68, 0x3C, 0x77, 0x86, 0x6F, 0x4B, 0xD7, 0x88, 0xCA, 0x8A, 0xD7, 0xCE, 0x36, 0xF0,
	//	0x45, 0x6E, 0xD5, 0x64, 0x79, 0x0F, 0x17, 0xFC, 0x64, 0xDD, 0x10, 0x6F, 0xF3, 0xF5, 0xE0, 0xA6,
	//	0xC3, 0xFB, 0x1B, 0x8C, 0x29, 0xEF, 0x8E, 0xE5, 0x34, 0xCB, 0xD1, 0x2A, 0xCE, 0x79, 0xC3, 0x9A,
	//	0x0D, 0x36, 0xEA, 0x01, 0xE0, 0xAA, 0x91, 0x20, 0x54, 0xF0, 0x72, 0xD8, 0x1E, 0xC7, 0x89, 0xD2,
	//}

	w := packet.NewWriter(packet.SMsgAddonInfo)
	for i := uint32(0); i < addonsCount; i++ {
		addonName := r.String()
		hasKey := r.Uint8()
		publicKeyCRC := r.Uint32()
		urlCRC := r.Uint32()

		w.Uint8(2) // mark everything as allowed & hidden
		w.Uint8(hasKey)
		if hasKey == 1 {
			w.Uint8(0)
			w.Uint32(0)
		}
		w.Uint8(0)

		s.logger.Debug().
			Str("name", addonName).
			Bool("hasKey", hasKey == 1).
			Uint32("publicKeyCRC", publicKeyCRC).
			Uint32("urlCRC", urlCRC).
			Msg("Addon Received")
	}

	w.Uint32(0)
	s.Send(w)

	return nil
}

func (s *GameSocket) sendOriginalPacket(p *packet.Packet) error {
	if e := s.logger.Debug(); e.Enabled() {
		s.logger.
			Debug().
			Str("opcode", fmt.Sprintf("%s (0x%X)", p.Opcode.String(), uint16(p.Opcode))).
			Int("size", len(p.Data)).
			Msg("📦 Balancer=>Game")
	}

	header := make([]byte, 4, len(p.Data)+4)
	binary.BigEndian.PutUint16(header[0:2], uint16(len(p.Data)+2))
	binary.LittleEndian.PutUint16(header[2:4], uint16(p.Opcode))

	if s.encryption != nil {
		s.encryption.Encrypt(header)
	}

	_, err := s.conn.Write(append(header, p.Data...))

	return err
}

func (s *GameSocket) processPacket(p *packet.Packet) error {
	switch p.Opcode {
	case packet.CMsgAuthSession:
		if s.session == nil {
			return s.AuthSession(p)
		}
	default:
		s.readChan <- p
	}
	return nil
}
