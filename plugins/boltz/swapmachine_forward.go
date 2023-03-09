package boltz

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/bolt-observer/agent/lightning"
	"github.com/btcsuite/btcd/chaincfg"
)

// (submarine) swap finite state machine

func (s *SwapMachine) FsmInitialForward(in FsmIn) FsmOut {
	ctx := context.Background()

	sats := in.SwapData.Sats

	log(in, fmt.Sprintf("Will do a submarine swap with %v sats", sats))

	keys, err := s.BoltzPlugin.CryptoAPI.GetKeys(fmt.Sprintf("%d", in.GetJobID()))
	if err != nil {
		return FsmOut{Error: err}
	}

	pairs, err := s.BoltzPlugin.BoltzAPI.GetPairs()
	if err != nil {
		return FsmOut{Error: err}
	}

	res, ok := pairs.Pairs[BtcPair]
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

	lnConnection, err := s.BoltzPlugin.LnAPI()
	if err != nil {
		return FsmOut{Error: err}
	}
	if lnConnection == nil {
		return FsmOut{Error: fmt.Errorf("error getting lightning API")}
	}

	defer lnConnection.Cleanup()

	invoice, err := lnConnection.CreateInvoice(ctx, int64(in.SwapData.Sats), hex.EncodeToString(keys.Preimage.Raw),
		fmt.Sprintf("Automatic Swap Boltz %d", in.GetJobID()), 24*time.Hour) // assume boltz returns 144 blocks
	if err != nil {
		return FsmOut{Error: err}
	}

	info, err := lnConnection.GetInfo(ctx)
	if err != nil {
		return FsmOut{Error: err}
	}

	response, err := CreateSwapWithSanityCheck(s.BoltzPlugin.BoltzAPI, keys, invoice, info.BlockHeight, s.BoltzPlugin.ChainParams)
	if err != nil {
		return FsmOut{Error: err}
	}

	fee := float64(response.ExpectedAmount-sats+minerFees) / float64(sats)
	if fee*100 > s.BoltzPlugin.MaxFeePercentage {
		return FsmOut{Error: fmt.Errorf("fee was calculated to be %.2f %%, max allowed is %.2f %%", fee*100, s.BoltzPlugin.MaxFeePercentage)}
	}

	log(in, fmt.Sprintf("Swap fee for %v will be approximately %v %%", response.Id, fee*100))

	// Check funds
	funds, err := lnConnection.GetOnChainFunds(ctx)
	if err != nil {
		return FsmOut{Error: err}
	}

	if funds.ConfirmedBalance < int64(response.ExpectedAmount+minerFees) {
		return FsmOut{Error: fmt.Errorf("we have %v sats on-chain but need %v sats", funds.ConfirmedBalance, response.ExpectedAmount+minerFees)}
	}

	in.SwapData.BoltzID = response.Id
	in.SwapData.Script = response.RedeemScript
	in.SwapData.Address = response.Address
	in.SwapData.TimoutBlockHeight = response.TimeoutBlockHeight

	// Explicitly first change state (in case we crash before sending)
	err = s.BoltzPlugin.changeState(in, OnChainFundsSent)
	if err != nil {
		return FsmOut{Error: err}
	}

	tx, err := lnConnection.SendToOnChainAddress(ctx, response.Address, int64(response.ExpectedAmount), false, lightning.Normal)
	if err != nil {
		return FsmOut{Error: err}
	}

	log(in, fmt.Sprintf("Transaction ID for swap is %v (invoice hash %v)", tx, invoice.Hash))
	in.SwapData.LockupTransactionId = tx

	return FsmOut{NextState: OnChainFundsSent}
}

