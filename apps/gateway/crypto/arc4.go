package crypto

import (
	"crypto/hmac"
	"crypto/rc4"
	"crypto/sha1"
)

type Arc struct {
	server *rc4.Cipher
	client *rc4.Cipher
}

func NewArc(sessionKey []byte) (*Arc, error) {
	serverEncryptionKey := []byte{0xCC, 0x98, 0xAE, 0x04, 0xE8, 0x97, 0xEA, 0xCA, 0x12, 0xDD, 0xC0, 0x93, 0x42, 0x91, 0x53, 0x57}
	k := hmacSha1Hash(serverEncryptionKey, sessionKey)
	server, err := rc4.NewCipher(k)
	if err != nil {
		return nil, err
	}

	clientEncryptionKey := []byte{0xC2, 0xB3, 0x72, 0x3C, 0xC6, 0xAE, 0xD9, 0xB5, 0x34, 0x3C, 0x53, 0xEE, 0x2F, 0x43, 0x67, 0xCE}
	client, err := rc4.NewCipher(hmacSha1Hash(clientEncryptionKey, sessionKey))
	if err != nil {
		return nil, err
	}

	// Drop first 1024 bytes, as WoW uses ARC4-drop1024.
	syncBuf := make([]byte, 1024)
	server.XORKeyStream(syncBuf, syncBuf)
	syncBuf = make([]byte, 1024)
	client.XORKeyStream(syncBuf, syncBuf)

	return &Arc{
		server: server,
		client: client,
	}, nil
}

func (a *Arc) Encrypt(d []byte) {
	a.server.XORKeyStream(d, d)
}

func (a *Arc) Decrypt(d []byte) {
	a.client.XORKeyStream(d, d)
}

func hmacSha1Hash(k, m []byte) []byte {
	mac := hmac.New(sha1.New, k)
	mac.Write(m)
	return mac.Sum(nil)
}
