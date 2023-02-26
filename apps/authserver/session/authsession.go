package session

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/walkline/ToCloud9/apps/authserver/crypto/srp6"
	"github.com/walkline/ToCloud9/apps/authserver/repo"
	"github.com/walkline/ToCloud9/apps/authserver/service"
)

type Status uint8

const (
	StatusChallenge Status = iota
	StatusLogonProof
	StatusReconnectProof
	StatusAuthed
	StatusClosed
)

type AuthResult uint8

const (
	AuthResultSuccess           AuthResult = 0
	AuthResultBanned            AuthResult = 3
	AuthResultUnkAccount        AuthResult = 4
	AuthResultIncorrectPassword AuthResult = 5
	AuthResultAlreadyOnline     AuthResult = 6
	AuthResultNoTime            AuthResult = 7
)

type Command uint8

const (
	CommandLogonChallenge Command = iota
	CommandLogonProof
	CommandReconnectChallenge
	CommandReconnectProof
	CommandRealmList Command = 0x10
)

type CommandData struct {
	ValidStatus Status
	Size        int
}

var Commands = map[Command]Status{
	CommandLogonChallenge:     StatusChallenge,
	CommandLogonProof:         StatusLogonProof,
	CommandReconnectChallenge: StatusChallenge,
	CommandReconnectProof:     StatusReconnectProof,
	CommandRealmList:          StatusAuthed,
}

type AuthSession struct {
	logger       zerolog.Logger
	accountRepo  repo.AccountRepo
	realmService service.RealmService

	conn net.Conn

	srp            *srp6.SRP6
	status         Status
	account        *repo.Account
	reconnectProof []byte
}

func NewAuthSession(conn net.Conn, accountRepo repo.AccountRepo, realmService service.RealmService) *AuthSession {
	return &AuthSession{
		conn:         conn,
		logger:       log.Logger.With().Str("address", conn.RemoteAddr().String()).Logger(),
		accountRepo:  accountRepo,
		realmService: realmService,
	}
}

func (s *AuthSession) ListenAndProcess() {
	s.logger.Debug().Msg("New connection")
	defer func(t time.Time) {
		_ = s.conn.Close()
		s.logger.Debug().Msgf("Socket closed. Session lifetime: %v.", time.Since(t))
	}(time.Now())

	opcode := make([]byte, 1)
	for {
		err := s.conn.SetDeadline(time.Now().UTC().Add(time.Minute * 2))
		if err != nil {
			s.logger.Error().Err(err).Msg("can't set deadline")
			return
		}

		_, err = s.conn.Read(opcode)
		if err != nil {
			// not really an error at this stage
			if errors.Is(err, io.EOF) {
				return
			} else if errors.Is(err, os.ErrDeadlineExceeded) {
				s.logger.Debug().Msg("disconnect idle connection")
				return
			}

			s.logger.Error().Err(err).Msg("can't read opcode")
			return
		}

		validStatus, found := Commands[Command(opcode[0])]
		if !found {
			s.logger.Debug().Msgf("unk command 0x%X", opcode[0])
			return
		}

		if s.status != validStatus {
			_ = s.conn.Close()
			s.logger.Error().Msgf("invalid status, cmd - 0x%X, expected - %d, have - %d", opcode[0], validStatus, s.status)
			return
		}

		switch Command(opcode[0]) {
		case CommandLogonChallenge:
			err = s.HandleLogonChallenge()
		case CommandLogonProof:
			err = s.HandleLogonProof()
		case CommandRealmList:
			err = s.HandleRealmList()
		case CommandReconnectChallenge:
			err = s.HandleReconnectChallenge()
		case CommandReconnectProof:
			err = s.HandleReconnectProof()
		}

		if err != nil {
			s.logger.Error().Err(err).Msgf("can't process command 0x%X", opcode[0])
			_ = s.conn.Close()
			return
		}
	}
}

func (s *AuthSession) HandleReconnectChallenge() error {
	s.status = StatusClosed

	type payload struct {
		Err          uint8
		Size         uint16
		GameName     [4]byte
		Version1     uint8
		Version2     uint8
		Version3     uint8
		Build        uint16
		Platform     [4]byte
		OS           [4]byte
		Country      [4]byte
		TimezoneBias uint32
		IP           uint32
		ILen         uint8
	}

	d := payload{}
	err := binary.Read(s.conn, binary.LittleEndian, &d)
	if err != nil {
		return err
	}

	login := make([]byte, d.ILen)

	_, err = s.conn.Read(login)
	if err != nil {
		return err
	}

	username := string(login)

	s.logger = s.logger.With().Str("login", username).Logger()
	s.logger.Debug().Interface("payload", &d).Msg("Received reconnect challenge")

	s.account, err = s.accountRepo.AccountByUserName(context.TODO(), username)
	if err != nil {
		return err
	}

	s.reconnectProof = make([]byte, 16)
	_, err = rand.Read(s.reconnectProof)
	if err != nil {
		return err
	}

	s.status = StatusReconnectProof

	err = s.Write(
		CommandReconnectChallenge,
		AuthResultSuccess,
		s.reconnectProof,
		[]byte{0xBA, 0xA3, 0x1E, 0x99, 0xA0, 0x0B, 0x21, 0x57, 0xFC, 0x37, 0x3F, 0xB3, 0x69, 0xCD, 0xD2, 0xF1},
	)
	if err != nil {
		return err
	}

	return nil
}