func (s *SwapMachine) FsmOnChainFundsSent(in FsmIn) FsmOut {
	ctx := context.Background()

	SleepTime := s.getSleepTime(in)

	if in.SwapData.BoltzID == "" {
		return FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}
	}

	for {
		lnAPI, err := s.BoltzPlugin.LnAPI()
		if err != nil {
			log(in, fmt.Sprintf("error getting LNAPI: %v", err))
			time.Sleep(SleepTime)
			continue
		}
		if lnAPI == nil {
			log(in, "error getting LNAPI")
			time.Sleep(SleepTime)
			continue
		}

		s.BoltzPlugin.EnsureConnected(ctx, lnAPI)

		s, err := s.BoltzPlugin.BoltzAPI.SwapStatus(in.SwapData.BoltzID)
		if err != nil {
			log(in, fmt.Sprintf("error communicating with BoltzAPI: %v", err))
			time.Sleep(SleepTime)
			continue
		}
		status := boltz.ParseEvent(s.Status)

		if in.SwapData.LockupTransactionId == "" {
			if status == boltz.TransactionMempool || status == boltz.TransactionConfirmed {
				in.SwapData.LockupTransactionId = s.Transaction.Id
			}

			// We are not transitioning back if we crashed before sending
		}

		if status.IsFailedStatus() {
			return FsmOut{NextState: RedeemLockedFunds}
		}

		if status.IsCompletedStatus() || status == boltz.ChannelCreated || status == boltz.InvoicePaid {
			return FsmOut{NextState: VerifyFundsReceived}
		}

		info, err := lnAPI.GetInfo(ctx)
		if err != nil {
			log(in, fmt.Sprintf("error communicating with LNAPI: %v", err))
			time.Sleep(SleepTime)
			continue
		}

		if uint32(info.BlockHeight) > in.SwapData.TimoutBlockHeight {
			return FsmOut{NextState: RedeemLockedFunds}
		}

		lnAPI.Cleanup()
		time.Sleep(SleepTime)
	}
}

func (s *SwapMachine) FsmRedeemLockedFunds(in FsmIn) FsmOut {
	ctx := context.Background()

	if in.SwapData.BoltzID == "" {
		return FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}
	}

	if in.SwapData.LockupTransactionId == "" {
		return FsmOut{Error: fmt.Errorf("invalid state txid not set")}
	}

	SleepTime := s.getSleepTime(in)

	// Wait for expiry
	for {
		lnConnection, err := s.BoltzPlugin.LnAPI()
		if err != nil {
			log(in, fmt.Sprintf("error getting LNAPI: %v", err))
			time.Sleep(SleepTime)
			continue
		}
		if lnConnection == nil {
			log(in, "error getting LNAPI")
			time.Sleep(SleepTime)
			continue
		}

		info, err := lnConnection.GetInfo(ctx)
		if err != nil {
			log(in, fmt.Sprintf("error communicating with LNAPI: %v", err))
			time.Sleep(SleepTime)
			continue
		}

		if uint32(info.BlockHeight) > in.SwapData.TimoutBlockHeight {
			break
		}

		log(in, fmt.Sprintf("Waiting for expiry %d < %d", info.BlockHeight, in.SwapData.TimoutBlockHeight))

		lnConnection.Cleanup()
		time.Sleep(SleepTime)
	}

	return FsmOut{NextState: RedeemingLockedFunds}
}

func (s *SwapMachine) FsmRedeemingLockedFunds(in FsmIn) FsmOut {
	// For state machine this is final state
	if in.SwapData.BoltzID == "" {
		return FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}

	}
	s.BoltzPlugin.Redeemer.AddEntry(in)
	return FsmOut{}
}

func (s *SwapMachine) FsmVerifyFundsReceived(in FsmIn) FsmOut {
	ctx := context.Background()

	if in.SwapData.BoltzID == "" {
		return FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}
	}

	SleepTime := s.getSleepTime(in)

	for {
		lnConnection, err := s.BoltzPlugin.LnAPI()
		if err != nil {
			log(in, fmt.Sprintf("error getting LNAPI: %v", err))
			time.Sleep(SleepTime)
			continue
		}
		if lnConnection == nil {
			log(in, "error getting LNAPI")
			time.Sleep(SleepTime)
			continue
		}

		keys, err := s.BoltzPlugin.CryptoAPI.GetKeys(fmt.Sprintf("%d", in.GetJobID()))
		if err != nil {
			log(in, "error getting keys")
			time.Sleep(SleepTime)
			continue
		}

		paid, err := lnConnection.IsInvoicePaid(ctx, hex.EncodeToString(keys.Preimage.Hash))
		if err != nil {
			log(in, "error checking whether invoice is paid")
			time.Sleep(SleepTime)
			continue
		}

		if paid {
			return FsmOut{NextState: SwapSuccess}
		} else {
			return FsmOut{NextState: SwapFailed}
		}
	}
}

func CreateSwapWithSanityCheck(api *boltz.Boltz, keys *Keys, invoice *lightning.InvoiceResp, currentBlockHeight int, chainparams *chaincfg.Params) (*boltz.CreateSwapResponse, error) {
	const BlockEps = 10

	response, err := api.CreateSwap(boltz.CreateSwapRequest{
		Type:            "submarine",
		PairId:          BtcPair,
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

	if currentBlockHeight+BlockEps > int(response.TimeoutBlockHeight) {
		return nil, fmt.Errorf("error checking blockheight - current %v vs. timeout %v", currentBlockHeight, int(response.TimeoutBlockHeight))
	}

	return response, nil
}
