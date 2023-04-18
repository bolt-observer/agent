//go:build plugins
// +build plugins

package swapmachine

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/bolt-observer/agent/entities"
	bapi "github.com/bolt-observer/agent/plugins/boltz/api"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	crypto "github.com/bolt-observer/agent/plugins/boltz/crypto"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/golang/glog"
	"github.com/lightningnetwork/lnd/zpay32"
)

// Reverse (submarine) swap finite state machine

func (s *SwapMachine) FsmInitialReverse(in common.FsmIn) common.FsmOut {
	ctx := context.Background()
	logger := NewLogEntry(in.SwapData)
	const SafetyMargin = 1000 // sats

	sats := in.SwapData.Sats

	log(in, fmt.Sprintf("Will do a reverse submarine swap with %v sats", sats),
		logger.Get("sats", sats))

	keys, err := s.CryptoAPI.GetKeys(in.GetUniqueJobID())
	if err != nil {
		return common.FsmOut{Error: err}
	}

	pairs, err := s.BoltzAPI.GetPairs()
	if err != nil {
		return common.FsmOut{Error: err}
	}

	res, ok := pairs.Pairs[common.BtcPair]
	if !ok {
		return common.FsmOut{Error: fmt.Errorf("pairs are not available")}
	}

	if sats < res.Limits.Minimal {
		return common.FsmOut{Error: fmt.Errorf("below minimum amount")}
	}

	if sats > res.Limits.Maximal {
		return common.FsmOut{Error: fmt.Errorf("above maximum amount")}
	}

	// times 2 is used as a safety margin
	minerFees := 2 * res.Fees.MinerFees.BaseAsset.Reverse.Claim

	lnConnection, err := s.LnAPI()
	if err != nil {
		return common.FsmOut{Error: err}
	}
	if lnConnection == nil {
		return common.FsmOut{Error: fmt.Errorf("error getting lightning API")}
	}

	defer lnConnection.Cleanup()

	info, err := lnConnection.GetInfo(ctx)
	if err != nil {
		return common.FsmOut{Error: err}
	}

	response, err := CreateReverseSwapWithSanityCheck(s.BoltzAPI, keys, sats, s.ReferralCode, info.BlockHeight, s.ChainParams)
	if err != nil {
		return common.FsmOut{Error: err}
	}

	fee := float64(sats-response.OnchainAmount+minerFees+SafetyMargin) / float64(sats)
	if fee*100 > in.SwapData.SwapLimits.MaxFeePercentage {
		return common.FsmOut{Error: fmt.Errorf("fee was calculated to be %.2f %%, max allowed is %.2f %%", fee*100, in.SwapData.SwapLimits.MaxFeePercentage)}
	}

	totalFee := float64(in.SwapData.FeesSoFar.FeesPaid+(sats-response.OnchainAmount)) / float64(in.SwapData.FeesSoFar.SatsSwapped+sats) * 100
	if totalFee > in.SwapData.SwapLimits.MaxFeePercentage {
		return common.FsmOut{Error: fmt.Errorf("total fee was calculated to be %.2f %%, max allowed is %.2f %%", totalFee, in.SwapData.SwapLimits.MaxFeePercentage)}
	}

	log(in, fmt.Sprintf("Swap fee for %v will be approximately %v %%", response.Id, fee*100),
		logger.Get("fee", fee*100))

	in.SwapData.FeesPending = common.Fees{
		FeesPaid:    (sats - response.OnchainAmount),
		SatsSwapped: sats,
	}

	// Check funds
	if in.SwapData.ReverseChannelId == 0 {
		capacity, err := common.GetByDescendingOutboundLiquidity(ctx, sats+SafetyMargin, lnConnection, s.Filter)
		if err != nil {
			return common.FsmOut{Error: err}
		}
		if len(capacity) <= 0 {
			return common.FsmOut{Error: fmt.Errorf("invalid capacities")}
		}

		chans := make([]uint64, 0)
		for _, one := range capacity {
			chans = append(chans, one.Channel.ChanID)
		}

		in.SwapData.ChanIdsToUse = chans
	} else {
		// Will error when sufficient funds are not available
		_, _, err = common.GetChanLiquidity(ctx, in.SwapData.ReverseChannelId, sats+SafetyMargin, true, lnConnection, s.Filter)
		if err != nil {
			return common.FsmOut{Error: err}
		}

		chans := make([]uint64, 0)
		chans = append(chans, in.SwapData.ReverseChannelId)
		in.SwapData.ChanIdsToUse = chans
	}

	in.SwapData.BoltzID = response.Id
	in.SwapData.ReverseInvoice = response.Invoice
	in.SwapData.Script = response.RedeemScript
	in.SwapData.TimoutBlockHeight = response.TimeoutBlockHeight
	in.SwapData.ExpectedSats = response.OnchainAmount

	return common.FsmOut{NextState: common.ReverseSwapCreated}
}

