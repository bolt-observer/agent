package boltz

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/bolt-observer/agent/lightning"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/golang/glog"
)

// Normal (submarine) swap finite state machine

func FsmInitialNormal(in FsmIn) FsmOut {
	ctx := context.Background()

	sats := in.SwapData.Sats

	keys, err := in.BoltzPlugin.GetKeys(fmt.Sprintf("%d", in.GetJobID()))
	if err != nil {
		return FsmOut{Error: err}
	}

	pairs, err := in.BoltzPlugin.BoltzAPI.GetPairs()
	if err != nil {
		return FsmOut{Error: err}
	}

	res, ok := pairs.Pairs["BTC/BTC"]
	if !ok {
		return FsmOut{Error: fmt.Errorf("pairs are not available")}
	}

	if sats < res.Limits.Minimal {
		return FsmOut{Error: fmt.Errorf("below minimum amount")}
	}

	if sats > res.Limits.Maximal {
		return FsmOut{Error: fmt.Errorf("above maximum amount")}
	}

	// times 2 is used as a safety margin
	minerFees := 2 * res.Fees.MinerFees.BaseAsset.Normal

	lnAPI, err := in.BoltzPlugin.LnAPI()
	if err != nil {
		return FsmOut{Error: err}
	}
	if lnAPI == nil {
		return FsmOut{Error: fmt.Errorf("error getting lightning API")}
	}

	defer lnAPI.Cleanup()

	invoice, err := lnAPI.CreateInvoice(ctx, int64(in.SwapData.Sats), hex.EncodeToString(keys.Preimage.Hash),
		fmt.Sprintf("Automatic Swap Boltz %d", in.GetJobID()), 24*time.Hour)
	if err != nil {
		return FsmOut{Error: err}
	}

	info, err := lnAPI.GetInfo(ctx)
	if err != nil {
		return FsmOut{Error: err}
	}

	response, err := CreateSwapWithSanityCheck(in.BoltzPlugin.BoltzAPI, keys, invoice, info.BlockHeight, in.BoltzPlugin.ChainParams)
	if err != nil {
		return FsmOut{Error: err}
	}

	fee := float64(response.ExpectedAmount-sats+minerFees) / float64(sats)
	if fee/100 > in.BoltzPlugin.MaxFeePercentage {
		return FsmOut{Error: fmt.Errorf("fee was calculated to be %v, max allowed is %v", fee/100, in.BoltzPlugin.MaxFeePercentage)}
	}

	funds, err := lnAPI.GetOnChainFunds(ctx)
	if err != nil {
		return FsmOut{Error: err}
	}

	if funds.ConfirmedBalance < int64(response.ExpectedAmount+minerFees) {
		return FsmOut{Error: fmt.Errorf("we have %v sats on-chain but need %v", funds.ConfirmedBalance, response.ExpectedAmount+minerFees)}
	}

	in.SwapData.BoltzID = response.Id
	in.SwapData.Script = response.RedeemScript
	in.SwapData.Address = response.Address
	in.SwapData.TimoutBlockHeight = response.TimeoutBlockHeight

	changeState(in, OnChainFundsSent)

	tx, err := lnAPI.SendToOnChainAddress(ctx, response.Address, int64(response.ExpectedAmount), false, lightning.Normal)
	if err != nil {
		return FsmOut{Error: err}
	}
	in.SwapData.LockupTransactionId = tx

	return FsmOut{NextState: OnChainFundsSent}
}

func CreateSwapWithSanityCheck(api *boltz.Boltz, keys *Keys, invoice *lightning.InvoiceResp, currentBlockHeight int, chainparams *chaincfg.Params) (*boltz.CreateSwapResponse, error) {
	const BlockEps = 10

	response, err := api.CreateSwap(boltz.CreateSwapRequest{
		Type:            "submarine",
		PairId:          "BTC/BTC",
		OrderSide:       "buy",
		PreimageHash:    hex.EncodeToString(keys.Preimage.Hash),
		RefundPublicKey: hex.EncodeToString(keys.Keys.PublicKey.SerializeCompressed()),
		Invoice:         invoice.PaymentRequest,
	})

	if err != nil {
		return nil, err
	}

	redeemScript, err := hex.DecodeString(response.RedeemScript)
	if err != nil {
		return nil, err
	}

	err = boltz.CheckSwapScript(redeemScript, keys.Preimage.Hash, keys.Keys.PrivateKey, response.TimeoutBlockHeight)
	if err != nil {
		return nil, err
	}

	err = boltz.CheckSwapAddress(chainparams, response.Address, redeemScript, true)
	if err != nil {
		return nil, err
	}

	if currentBlockHeight+BlockEps < int(response.TimeoutBlockHeight) {
		return nil, fmt.Errorf("error checking blockheight")
	}

	return response, nil
}

func FsmOnChainFundsSent(in FsmIn) FsmOut {
	ctx := context.Background()
	const SleepTime = 5 * time.Minute

	if in.SwapData.BoltzID == "" {
		return FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}
	}

	for {
		lnAPI, err := in.BoltzPlugin.LnAPI()
		if err != nil {
			glog.Warningf("error gettings LNAPI: %v", err)
			time.Sleep(SleepTime)
			continue
		}
		if lnAPI == nil {
			glog.Warning("error getting LNAPI")
			time.Sleep(SleepTime)
			continue
		}

		status, err := in.BoltzPlugin.BoltzAPI.SwapStatus(in.SwapData.BoltzID)
		if err != nil {
			glog.Warningf("error communicating with BoltzAPI: %v", err)
			time.Sleep(SleepTime)
			continue
		}

		if in.SwapData.LockupTransactionId == "" {
			if status.Status == "transaction.mempool" || status.Status == "transaction.confirmed" {
				in.SwapData.LockupTransactionId = status.Transaction.Id
			}
		}

		if status.Status == "swap.expired" || status.Status == "invoice.failedToPay" {
			return FsmOut{NextState: RedeemLockedFunds}
		}

		if status.Status == "transaction.claimed" || status.Status == "channel.created" || status.Status == "invoice.paid" {
			return FsmOut{NextState: VerifyFundsReceived}
		}

		info, err := lnAPI.GetInfo(ctx)
		if err != nil {
			glog.Warningf("error communicating with LNAPI: %v", err)
			time.Sleep(SleepTime)
		}

		if uint32(info.BlockHeight) > in.SwapData.TimoutBlockHeight {
			return FsmOut{NextState: RedeemLockedFunds}
		}

		lnAPI.Cleanup()
		time.Sleep(SleepTime)
	}
}

func FsmRedeemLockedFunds(in FsmIn) FsmOut {
	return FsmOut{NextState: InitialNormal}
}

func FsmRedeemingLockedFunds(in FsmIn) FsmOut {
	return FsmOut{NextState: InitialNormal}
}

func FsmVerifyFundsReceived(in FsmIn) FsmOut {
	return FsmOut{NextState: InitialNormal}
}
