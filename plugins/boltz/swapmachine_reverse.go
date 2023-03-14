//go:build plugins
// +build plugins

package boltz

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"
)

// Reverse (submarine) swap finite state machine

func (s *SwapMachine) FsmInitialReverse(in FsmIn) FsmOut {
	ctx := context.Background()
	const SafetyMargin = 1000 // sats

	sats := in.SwapData.Sats

	log(in, fmt.Sprintf("Will do a reverse submarine swap with %v sats", sats))

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
	minerFees := 2 * res.Fees.MinerFees.BaseAsset.Reverse.Claim

	lnConnection, err := s.BoltzPlugin.LnAPI()
	if err != nil {
		return FsmOut{Error: err}
	}
	if lnConnection == nil {
		return FsmOut{Error: fmt.Errorf("error getting lightning API")}
	}

	defer lnConnection.Cleanup()

	info, err := lnConnection.GetInfo(ctx)
	if err != nil {
		return FsmOut{Error: err}
	}

	response, err := CreateReverseSwapWithSanityCheck(s.BoltzPlugin.BoltzAPI, keys, sats, info.BlockHeight, s.BoltzPlugin.ChainParams)
	if err != nil {
		return FsmOut{Error: err}
	}

	fee := float64(sats-response.OnchainAmount+minerFees+SafetyMargin) / float64(sats)
	if fee*100 > s.BoltzPlugin.MaxFeePercentage {
		return FsmOut{Error: fmt.Errorf("fee was calculated to be %.2f %%, max allowed is %.2f %%", fee*100, s.BoltzPlugin.MaxFeePercentage)}
	}

	log(in, fmt.Sprintf("Swap fee for %v will be approximately %v %%", response.Id, fee*100))

	// Check funds
	if in.SwapData.ReverseChannelId == 0 {
		capacity, err := s.BoltzPlugin.GetByDescendingOutboundLiquidity(ctx, sats+SafetyMargin, lnConnection)
		if err != nil {
			return FsmOut{Error: err}
		}
		if len(capacity) <= 0 {
			return FsmOut{Error: fmt.Errorf("invalid capacities")}
		}

		chans := make([]uint64, 0)
		for _, one := range capacity {
			chans = append(chans, one.Channel.ChanID)
		}

		in.SwapData.ChanIdsToUse = chans
	} else {
		// Will error when sufficient funds are not available
		_, _, err = s.BoltzPlugin.GetChanLiquidity(ctx, in.SwapData.ReverseChannelId, sats+SafetyMargin, true, lnConnection)
		if err != nil {
			return FsmOut{Error: err}
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

	return FsmOut{NextState: ReverseSwapCreated}
}

func (s *SwapMachine) FsmReverseSwapCreated(in FsmIn) FsmOut {
	ctx := context.Background()
	paid := false

	SleepTime := s.getSleepTime(in)

	if in.SwapData.BoltzID == "" {
		return FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}
	}

	if in.SwapData.IsDryRun {
		return FsmOut{NextState: SwapSuccess}
	}

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

		if !paid {
			log(in, fmt.Sprintf("Paying invoice %v", in.SwapData.ReverseInvoice))

			_, err = lnConnection.PayInvoice(ctx, in.SwapData.ReverseInvoice, 0, in.SwapData.ChanIdsToUse)
			if err != nil {
				log(in, fmt.Sprintf("Failed paying invoice %v", in.SwapData.ReverseInvoice))
				if in.SwapData.ReverseChannelId == 0 {
					// this means node level liquidity - if the hints worked that would be nice, but try without them too

					_, err = lnConnection.PayInvoice(ctx, in.SwapData.ReverseInvoice, 0, nil)
					if err == nil {
						paid = true
					}
				}
			} else {
				paid = true
			}
		}

		s, err := s.BoltzPlugin.BoltzAPI.SwapStatus(in.SwapData.BoltzID)
		if err != nil {
			log(in, fmt.Sprintf("error communicating with BoltzAPI: %v", err))
			time.Sleep(SleepTime)
			continue
		}
		status := boltz.ParseEvent(s.Status)

		if (in.SwapData.AllowZeroConf && status == boltz.TransactionMempool) || status == boltz.TransactionConfirmed {
			return FsmOut{NextState: ClaimReverseFunds}
		}

		if status.IsFailedStatus() {
			return FsmOut{NextState: SwapFailed}
		}

		info, err := lnConnection.GetInfo(ctx)
		if err != nil {
			log(in, fmt.Sprintf("error communicating with LNAPI: %v", err))
			time.Sleep(SleepTime)
			continue
		}

		if uint32(info.BlockHeight) > in.SwapData.TimoutBlockHeight {
			return FsmOut{NextState: SwapFailed}
		}

		lnConnection.Cleanup()
		time.Sleep(SleepTime)
	}
}

func (s *SwapMachine) FsmClaimReverseFunds(in FsmIn) FsmOut {
	// For state machine this is final state
	if in.SwapData.BoltzID == "" {
		return FsmOut{Error: fmt.Errorf("invalid state boltzID not set")}
	}

	s.BoltzPlugin.ReverseRedeemer.AddEntry(in)
	return FsmOut{}
}

func CreateReverseSwapWithSanityCheck(api *boltz.Boltz, keys *Keys, sats uint64, currentBlockHeight int, chainparams *chaincfg.Params) (*boltz.CreateReverseSwapResponse, error) {
	const BlockEps = 10

	response, err := api.CreateReverseSwap(boltz.CreateReverseSwapRequest{
		Type:           "reversesubmarine",
		PairId:         BtcPair,
		OrderSide:      "buy",
		PreimageHash:   hex.EncodeToString(keys.Preimage.Hash),
		InvoiceAmount:  sats,
		ClaimPublicKey: hex.EncodeToString(keys.Keys.PublicKey.SerializeCompressed()),
	})

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
