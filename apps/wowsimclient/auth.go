package wowsimclient

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"time"
)

// Auth command opcodes
const (
	AuthLogonChallenge  byte = 0x00
	AuthLogonProof      byte = 0x01
	AuthRealmList       byte = 0x10
)

// SRP6 parameters
var (
	srpN = bigIntFromBytes([]byte{
		0xB7, 0x9B, 0x3E, 0x2A, 0x87, 0x82, 0x3C, 0xAB,
		0x8F, 0x5E, 0xBF, 0xBF, 0x8E, 0xB1, 0x01, 0x08,
		0x53, 0x50, 0x06, 0x29, 0x8B, 0x5B, 0xAD, 0xBD,
		0x5B, 0x53, 0xE1, 0x89, 0x5E, 0x64, 0x4B, 0x89,
	})
	srpG = big.NewInt(7)
)

// RealmInfo holds info about a realm
type RealmInfo struct {
	Name    string
	Address string
}

// AuthClient handles the authentication protocol
type AuthClient struct {
	conn     net.Conn
	username string
	password string

	sessionKey []byte
}

// NewAuthClient creates a new auth client
func NewAuthClient(username, password string) *AuthClient {
	return &AuthClient{
		username: strings.ToUpper(username),
		password: strings.ToUpper(password),
	}
}

// Authenticate connects to the auth server and performs SRP6 authentication
func (a *AuthClient) Authenticate(authAddr string) ([]RealmInfo, error) {
	var err error
	a.conn, err = net.DialTimeout("tcp", authAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to authserver: %w", err)
	}
	defer a.conn.Close()

	if err := a.conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return nil, err
	}

	// Send logon challenge
	if err := a.sendLogonChallenge(); err != nil {
		return nil, fmt.Errorf("send logon challenge: %w", err)
	}

	// Read logon challenge response
	B, g, N, s, err := a.readLogonChallengeResponse()
	if err != nil {
		return nil, fmt.Errorf("read logon challenge response: %w", err)
	}

	// Compute SRP6 proof
	if err := a.sendLogonProof(B, g, N, s); err != nil {
		return nil, fmt.Errorf("send logon proof: %w", err)
	}

	// Read logon proof response
	if err := a.readLogonProofResponse(); err != nil {
		return nil, fmt.Errorf("read logon proof response: %w", err)
	}

	// Request realm list
	realms, err := a.requestRealmList()
	if err != nil {
		return nil, fmt.Errorf("request realm list: %w", err)
	}

	return realms, nil
}

func (a *AuthClient) sendLogonChallenge() error {
	// FourCC values are sent in reverse byte order per the WoW protocol
	gameName := [4]byte{0, 'W', 'o', 'W'}   // "WoW" reversed
	platform := [4]byte{0, '6', '8', 'x'}   // "x86" reversed
	os := [4]byte{0, 'n', 'i', 'W'}         // "Win" reversed
	country := [4]byte{'S', 'U', 'n', 'e'}  // "enUS" reversed

	loginBytes := []byte(a.username)

	// Build payload
	buf := new(bytes.Buffer)
	buf.WriteByte(AuthLogonChallenge)
	buf.WriteByte(3) // error (protocol version)
	binary.Write(buf, binary.LittleEndian, uint16(30+len(loginBytes)))
	buf.Write(gameName[:])
	buf.WriteByte(3)  // version1
	buf.WriteByte(3)  // version2
	buf.WriteByte(5)  // version3
	binary.Write(buf, binary.LittleEndian, uint16(12340)) // build
	buf.Write(platform[:])
	buf.Write(os[:])
	buf.Write(country[:])
	binary.Write(buf, binary.LittleEndian, uint32(0)) // timezone bias
	binary.Write(buf, binary.LittleEndian, uint32(0x0100007f)) // IP (127.0.0.1)
	buf.WriteByte(byte(len(loginBytes)))
	buf.Write(loginBytes)

	_, err := a.conn.Write(buf.Bytes())
	return err
}

