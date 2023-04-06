//go:build plugins
// +build plugins

package crypto

import (
	"encoding/hex"
	"math/rand"
	"testing"

	common "github.com/bolt-observer/agent/plugins/boltz/common"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/assert"
	"github.com/tyler-smith/go-bip39"
)

func TestCrypto(t *testing.T) {
	const Max = 10000
	for i := 0; i < Max; i++ {
		entropy := make([]byte, 32)
		_, err := rand.Read(entropy)
		assert.NoError(t, err)

		privKey, pubKey := btcec.PrivKeyFromBytes(entropy)
		id := RandSeq(10)
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
		id := RandSeq(10)
		b := DeterministicPreimage(id, entropy)
		assert.Equal(t, 32, len(b))
		result := string(b)
		_, exists := m[result]
		assert.NotEqual(t, true, exists)
		m[result] = struct{}{}
	}
}

func TestGetKeys(t *testing.T) {
	entropy, err := bip39.NewEntropy(common.SecretBitSize)
	c := NewCryptoAPI(entropy)

	assert.NoError(t, err)
	_, err = c.GetKeys("aa")
	assert.NoError(t, err)
}

func TestMnemonic(t *testing.T) {
	entropy, err := bip39.NewEntropy(common.SecretBitSize)
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
