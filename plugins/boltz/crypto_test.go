package boltz

import (
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/assert"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func TestCrypto(t *testing.T) {
	const Max = 10000
	for i := 0; i < Max; i++ {
		entropy := make([]byte, 32)
		_, err := rand.Read(entropy)
		assert.NoError(t, err)

		privKey, pubKey := btcec.PrivKeyFromBytes(entropy)
		id := randSeq(10)
		fromPriv := hex.EncodeToString(DeterministicPrivateKey(id, privKey).PubKey().SerializeCompressed())
		fromPub := hex.EncodeToString(DeterministicPublicKey(id, pubKey).SerializeCompressed())

		assert.Equal(t, fromPriv, fromPub)
	}
}
