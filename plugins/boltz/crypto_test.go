package boltz

import (
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/assert"
	"github.com/tyler-smith/go-bip39"
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

func TestHmac(t *testing.T) {
	entropy := make([]byte, 32)
	_, err := rand.Read(entropy)
	assert.NoError(t, err)

	const Max = 100

	m := make(map[string]struct{}, Max)
	for i := 0; i < Max; i++ {
		id := randSeq(10)
		b := DeterministicPreimage(id, entropy)
		assert.Equal(t, 32, len(b))
		result := string(b)
		_, exists := m[result]
		assert.NotEqual(t, true, exists)
		m[result] = struct{}{}
	}
}

func TestGetKeys(t *testing.T) {
	f, err := filter.NewAllowAllFilter()
	assert.NoError(t, err)

	b, err := NewPlugin(getAPI(t, "fixture.secret", api.LndRest), f, getMockCliCtx())
	assert.NoError(t, err)
	_, err = b.GetKeys("aa")
	assert.NoError(t, err)
}

func TestMnemonic(t *testing.T) {
	entropy, err := bip39.NewEntropy(SecretBitSize)
	assert.NoError(t, err)
	mnemonic, err := bip39.NewMnemonic(entropy)
	assert.NoError(t, err)

	a, err := bip39.MnemonicToByteArray(mnemonic, true)
	assert.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(entropy), hex.EncodeToString(a))
	mnemonic2, err := bip39.NewMnemonic(a)
	assert.NoError(t, err)

	assert.Equal(t, mnemonic, mnemonic2)
}
