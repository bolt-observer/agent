package boltz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/btcsuite/btcd/btcec/v2"
)

// Swap - does the submarine swap - PoC
func (b *Plugin) Swap(id string) error {
	const BlockEps = 10

	preimage := DeterministicPreimage(id, b.MasterSecret)
	hash := sha256.Sum256(preimage)
	preimageHash := hash[:]
	origPriv, _ := btcec.PrivKeyFromBytes(b.MasterSecret)
	priv := DeterministicPrivateKey(id, origPriv)

	response, err := b.BoltzAPI.CreateSwap(boltz.CreateSwapRequest{
		Type:            "submarine",
		PairId:          "BTC/BTC",
		OrderSide:       "buy",
		PreimageHash:    hex.EncodeToString(preimageHash),
		RefundPublicKey: hex.EncodeToString(priv.PubKey().SerializeCompressed()),
	})

	if err != nil {
		return err
	}

	fmt.Printf("Response: %+v\n", response)

	redeemScript, err := hex.DecodeString(response.RedeemScript)
	if err != nil {
		return err
	}

	fmt.Printf("Timeout %v\n", response.TimeoutBlockHeight)

	err = boltz.CheckSwapScript(redeemScript, preimageHash, priv, response.TimeoutBlockHeight)
	if err != nil {
		return err
	}

	err = boltz.CheckSwapAddress(b.ChainParams, response.Address, redeemScript, true)
	if err != nil {
		return err
	}

	l := b.LnAPI()
	if l == nil {
		return fmt.Errorf("error checkig lightning")
	}
	defer l.Cleanup()
	resp, err := l.GetInfo(context.Background())
	if err != nil || resp.BlockHeight+BlockEps < int(response.TimeoutBlockHeight) {
		return fmt.Errorf("error checking blockheight")
	}

	fmt.Printf("ID %v\n", response.Id)

	return nil
}
