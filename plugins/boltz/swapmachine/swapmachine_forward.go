//go:build plugins
// +build plugins

package swapmachine

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/bolt-observer/agent/lightning"
	bapi "github.com/bolt-observer/agent/plugins/boltz/api"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	connection "github.com/bolt-observer/agent/plugins/boltz/connection"
	crypto "github.com/bolt-observer/agent/plugins/boltz/crypto"
	"github.com/btcsuite/btcd/chaincfg"
)

// (submarine) swap finite state machine

func (s *SwapMachine) FsmInitialForward(in common.FsmIn) common.FsmOut {
	ctx := context.Background()
	logger := NewLogEntry(in.SwapData)

	sats := in.SwapData.Sats

	log(in, fmt.Sprintf("Will do a submarine swap with %v sats", sats),
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
	minerFees := 2 * res.Fees.MinerFees.BaseAsset.Normal

	lnConnection, err := s.LnAPI()
	if err != nil {
		return common.FsmOut{Error: err}
	}
	if lnConnection == nil {
		return common.FsmOut{Error: fmt.Errorf("error getting lightning API")}
	}

	defer lnConnection.Cleanup()

	invoice, err := lnConnection.CreateInvoice(ctx, int64(in.SwapData.Sats), hex.EncodeToString(keys.Preimage.Raw),
		fmt.Sprintf("Automatic Swap Boltz %d", in.GetJobID()), 24*time.Hour) // assume boltz returns 144 blocks
	if err != nil {
		return common.FsmOut{Error: err}
	}

	info, err := lnConnection.GetInfo(ctx)
	if err != nil {
		return common.FsmOut{Error: err}
	}

	response, err := CreateSwapWithSanityCheck(s.BoltzAPI, keys, invoice, s.ReferralCode, info.BlockHeight, s.ChainParams)
	if err != nil {
		return common.FsmOut{Error: err}
	}

	fee := float64(response.ExpectedAmount-sats+minerFees) / float64(sats)
	if fee*100 > in.SwapData.SwapLimits.MaxFeePercentage {
		return common.FsmOut{Error: fmt.Errorf("fee was calculated to be %.2f %%, max allowed is %.2f %%", fee*100, in.SwapData.SwapLimits.MaxFeePercentage)}
	}

	totalFee := float64(in.SwapData.FeesSoFar.FeesPaid+(response.ExpectedAmount-sats)) / float64(in.SwapData.FeesSoFar.SatsSwapped+response.ExpectedAmount) * 100
	if totalFee > in.SwapData.SwapLimits.MaxFeePercentage {
		return common.FsmOut{Error: fmt.Errorf("total fee was calculated to be %.2f %%, max allowed is %.2f %%", totalFee, in.SwapData.SwapLimits.MaxFeePercentage)}
	}

	log(in, fmt.Sprintf("Swap fee for %v will be approximately %v %%", response.Id, fee*100),
		logger.Get("fee", fee*100))

	// Check funds
	funds, err := lnConnection.GetOnChainFunds(ctx)
	if err != nil {
		return common.FsmOut{Error: err}
	}

	if funds.ConfirmedBalance < int64(response.ExpectedAmount+minerFees) {
		return common.FsmOut{Error: fmt.Errorf("we have %v sats on-chain but need %v sats", funds.ConfirmedBalance, response.ExpectedAmount+minerFees)}
	}

	in.SwapData.BoltzID = response.Id
	in.SwapData.Script = response.RedeemScript
	in.SwapData.Address = response.Address
	in.SwapData.TimoutBlockHeight = response.TimeoutBlockHeight

	if in.SwapData.IsDryRun {
		return common.FsmOut{NextState: common.SwapSuccess}
	}

	// Explicitly first change state (in case we crash before sending)
	err = s.ChangeStateFn(in, common.OnChainFundsSent)
	if err != nil {
		return common.FsmOut{Error: err}
	}

	tx, err := lnConnection.SendToOnChainAddress(ctx, response.Address, int64(response.ExpectedAmount), false, lightning.Normal)
	if err != nil {
		return common.FsmOut{Error: err}
	}

	log(in, fmt.Sprintf("Transaction ID for swap %v sats to %v is %v (invoice hash %v)", int64(response.ExpectedAmount), response.Address, tx, invoice.Hash),
		logger.Get("sats", int64(response.ExpectedAmount), "address", response.Address, "transaction", tx, "invoice", invoice.PaymentRequest))
	in.SwapData.LockupTransactionId = tx

	txResp, err := s.BoltzAPI.GetTransaction(bapi.GetTransactionRequest{Currency: common.Btc, TransactionId: tx})
	if err == nil && txResp.TransactionHex != "" {
		log(in, fmt.Sprintf("Transaction hex for swap %v sats to %v is %v (invoice hash %v)", int64(response.ExpectedAmount), response.Address, txResp.TransactionHex, invoice.Hash),
			logger.Get("sats", int64(response.ExpectedAmount), "address", response.Address, "transaction_hex", txResp.TransactionHex, "invoice", invoice.PaymentRequest))

		in.SwapData.TransactionHex = txResp.TransactionHex
	}

	in.SwapData.FeesPending = common.Fees{
		FeesPaid:    (response.ExpectedAmount - sats),
		SatsSwapped: response.ExpectedAmount,
	}
	in.SwapData.CommitFees()

	return common.FsmOut{NextState: common.OnChainFundsSent}
}

func (s *SwapMachine) FsmOnChainFundsSent(in common.FsmIn) common.FsmOut {
	ctx := context.Background()
	logger := NewLogEntry(in.SwapData)

	SleepTime := s.GetSleepTimeFn(in)

	if in.SwapData.BoltzID == "" {
		return common.FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}
	}

	for {
		lnAPI, err := s.LnAPI()
		if err != nil {
			log(in, fmt.Sprintf("error getting LNAPI: %v", err), nil)
			time.Sleep(SleepTime)
			continue
		}
		if lnAPI == nil {
			log(in, "error getting LNAPI", nil)
			time.Sleep(SleepTime)
			continue
		}

		err = connection.EnsureConnected(ctx, lnAPI, s.BoltzAPI)
		if err != nil {
			log(in, fmt.Sprintf("error ensuring we are connected: %v", err), nil)
		}

		stat, err := s.BoltzAPI.SwapStatus(in.SwapData.BoltzID)
		if err != nil {
			log(in, fmt.Sprintf("error communicating with BoltzAPI: %v", err), nil)
			time.Sleep(SleepTime)
			continue
		}
		status := boltz.ParseEvent(stat.Status)

		if in.SwapData.LockupTransactionId == "" {
			if status == boltz.TransactionMempool || status == boltz.TransactionConfirmed {
				in.SwapData.LockupTransactionId = stat.Transaction.Id
			}

			// We are not transitioning back if we crashed before sending
		}

		if status.IsFailedStatus() {
			if in.SwapData.TransactionHex != "" {
				return common.FsmOut{NextState: common.RedeemLockedFunds}
			} else {
				swapTransactionResponse, err := s.BoltzAPI.GetSwapTransaction(in.SwapData.BoltzID)
				if err == nil && swapTransactionResponse.TransactionHex != "" {
					in.SwapData.TransactionHex = swapTransactionResponse.TransactionHex
					return common.FsmOut{NextState: common.RedeemLockedFunds}
				} else {
					txHex, err := s.BoltzAPI.GetTransaction(bapi.GetTransactionRequest{
						Currency:      common.Btc,
						TransactionId: in.SwapData.LockupTransactionId,
					})
					if err == nil && txHex.TransactionHex != "" {
						in.SwapData.TransactionHex = txHex.TransactionHex
						return common.FsmOut{NextState: common.RedeemLockedFunds}
					}
				}
			}
		}

		if status.IsCompletedStatus() || status == boltz.ChannelCreated || status == boltz.InvoicePaid {
			return common.FsmOut{NextState: common.VerifyFundsReceived}
		}

		info, err := lnAPI.GetInfo(ctx)
		if err != nil {
			log(in, fmt.Sprintf("error communicating with LNAPI: %v", err), logger.Get("error", err.Error()))
			time.Sleep(SleepTime)
			continue
		}

		if uint32(info.BlockHeight) > in.SwapData.TimoutBlockHeight {
			if in.SwapData.TransactionHex != "" {
				return common.FsmOut{NextState: common.RedeemLockedFunds}
			} else {
				swapTransactionResponse, err := s.BoltzAPI.GetSwapTransaction(in.SwapData.BoltzID)
				if err == nil && swapTransactionResponse.TransactionHex != "" {
					in.SwapData.TransactionHex = swapTransactionResponse.TransactionHex
					return common.FsmOut{NextState: common.RedeemLockedFunds}
				} else {
					txHex, err := s.BoltzAPI.GetTransaction(bapi.GetTransactionRequest{
						Currency:      common.Btc,
						TransactionId: in.SwapData.LockupTransactionId,
					})
					if err == nil && txHex.TransactionHex != "" {
						in.SwapData.TransactionHex = txHex.TransactionHex
						return common.FsmOut{NextState: common.RedeemLockedFunds}
					} else {
						return common.FsmOut{Error: fmt.Errorf("transaction hex not found for swap")}
					}
				}
			}
		}

		lnAPI.Cleanup()
		time.Sleep(SleepTime)
	}
}

