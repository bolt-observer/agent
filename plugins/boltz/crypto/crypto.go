//go:build plugins
// +build plugins

package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"math/rand"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/golang/glog"
	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
)


// DeterministicPrivateKey - from private key x generated x + hash(id)
func DeterministicPrivateKey(id string, orig *secp256k1.PrivateKey) *secp256k1.PrivateKey {
	hash := sha256.Sum256([]byte(id))

	scalar := &secp256k1.ModNScalar{}
	scalar.SetBytes(&hash)

	return secp256k1.NewPrivateKey(orig.Key.Add(scalar))
}

// DeterministicPublicKey - from public key A generate A + hash(id)*G
func DeterministicPublicKey(id string, orig *secp256k1.PublicKey) *secp256k1.PublicKey {
	curve := secp256k1.S256()

	hash := sha256.Sum256([]byte(id))
	scalar := &secp256k1.ModNScalar{}
	scalar.SetBytes(&hash)
	bytes := scalar.Bytes()

	x, y := curve.ScalarBaseMult(bytes[:])
	x2, y2 := curve.Add(orig.X(), orig.Y(), x, y)

	var (
		fx secp256k1.FieldVal
		fy secp256k1.FieldVal
	)
	fx.SetByteSlice(x2.Bytes())
	fy.SetByteSlice(y2.Bytes())

	return secp256k1.NewPublicKey(&fx, &fy)
}

// DeterministicPreimage - from key generate a hmac derivation with id
func DeterministicPreimage(id string, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(id))
	r := h.Sum(nil)
	return r
}

// Preimage struct.
type Preimage struct {
	Raw  []byte
	Hash []byte
}

type AsymmetricKeys struct {
	PrivateKey *secp256k1.PrivateKey
	PublicKey  *secp256k1.PublicKey
}

// Keys struct.
type Keys struct {
	Preimage Preimage
	Keys     AsymmetricKeys
}

type CryptoAPI struct {
	MasterSecret []byte
}

func NewCryptoAPI(masterSecret []byte) *CryptoAPI {
	return &CryptoAPI{MasterSecret: masterSecret}
}

// GetKeys
func (b *CryptoAPI) GetKeys(id string) (*Keys, error) {
	mnemonic, err := bip39.NewMnemonic(b.MasterSecret)
	if err != nil {
		return nil, err
	}

	seed := bip39.NewSeed(mnemonic, "")
	master, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, err
	}

	preimage_seed, err := master.NewChildKey(1)
	if err != nil {
		return nil, err
	}

	result := &Keys{}
	result.Preimage = Preimage{}

	preimage := DeterministicPreimage(id, preimage_seed.Key)
	hash := sha256.Sum256(preimage)
	preimageHash := hash[:]
	result.Preimage.Raw = preimage
	result.Preimage.Hash = preimageHash

	key_seed, err := master.NewChildKey(2)
	if err != nil {
		return nil, err
	}

	origPriv, _ := btcec.PrivKeyFromBytes(key_seed.Key)
	priv := DeterministicPrivateKey(id, origPriv)

	result.Keys = AsymmetricKeys{}
	result.Keys.PrivateKey = priv
	result.Keys.PublicKey = priv.PubKey()

	return result, nil
}

func (b *CryptoAPI) DumpMnemonic() string {
	mnemonic, err := bip39.NewMnemonic(b.MasterSecret)
	if err != nil {
		glog.Warningf("DumpMnemonic failed %v", err)
		return ""
	}

	return mnemonic
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