func (a *AuthClient) readLogonChallengeResponse() (B, g, N, s []byte, err error) {
	// Read: cmd(1) + unk(1) + error(1) = 3 bytes
	header := make([]byte, 3)
	if _, err = io.ReadFull(a.conn, header); err != nil {
		return nil, nil, nil, nil, err
	}

	if header[2] != 0 {
		return nil, nil, nil, nil, fmt.Errorf("logon challenge failed with error code %d", header[2])
	}

	// B (32 bytes)
	B = make([]byte, 32)
	if _, err = io.ReadFull(a.conn, B); err != nil {
		return
	}

	// g length (1 byte) + g
	gLen := make([]byte, 1)
	if _, err = io.ReadFull(a.conn, gLen); err != nil {
		return
	}
	g = make([]byte, gLen[0])
	if _, err = io.ReadFull(a.conn, g); err != nil {
		return
	}

	// N length (1 byte) + N
	nLen := make([]byte, 1)
	if _, err = io.ReadFull(a.conn, nLen); err != nil {
		return
	}
	N = make([]byte, nLen[0])
	if _, err = io.ReadFull(a.conn, N); err != nil {
		return
	}

	// s (salt, 32 bytes)
	s = make([]byte, 32)
	if _, err = io.ReadFull(a.conn, s); err != nil {
		return
	}

	// unk3 (16 bytes) + security flags (1 byte)
	trailing := make([]byte, 17)
	if _, err = io.ReadFull(a.conn, trailing); err != nil {
		return
	}

	return
}

func (a *AuthClient) sendLogonProof(serverB, g, N, s []byte) error {
	_B := bigIntFromBytes(serverB)
	_g := bigIntFromBytes(g)
	_N := bigIntFromBytes(N)

	// Generate random a (private key)
	aBytes := make([]byte, 32)
	randBytes(aBytes)
	_a := bigIntFromBytes(aBytes)

	// A = g^a mod N
	_A := new(big.Int).Exp(_g, _a, _N)
	A := bigIntToBytes(_A)

	// u = SHA1(A | B)
	uHash := sha1.Sum(append(A, serverB...))
	_u := bigIntFromBytes(uHash[:])

	// x = SHA1(s | SHA1(username:password))
	credHash := sha1.Sum([]byte(a.username + ":" + a.password))
	xInput := append(s, credHash[:]...)
	xHash := sha1.Sum(xInput)
	_x := bigIntFromBytes(xHash[:])

	// S = (B - 3*g^x mod N)^(a + u*x) mod N
	_gx := new(big.Int).Exp(_g, _x, _N)
	_gx3 := new(big.Int).Mul(big.NewInt(3), _gx)
	_gx3.Mod(_gx3, _N)

	diff := new(big.Int).Sub(_B, _gx3)
	if diff.Sign() < 0 {
		diff.Add(diff, _N)
	}

	exp := new(big.Int).Mul(_u, _x)
	exp.Add(exp, _a)

	_S := new(big.Int).Exp(diff, exp, _N)
	S := bigIntToBytes(_S)

	// Interleave hash S to get K (session key)
	K := sha1Interleave(S)
	a.sessionKey = K

	// Compute M
	// M = SHA1(SHA1(N) XOR SHA1(g), SHA1(username), s, A, B, K)
	nHash := sha1.Sum(bigIntToBytes(_N))
	gHash := sha1.Sum(bigIntToBytes(_g))
	ngHash := [20]byte{}
	for i := 0; i < 20; i++ {
		ngHash[i] = nHash[i] ^ gHash[i]
	}

	usrHash := sha1.Sum([]byte(a.username))

	mInput := make([]byte, 0, 20+20+32+32+32+40)
	mInput = append(mInput, ngHash[:]...)
	mInput = append(mInput, usrHash[:]...)
	mInput = append(mInput, s...)
	mInput = append(mInput, A...)
	mInput = append(mInput, serverB...)
	mInput = append(mInput, K...)
	M := sha1.Sum(mInput)

	// CRC hash (unused, just zero fill or random)
	crcHash := make([]byte, 20)

	// Build proof packet
	buf := new(bytes.Buffer)
	buf.WriteByte(AuthLogonProof)
	buf.Write(A)
	buf.Write(M[:])
	buf.Write(crcHash)
	buf.WriteByte(0) // number of keys
	buf.WriteByte(0) // security flags

	_, err := a.conn.Write(buf.Bytes())
	return err
}