func (s *SwapMachine) FsmRedeemLockedFunds(in common.FsmIn) common.FsmOut {
	ctx := context.Background()
	logger := NewLogEntry(in.SwapData)

	if in.SwapData.BoltzID == "" {
		return common.FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}
	}

	if in.SwapData.LockupTransactionId == "" {
		return common.FsmOut{Error: fmt.Errorf("invalid state txid not set")}
	}

	if in.SwapData.TransactionHex == "" {
		return common.FsmOut{Error: fmt.Errorf("invalid state transaction hex not set")}
	}

	SleepTime := s.GetSleepTimeFn(in)

	// Wait for expiry
	for {
		lnConnection, err := s.LnAPI()
		if err != nil {
			log(in, fmt.Sprintf("error getting LNAPI: %v", err), logger.Get("error", err.Error()))
			time.Sleep(SleepTime)
			continue
		}
		if lnConnection == nil {
			log(in, "error getting LNAPI", nil)
			time.Sleep(SleepTime)
			continue
		}

		info, err := lnConnection.GetInfo(ctx)
		if err != nil {
			log(in, fmt.Sprintf("error communicating with LNAPI: %v", err), logger.Get("error", err.Error()))
			time.Sleep(SleepTime)
			continue
		}

		if uint32(info.BlockHeight) > in.SwapData.TimoutBlockHeight {
			break
		}

		log(in, fmt.Sprintf("Waiting for expiry %d < %d", info.BlockHeight, in.SwapData.TimoutBlockHeight),
			logger.Get("blockheight", info.BlockHeight, "timeout_blockheight", in.SwapData.TimoutBlockHeight))

		lnConnection.Cleanup()
		time.Sleep(SleepTime)
	}

	return common.FsmOut{NextState: common.RedeemingLockedFunds}
}

