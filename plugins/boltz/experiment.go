package boltz

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"

	//bolt "github.com/timshannon/bolthold"
	"github.com/tyler-smith/go-bip39"
)

// BitSize is a default.
const BitSize = 256

// Burek is delicious.
func Burek() {
	entropy, err := bip39.NewEntropy(BitSize)
	if err != nil {
		return
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return
	}

	// func EntropyFromMnemonic(mnemonic string) ([]byte, error) {

	privKey, pubKey := btcec.PrivKeyFromBytes(entropy)

	fmt.Printf("%s vs %s\n", hex.EncodeToString(pubKey.SerializeCompressed()), hex.EncodeToString(privKey.PubKey().SerializeCompressed()))

	fmt.Printf("Deterministic pubkey #1 %s\n", hex.EncodeToString(DeterministicPrivateKey("burek2", privKey).PubKey().SerializeCompressed()))
	fmt.Printf("Deterministic pubkey #2 %s\n", hex.EncodeToString(DeterministicPublicKey("burek2", pubKey).SerializeCompressed()))

	addr, err := btcutil.NewAddressWitnessPubKeyHash(
		btcutil.Hash160(pubKey.SerializeCompressed()),
		&chaincfg.MainNetParams,
	)
	if err != nil {
		return
	}

	fmt.Printf("%s %+v %s\n", mnemonic, privKey, addr.EncodeAddress())

	addr2, err := btcutil.NewAddressPubKeyHash(
		btcutil.Hash160(pubKey.SerializeCompressed()),
		&chaincfg.MainNetParams,
	)
	if err != nil {
		return
	}

	fmt.Printf("%s\n", addr2.EncodeAddress())


	return
	/*
		//&chaincfg.MainNetParams
		// Decode the hex-encoded private key.
		pkBytes, err := hex.DecodeString("a11b0a4e1a132305652ee7a8eb7848f6ad" +
			"5ea381e3ce20a2c086a2e388230811")
		if err != nil {
			fmt.Println(err)
			return
		}

		privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), pkBytes)

		witnessProg := btcutil.Hash160(pubkey.SerializeCompressed())
		addressWitnessPubKeyHash, err := btcutil.NewAddressWitnessPubKeyHash(witnessProg, chainParams)
		if err != nil {
			panic(err)
		}
		address := addressWitnessPubKeyHash.EncodeAddress()
	*/
}