func (s *SwapMachine) FsmReverseSwapCreated(in common.FsmIn) common.FsmOut {
	const PayRetries = 5

	ctx := context.Background()
	logger := NewLogEntry(in.SwapData)
	paid := false

	SleepTime := s.GetSleepTimeFn(in)

	if in.SwapData.BoltzID == "" {
		return common.FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}
	}

	if in.SwapData.IsDryRun {
		return common.FsmOut{NextState: common.SwapSuccess}
	}

	payAttempt := 0

	for {
		lnConnection, err := s.LnAPI()
		if err != nil {
			log(in, fmt.Sprintf("error getting LNAPI: %v", err), logger.Get("error", err.Error()))
			time.Sleep(SleepTime)
			continue
		}
		if lnConnection == nil {
			log(in, "error getting LNAPI", logger.Get("error", "error getting LNAPI"))
			time.Sleep(SleepTime)
			continue
		}

		if !paid {
			payAttempt++
			log(in, fmt.Sprintf("Paying invoice %v %+v", in.SwapData.ReverseInvoice, in.SwapData.ChanIdsToUse),
				logger.Get("invoice", in.SwapData.ReverseInvoice))

			_, err = lnConnection.PayInvoice(ctx, in.SwapData.ReverseInvoice, 0, in.SwapData.ChanIdsToUse)
			if err != nil {
				log(in, fmt.Sprintf("Failed paying invoice %v due to %v", in.SwapData.ReverseInvoice, err), logger.Get("error", err.Error()))
				if in.SwapData.ReverseChannelId == 0 {
					// this means node level liquidity - if the hints worked that would be nice, but try without them too

					_, err = lnConnection.PayInvoice(ctx, in.SwapData.ReverseInvoice, 0, nil)
					if err == nil {
						paid = true
						in.SwapData.CommitFees()
					}
				}
			} else {
				paid = true
				in.SwapData.CommitFees()
			}

			if !paid && payAttempt > PayRetries {
				return common.FsmOut{NextState: common.SwapInvoiceCouldNotBePaid}
			}
		}

		s, err := s.BoltzAPI.SwapStatus(in.SwapData.BoltzID)
		if err != nil {
			log(in, fmt.Sprintf("Error communicating with BoltzAPI: %v", err), logger.Get("error", err.Error()))
			time.Sleep(SleepTime)
			continue
		}
		status := boltz.ParseEvent(s.Status)

		log(in, fmt.Sprintf("Swap status is: %v", status),
			logger.Get("status", status))

		if (in.SwapData.SwapLimits.AllowZeroConf && status == boltz.TransactionMempool) || status == boltz.TransactionConfirmed {
			if s.Transaction.Hex != "" {
				in.SwapData.TransactionHex = s.Transaction.Hex
				return common.FsmOut{NextState: common.ClaimReverseFunds}
			}
		}

		if status.IsFailedStatus() {
			return common.FsmOut{NextState: common.SwapFailed}
		}

		info, err := lnConnection.GetInfo(ctx)
		if err != nil {
			log(in, fmt.Sprintf("Error communicating with LNAPI: %v", err), logger.Get("error", err.Error()))
			time.Sleep(SleepTime)
			continue
		}

		if uint32(info.BlockHeight) > in.SwapData.TimoutBlockHeight {
			return common.FsmOut{NextState: common.SwapFailed}
		}

		lnConnection.Cleanup()
		time.Sleep(SleepTime)
	}
}

func (s *SwapMachine) FsmSwapInvoiceCouldNotBePaid(in common.FsmIn) common.FsmOut {
	logger := NewLogEntry(in.SwapData)

	message := fmt.Sprintf("Swap %d (attempt %d) failed since invoice for %v sats could not be paid", in.GetJobID(), in.SwapData.Attempt, in.SwapData.ExpectedSats)
	if in.SwapData.IsDryRun {
		// Probably not reachable anyway, since we never try to pay an invoice in dry mode
		message = fmt.Sprintf("Swap %d failed in dry-run mode (no funds were used)", in.GetJobID())
	}

	glog.Infof("[Boltz] [%d] %s", in.GetJobID(), message)
	if in.MsgCallback != nil {
		in.MsgCallback(entities.PluginMessage{
			JobID:      int32(in.GetJobID()),
			Message:    message,
			IsError:    false,
			IsFinished: in.SwapData.IsDryRun,
		})
	}

	if in.SwapData.IsDryRun {
		return common.FsmOut{}
	}

	in.SwapData.RevertFees()

	newMax := uint64(math.Round(float64(in.SwapData.ExpectedSats) * in.SwapData.SwapLimits.BackOffAmount))
	if newMax < in.SwapData.SwapLimits.MaxSwap {
		// Try with lower limit, newMax MUST be lower so we converge to 0 in order to prevent infinite loop
		log(in, fmt.Sprintf("Retrying with new maximum swap size %v sats", newMax), logger.Get("new_max", newMax))

		in.SwapData.SwapLimits.MaxSwap = newMax
		if in.SwapData.SwapLimits.MinSwap > in.SwapData.SwapLimits.MaxSwap {
			in.SwapData.SwapLimits.MinSwap = in.SwapData.SwapLimits.MaxSwap
		}
		if in.SwapData.SwapLimits.DefaultSwap > in.SwapData.SwapLimits.MaxSwap {
			in.SwapData.SwapLimits.DefaultSwap = in.SwapData.SwapLimits.MaxSwap
		}
		return s.nextRound(in)
	}

	return common.FsmOut{Error: fmt.Errorf("invoice could not be paid and no backup plan")}
}