func (s *AuthSession) HandleReconnectProof() error {
	s.logger.Debug().Msg("Handling reconnect proof")

	type payload struct {
		R1           [16]byte
		R2           [20]byte
		R3           [20]byte
		NumberOfKeys uint8
	}

	d := payload{}
	err := binary.Read(s.conn, binary.LittleEndian, &d)
	if err != nil {
		return err
	}

	if !srp6.ReconnectChallengeValid(s.account.Username, d.R1[:], d.R2[:], s.reconnectProof, s.account.SessionKeyAuth) {
		return errors.New("received bad password during reconnect proof")
	}

	err = s.Write(
		CommandReconnectProof,
		AuthResultSuccess,
		uint16(0),
	)
	if err != nil {
		return err
	}

	s.status = StatusAuthed

	return nil
}

func (s *AuthSession) HandleLogonChallenge() error {
	s.status = StatusClosed

	type payload struct {
		Err          uint8
		Size         uint16
		GameName     [4]byte
		Version1     uint8
		Version2     uint8
		Version3     uint8
		Build        uint16
		Platform     [4]byte
		OS           [4]byte
		Country      [4]byte
		TimezoneBias uint32
		IP           uint32
		ILen         uint8
	}

	d := payload{}
	err := binary.Read(s.conn, binary.LittleEndian, &d)
	if err != nil {
		return err
	}

	login := make([]byte, d.ILen)

	_, err = s.conn.Read(login)
	if err != nil {
		return err
	}

	username := string(login)

	s.logger = s.logger.With().Str("login", username).Logger()
	s.logger.Debug().Interface("payload", &d).Msg("Received login challenge")

	s.account, err = s.accountRepo.AccountByUserName(context.TODO(), username)
	if err != nil {
		return err
	}

	if s.account == nil {
		return s.Write(CommandLogonProof, AuthResultUnkAccount, uint16(0))
	}

	s.srp = srp6.NewSRP(string(login), s.account.Salt, s.account.Verifier)

	B, g, N, _s := s.srp.DataForClient()
	err = s.Write(
		[]byte{byte(CommandLogonChallenge), 0},
		AuthResultSuccess,
		B,
		byte(1),
		g,
		byte(32),
		N,
		_s,
		[]byte{0xBA, 0xA3, 0x1E, 0x99, 0xA0, 0x0B, 0x21, 0x57, 0xFC, 0x37, 0x3F, 0xB3, 0x69, 0xCD, 0xD2, 0xF1},
		byte(0),
	)
	if err != nil {
		return err
	}

	s.status = StatusLogonProof

	return nil
}

func (s *AuthSession) HandleLogonProof() error {
	s.logger.Debug().Msg("Handling logon proof")

	type payload struct {
		A             [32]byte
		ClientM       [20]byte
		CRCHash       [20]byte
		NumberOfKeys  uint8
		SecurityFlags uint8
	}

	d := payload{}
	err := binary.Read(s.conn, binary.LittleEndian, &d)
	if err != nil {
		return err
	}

	K := s.srp.VerifyChallengeResponse(d.A[:], d.ClientM[:])
	if K == nil {
		s.logger.Debug().Msg("Received bad password")
		return s.Write(CommandLogonProof, AuthResultUnkAccount, uint16(0))
	}

	s.account.SessionKeyAuth = K
	err = s.accountRepo.UpdateAccount(context.TODO(), s.account)
	if err != nil {
		return err
	}

	type responsePayload struct {
		CMD          uint8
		Err          uint8
		M2           [20]byte
		AccountFlags uint32
		SurveyID     uint32
		LoginFlags   uint16
	}

	response := responsePayload{
		CMD:          uint8(CommandLogonProof),
		Err:          0,
		M2:           srp6.GetSessionVerifier(d.A[:], d.ClientM[:], K),
		AccountFlags: 0x00800000,
		SurveyID:     0,
		LoginFlags:   0,
	}

	err = s.Write(&response)
	if err != nil {
		return err
	}

	s.status = StatusAuthed

	return nil
}

func (s *AuthSession) HandleRealmList() error {
	var realmList []service.RealmListItem

	s.logger.Debug().Msg("Handling realm list")
	defer func(t time.Time) {
		s.logger.Debug().Interface("realms", realmList).Msgf("Processed realm list. Took %v time.", time.Since(t))
	}(time.Now())

	// need to read 4 bytes, but we don't need them
	unk := uint32(0)
	err := binary.Read(s.conn, binary.LittleEndian, &unk)
	if err != nil {
		return err
	}

	realmList, err = s.realmService.RealmListForAccount(context.TODO(), s.account)
	if err != nil {
		return err
	}

	pkt := new(bytes.Buffer)
	for _, realm := range realmList {
		err = s.write(pkt,
			realm.Icon,
			realm.Locked,
			realm.Flag,
			realm.Name,
			realm.Address,
			realm.PopulationLevel,
			realm.CharsCount,
			realm.Timezone,
			uint8(realm.ID),
		)
		if err != nil {
			return err
		}
	}

	err = s.write(pkt, uint8(0x10), uint8(0x00))
	if err != nil {
		return err
	}

	pktData, err := ioutil.ReadAll(pkt)
	if err != nil {
		return err
	}

	err = s.Write(
		CommandRealmList,
		uint16(len(pktData)+6),
		uint32(0),
		uint16(len(realmList)),
		pktData,
	)

	if err != nil {
		return err
	}

	s.status = StatusAuthed

	return nil
}

func (s *AuthSession) Write(v ...interface{}) error {
	return s.write(s.conn, v...)
}

func (s *AuthSession) write(writer io.Writer, v ...interface{}) error {
	var err error
	for i := range v {
		d := v[i]
		switch d.(type) {
		case string:
			d = append([]byte(d.(string)), 0)
		}

		err = binary.Write(writer, binary.LittleEndian, d)
		if err != nil {
			return err
		}
	}
	return nil
}
