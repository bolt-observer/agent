package boltz

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
)

// CalcFundsToReceive calculates how much funds you would receive
func (b *Plugin) CalcFundsToReceive(ctx context.Context, reverse bool, sats uint64) (uint64, int64, error) {
	if b.LnAPI == nil {
		return 0, 0, fmt.Errorf("lightning api is not available")
	}
	lnAPI, err := b.LnAPI()
	if err != nil {
		return 0, 0, err
	}

	if !reverse {
		// normal submarine requires on-chain funds
		funds, err := lnAPI.GetOnChainFunds(ctx)
		if err != nil {
			return 0, 0, err
		}

		bal := uint64(funds.ConfirmedBalance)
		if bal < sats {
			return 0, int64(bal - sats), fmt.Errorf("insufficient on-chain funds")
		}
	} else {
		// reverse submarine requires huge outbound liquidity
		liq, err := b.GetNodeLiquidity(ctx)
		if err != nil {
			return 0, 0, err
		}

		if liq.OutboundSats < sats {
			return 0, int64(liq.OutboundSats - sats), fmt.Errorf("insufficent off-chain funds")
		}
	}

	resp, err := b.BoltzAPI.GetPairs()
	if err != nil {
		return 0, 0, err
	}

	res, ok := resp.Pairs["BTC/BTC"]
	if !ok {
		return 0, 0, fmt.Errorf("pairs are not available")
	}

	fmt.Printf("Pairs %+v\n", res)

	if sats < res.Limits.Minimal {
		return 0, int64(res.Limits.Minimal - sats), fmt.Errorf("below minimum amount")
	}

	if sats > res.Limits.Maximal {
		return 0, int64(res.Limits.Maximal - sats), fmt.Errorf("above maximum amount")
	}

	amt := float64(0)
	if !reverse {
		// normal swap - first pay mines (- fees) then apply percentage cut
		fees := float64(res.Fees.MinerFees.BaseAsset.Normal)
		amt = math.Round(float64(sats)-fees) * (1 - float64(res.Fees.Percentage))
	} else {
		// reverse swap
		fees := float64(res.Fees.MinerFees.BaseAsset.Reverse.Lockup)
		amt = math.Round(float64(sats)*(1-float64(res.Fees.Percentage))) - fees
	}

	return uint64(amt), 0, nil
}

// Swap - does the submarine swap - PoC
func (b *Plugin) Swap(id string) error {
	const BlockEps = 10

	keys, err := b.GetKeys(id)
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

	ctx := context.Background()
	info, err := lnAPI.GetInfo(ctx)
	if err != nil {
		fmt.Printf("Info error: %v", err)
		return err
	}

	fmt.Printf("Doing swap from %s\n", info.Alias)

	invoice, err := lnAPI.CreateInvoice(ctx, 40000, hex.EncodeToString(keys.Preimage.Hash), "", 24*time.Hour)
	if err != nil {
		fmt.Printf("Invoice error: %v", err)
		return err
	}

	fmt.Printf("Invoice %s %s\n", invoice.Hash, invoice.PaymentRequest)

	response, err := b.BoltzAPI.CreateSwap(boltz.CreateSwapRequest{
		Type:            "submarine",
		PairId:          "BTC/BTC",
		OrderSide:       "buy",
		PreimageHash:    hex.EncodeToString(keys.Preimage.Hash),
		RefundPublicKey: hex.EncodeToString(keys.Keys.PublicKey.SerializeCompressed()),
		Invoice:         invoice.PaymentRequest,
	})

	if err != nil {
		fmt.Printf("Error creating swap: %v", err)
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

	if info.BlockHeight+BlockEps < int(response.TimeoutBlockHeight) {
		return fmt.Errorf("error checking blockheight")
	}

	fmt.Printf("ID %v\n", response.Id)

	return nil
}