func (s *SwapMachine) FsmClaimReverseFunds(in common.FsmIn) common.FsmOut {
	logger := NewLogEntry(in.SwapData)

	// For state machine this is final state
	if in.SwapData.BoltzID == "" {
		return common.FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}
	}
	if in.SwapData.TransactionHex == "" {
		return common.FsmOut{Error: fmt.Errorf("invalid state transaction hex not set")}
	}

	// debug
	log(in, fmt.Sprintf("Adding entry %v to redeem locked funds", in.SwapData.JobID), logger.Get("state", in.SwapData.State))

	s.ReverseRedeemer.AddEntry(in)
	return common.FsmOut{}
}

func (s *SwapMachine) FsmSwapClaimed(in common.FsmIn) common.FsmOut {
	// This just happpened while e2e testing, in practice we don't really care if
	// Boltz does not claim their funds

	logger := NewLogEntry(in.SwapData)

	log(in, fmt.Sprintf("Locked funds were claimed %v", in.SwapData.JobID), logger.Get("state", in.SwapData.State))

	SleepTime := s.GetSleepTimeFn(in)
	MaxWait := 2 * time.Minute // do we need to make this configurable?

	start := time.Now()
	alert := time.Now()
	for {
		now := time.Now()
		if now.After(start.Add(MaxWait)) {
			break
		}

		s, err := s.BoltzAPI.SwapStatus(in.SwapData.BoltzID)
		if err != nil {
			log(in, fmt.Sprintf("Error communicating with BoltzAPI: %v", err), logger.Get("error", err.Error()))
			time.Sleep(SleepTime)
			continue
		}

		status := boltz.ParseEvent(s.Status)

		if status == boltz.InvoiceSettled {
			break
		} else {
			if now.After(alert.Add(30 * time.Second)) {
				log(in, fmt.Sprintf("Boltz did not claim funds on their side %v, status is %v", in.SwapData.JobID, status), nil)
				alert = now
			}
		}
	}

	return common.FsmOut{NextState: common.SwapSuccess}

}

func CreateReverseSwapWithSanityCheck(api *bapi.BoltzPrivateAPI, keys *crypto.Keys, sats uint64, referralCode string, currentBlockHeight int, chainparams *chaincfg.Params) (*boltz.CreateReverseSwapResponse, error) {
	const BlockEps = 10

	response, err := api.CreateReverseSwap(bapi.CreateReverseSwapRequestOverride{
		CreateReverseSwapRequest: boltz.CreateReverseSwapRequest{
			Type:           "reversesubmarine",
			PairId:         common.BtcPair,
			OrderSide:      "buy",
			PreimageHash:   hex.EncodeToString(keys.Preimage.Hash),
			InvoiceAmount:  sats,
			ClaimPublicKey: hex.EncodeToString(keys.Keys.PublicKey.SerializeCompressed()),
		},
		ReferralId: referralCode,
	},
	)

	if err != nil {
		return nil, err
	}

	redeemScript, err := hex.DecodeString(response.RedeemScript)
	if err != nil {
		return nil, err
	}

	err = boltz.CheckReverseSwapScript(redeemScript, keys.Preimage.Hash, keys.Keys.PrivateKey, response.TimeoutBlockHeight)
	if err != nil {
		return nil, err
	}

	lockupAddress, err := boltz.WitnessScriptHashAddress(chainparams, redeemScript)
	if response.LockupAddress != lockupAddress {
		return nil, fmt.Errorf("lockup address invalid")
	}

	if currentBlockHeight+BlockEps > int(response.TimeoutBlockHeight) {
		return nil, fmt.Errorf("error checking blockheight")
	}

	invoice, err := zpay32.Decode(response.Invoice, chainparams)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(keys.Preimage.Hash, invoice.PaymentHash[:]) {
		return nil, fmt.Errorf("invalid invoice preimage hash")
	}

	if uint64(invoice.MilliSat.ToSatoshis()) != sats {
		return nil, fmt.Errorf("invalid invoice expected %v sats got invoice for %v sats", sats, invoice.MilliSat.ToSatoshis())
	}

	return response, nil
}
