package srp6

import (
	"crypto/rand"
	"crypto/sha1"
	"math/big"

	"github.com/walkline/ToCloud9/shared/slices"
)

var _N = big.NewInt(0).
	SetBytes([]byte{
		137, 75, 100, 94, 137, 225, 83, 91, 189, 173, 91, 139, 41, 6, 80, 83,
		8, 1, 177, 142, 191, 191, 94, 143, 171, 60, 130, 135, 42, 62, 155, 183,
	})
var _g = big.NewInt(7)

var randBytesProvider = randCryptoBytesProvider

func GetSessionVerifier(A, clientM, K []byte) [sha1.Size]byte {
	return sha1.Sum(slices.AppendBytes(A, clientM, K))
}

type SRP6 struct {
	_I []byte
	_b *big.Int
	_v *big.Int
	_s []byte
	_B []byte
}

func (s *SRP6) DataForClient() (B []byte, g []byte, N []byte, _s []byte) {
	B, g, N, _s = s._B, bigIntToBytesLittleEndian(_g), bigIntToBytesLittleEndian(_N), s._s
	return
}

func NewSRP(username string, salt []byte, verifier []byte) *SRP6 {
	v := bigIntFromLittleEndian(verifier)

	b := make([]byte, 32)
	b = randBytesProvider(b)
	_b := bigIntFromLittleEndian(b)

	usrHash := sha1.Sum([]byte(username))

	return &SRP6{
		_I: usrHash[:],
		_v: v,
		_b: _b,
		_s: salt,
		_B: _B(_b, v),
	}
}

func (s *SRP6) sha1Interleave(S []byte) []byte {
	// split S into two buffers
	buf0 := make([]byte, len(S)/2)
	buf1 := make([]byte, len(S)/2)
	for i := 0; i < len(S)/2; i++ {
		buf0[i] = S[2*i+0]
		buf1[i] = S[2*i+1]
	}

	// find position of first nonzero byte
	p := 0
	for ; p < len(S) && S[p] == 0; p++ {
	}

	// skip one extra byte if p is odd
	if p&1 > 0 {
		p++
	}

	// offset into buffers
	p /= 2

	// hash each of the halves, starting at the first nonzero byte
	hash0 := sha1.Sum(buf0[p:])
	hash1 := sha1.Sum(buf1[p:])

	// stick the two hashes back together
	K := make([]byte, sha1.Size*2)
	for i := 0; i < sha1.Size; i++ {
		K[2*i+0] = hash0[i]
		K[2*i+1] = hash1[i]
	}

	return K
}

func (s *SRP6) VerifyChallengeResponse(A []byte, clientM []byte) []byte {
	_A := bigIntFromLittleEndian(A)
	if big.NewInt(0).Mod(_A, _N).Uint64() == 0 {
		return nil
	}

	ABHash := sha1.Sum(append(A, s._B...))
	u := bigIntFromLittleEndian(ABHash[:])

	// (_A * (_v.ModExp(u, _N))).ModExp(_b, N)
	SBig := big.NewInt(0).Exp(
		big.NewInt(0).Mul(
			_A,
			big.NewInt(0).Exp(s._v, u, _N),
		),
		s._b,
		_N,
	)

	S := bigIntToBytesLittleEndian(SBig)
	K := s.sha1Interleave(S)

	NHash := sha1.Sum(bigIntToBytesLittleEndian(_N))
	gHash := sha1.Sum(bigIntToBytesLittleEndian(_g))

	NgHash := [sha1.Size]byte{}
	for i := range NgHash {
		NgHash[i] = NHash[i] ^ gHash[i]
	}

	ourM := sha1.Sum(slices.AppendBytes(NgHash[:], s._I, s._s, A, s._B, K))

	if slices.SameBytes(ourM[:], clientM) {
		return K
	}

	return nil
}

// ((_g.ModExp(b,_N) + (v * 3)) % N)
func _B(b, v *big.Int) []byte {
	v3 := big.NewInt(0).Mul(v, big.NewInt(3))

	gbnExp := big.Int{}
	gbnExp.Exp(_g, b, _N)

	gbnExpPlusV3 := big.Int{}
	gbnExpPlusV3.Add(&gbnExp, v3)

	result := big.Int{}
	result.Mod(&gbnExpPlusV3, _N)

	return bigIntToBytesLittleEndian(&result)
}

func ReconnectChallengeValid(username string, R1, R2, reconnectProof, K []byte) bool {
	hash := sha1.Sum(slices.AppendBytes([]byte(username), R1, reconnectProof, K))
	return slices.SameBytes(R2, hash[:])
}

func switchEndian(b []byte) []byte {
	r := make([]byte, len(b))
	copy(r, b)
	for i := 0; i < len(b)/2; i++ {
		r[i], r[len(b)-i-1] = b[len(b)-i-1], b[i]
	}
	return r
}

func bigIntToBytesLittleEndian(i *big.Int) []byte {
	return switchEndian(i.Bytes())
}

func bigIntFromLittleEndian(b []byte) *big.Int {
	u := &big.Int{}
	u.SetBytes(switchEndian(b))
	return u
}

func randCryptoBytesProvider(b []byte) []byte {
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}