func (a *AuthClient) readLogonProofResponse() error {
	// Read cmd (1) + error (1)
	header := make([]byte, 2)
	if _, err := io.ReadFull(a.conn, header); err != nil {
		return err
	}

	if header[0] != AuthLogonProof {
		return fmt.Errorf("unexpected cmd in logon proof response: 0x%X", header[0])
	}

	if header[1] != 0 {
		return fmt.Errorf("logon proof failed with error code %d", header[1])
	}

	// Success response: M2(20) + AccountFlags(4) + SurveyID(4) + LoginFlags(2) = 30 bytes
	rest := make([]byte, 30)
	if _, err := io.ReadFull(a.conn, rest); err != nil {
		return err
	}

	return nil
}

func (a *AuthClient) requestRealmList() ([]RealmInfo, error) {
	// Send realm list request
	buf := new(bytes.Buffer)
	buf.WriteByte(AuthRealmList)
	binary.Write(buf, binary.LittleEndian, uint32(0))
	if _, err := a.conn.Write(buf.Bytes()); err != nil {
		return nil, err
	}

	// Read response header: cmd(1) + size(2)
	header := make([]byte, 3)
	if _, err := io.ReadFull(a.conn, header); err != nil {
		return nil, err
	}

	size := binary.LittleEndian.Uint16(header[1:3])
	data := make([]byte, size)
	if _, err := io.ReadFull(a.conn, data); err != nil {
		return nil, err
	}

	r := bytes.NewReader(data)

	// padding (4 bytes)
	var padding uint32
	binary.Read(r, binary.LittleEndian, &padding)

	// realm count (2 bytes)
	var count uint16
	binary.Read(r, binary.LittleEndian, &count)

	var realms []RealmInfo
	for i := uint16(0); i < count; i++ {
		// icon (1) + locked (1) + flag (1)
		skip := make([]byte, 3)
		r.Read(skip)

		// name (null-terminated string)
		name := readCString(r)
		// address (null-terminated string)
		address := readCString(r)

		// population(4) + chars(1) + timezone(1) + realmID(1)
		trailer := make([]byte, 7)
		r.Read(trailer)

		realms = append(realms, RealmInfo{
			Name:    name,
			Address: address,
		})
	}

	return realms, nil
}

// SessionKey returns the session key after successful authentication
func (a *AuthClient) SessionKey() []byte {
	return a.sessionKey
}

// Helper functions

func readCString(r io.Reader) string {
	var result []byte
	b := make([]byte, 1)
	for {
		_, err := r.Read(b)
		if err != nil || b[0] == 0 {
			break
		}
		result = append(result, b[0])
	}
	return string(result)
}

func bigIntFromBytes(b []byte) *big.Int {
	// Bytes are in little-endian, need to reverse for big.Int
	reversed := make([]byte, len(b))
	for i := 0; i < len(b); i++ {
		reversed[i] = b[len(b)-1-i]
	}
	return new(big.Int).SetBytes(reversed)
}

func bigIntToBytes(i *big.Int) []byte {
	b := i.Bytes()
	// Reverse to little-endian
	reversed := make([]byte, len(b))
	for j := 0; j < len(b); j++ {
		reversed[j] = b[len(b)-1-j]
	}
	return reversed
}

func sha1Interleave(S []byte) []byte {
	// Ensure S is 32 bytes
	for len(S) < 32 {
		S = append(S, 0)
	}

	buf0 := make([]byte, len(S)/2)
	buf1 := make([]byte, len(S)/2)
	for i := 0; i < len(S)/2; i++ {
		buf0[i] = S[2*i]
		buf1[i] = S[2*i+1]
	}

	// Find first nonzero byte position
	p := 0
	for p < len(S) && S[p] == 0 {
		p++
	}
	if p&1 > 0 {
		p++
	}
	p /= 2

	hash0 := sha1.Sum(buf0[p:])
	hash1 := sha1.Sum(buf1[p:])

	K := make([]byte, 40)
	for i := 0; i < 20; i++ {
		K[2*i] = hash0[i]
		K[2*i+1] = hash1[i]
	}

	return K
}

func randBytes(b []byte) {
	// Use crypto/rand
	crandRead(b)
}