func (s *SwapMachine) FsmRedeemingLockedFunds(in common.FsmIn) common.FsmOut {
	// For state machine this is final state
	if in.SwapData.BoltzID == "" {
		return common.FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}

	}
	s.Redeemer.AddEntry(in)
	return common.FsmOut{}
}

func (s *SwapMachine) FsmVerifyFundsReceived(in common.FsmIn) common.FsmOut {
	ctx := context.Background()
	logger := NewLogEntry(in.SwapData)

	if in.SwapData.BoltzID == "" {
		return common.FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}
	}

	SleepTime := s.GetSleepTimeFn(in)

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

		keys, err := s.CryptoAPI.GetKeys(in.GetUniqueJobID())
		if err != nil {
			log(in, "error getting keys", logger.Get("error", err.Error()))
			time.Sleep(SleepTime)
			continue
		}

		paid, err := lnConnection.IsInvoicePaid(ctx, hex.EncodeToString(keys.Preimage.Hash))
		if err != nil {
			log(in, "error checking whether invoice is paid", logger.Get("error", err.Error()))
			time.Sleep(SleepTime)
			continue
		}

		if paid {
			return common.FsmOut{NextState: common.SwapSuccess}
		} else {
			return common.FsmOut{NextState: common.SwapFailed}
		}
	}
}

func CreateSwapWithSanityCheck(api *bapi.BoltzPrivateAPI, keys *crypto.Keys, invoice *lightning.InvoiceResp, referralCode string, currentBlockHeight int, chainparams *chaincfg.Params) (*boltz.CreateSwapResponse, error) {
	const BlockEps = 10

	response, err := api.CreateSwap(bapi.CreateSwapRequestOverride{
		CreateSwapRequest: boltz.CreateSwapRequest{
			Type:            "submarine",
			PairId:          common.BtcPair,
			OrderSide:       "buy",
			PreimageHash:    hex.EncodeToString(keys.Preimage.Hash),
			RefundPublicKey: hex.EncodeToString(keys.Keys.PublicKey.SerializeCompressed()),
			Invoice:         invoice.PaymentRequest,
		},
		ReferralId: referralCode,
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
