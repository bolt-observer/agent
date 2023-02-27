package boltz

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/BoltzExchange/boltz-lnd/boltz"
)

// Swap - does the submarine swap - PoC
func (b *Plugin) Swap(id string) error {
	const BlockEps = 10

	keys, err := b.GetKeys(id)
	if err != nil {
		return err
	}

	response, err := b.BoltzAPI.CreateSwap(boltz.CreateSwapRequest{
		Type:            "submarine",
		PairId:          "BTC/BTC",
		OrderSide:       "buy",
		PreimageHash:    hex.EncodeToString(keys.Preimage.Hash),
		RefundPublicKey: hex.EncodeToString(keys.Keys.PublicKey.SerializeCompressed()),
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

	err = boltz.CheckSwapScript(redeemScript, keys.Preimage.Hash, keys.Keys.PrivateKey, response.TimeoutBlockHeight)
	if err != nil {
		return err
	}

	err = boltz.CheckSwapAddress(b.ChainParams, response.Address, redeemScript, true)
	if err != nil {
		return err
	}

	lnAPI, err := b.LnAPI()
	if err != nil {
		return err
	}
	if lnAPI == nil {
		return fmt.Errorf("error checking lightning")
	}
	defer lnAPI.Cleanup()
	resp, err := lnAPI.GetInfo(context.Background())
	if err != nil || resp.BlockHeight+BlockEps < int(response.TimeoutBlockHeight) {
		return fmt.Errorf("error checking blockheight")
	}

	fmt.Printf("ID %v\n", response.Id)

	return nil
}
