package srp6

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSRP6VerifyChallengeResponse(t *testing.T) {
	var (
		user     = "ADMIN"
		salt     = []byte{39, 32, 164, 237, 123, 182, 213, 12, 150, 197, 10, 186, 141, 89, 23, 101, 226, 252, 166, 144, 1, 131, 4, 64, 92, 83, 201, 188, 117, 80, 67, 4}
		verifier = []byte{218, 129, 159, 190, 228, 213, 162, 80, 37, 65, 81, 59, 148, 105, 211, 156, 133, 121, 201, 12, 248, 86, 42, 24, 39, 141, 72, 233, 232, 237, 93, 61}
		A        = []byte{121, 83, 168, 57, 8, 98, 11, 103, 71, 189, 75, 152, 0, 119, 196, 218, 186, 114, 69, 107, 250, 94, 144, 57, 140, 22, 255, 93, 101, 62, 232, 66}
		clientM  = []byte{130, 97, 204, 98, 61, 208, 36, 84, 197, 233, 128, 14, 233, 198, 82, 95, 2, 52, 128, 57}
	)

	randBytesProvider = func(bytes []byte) []byte {
		return []byte{55, 32, 242, 56, 169, 54, 240, 113, 125, 40, 199, 165, 13, 232, 179, 155, 50, 170, 39, 210, 181, 252, 114, 188, 218, 210, 140, 194, 239, 71, 142, 45}
	}

	srp := NewSRP(user, salt, verifier)
	r := srp.VerifyChallengeResponse(A, clientM)
	assert.NotNil(t, r)
}

func TestReconnectChallengeValid(t *testing.T) {
	R1 := []byte{233, 248, 19, 122, 141, 205, 37, 95, 0, 231, 254, 63, 7, 128, 206, 157}
	user := "ADMIN"
	reconnectProof := []byte{245, 75, 232, 199, 75, 73, 133, 142, 157, 29, 19, 101, 22, 210, 137, 91}
	sessionKey := []byte{39, 91, 5, 20, 186, 53, 80, 110, 73, 20, 199, 246, 34, 235, 14, 23, 138, 84, 52, 215, 61, 120, 119, 1, 124, 0, 76, 23, 54, 217, 213, 136, 241, 140, 124, 12, 172, 101, 227, 124}
	R2 := []byte{132, 61, 17, 12, 194, 243, 44, 205, 171, 199, 208, 14, 129, 208, 167, 234, 153, 155, 19, 112}
	assert.True(t, ReconnectChallengeValid(user, R1, R2, reconnectProof, sessionKey))
}
