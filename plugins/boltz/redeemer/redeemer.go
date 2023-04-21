//go:build plugins
// +build plugins

package redeemer

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	api "github.com/bolt-observer/agent/lightning"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	crypto "github.com/bolt-observer/agent/plugins/boltz/crypto"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/golang/glog"
)

// Redeemer is an abstraction that periodically gathers "stuck" UTXOs and spends them to a new lightning node address.
// It can combine failed forward submarine (swap-in) transactions (RedeemForward) and/or the outputs that need to be claimed for reverse submarine (swap-out) transactions (RedeemReverse)
// Redeemer is supposed to work with any type that wraps SwapData (implements SwapDataGetter interface - generics in Go are a bit weird).

// RedeemerType is a bitmask
type RedeemerType int

const (
	RedeemForward RedeemerType = 1 << iota
	RedeemReverse
)

// Redeemer struct.
type Redeemer[T SwapDataGetter] struct {
	Type     RedeemerType
	Ctx      context.Context
	Callback RedeemedCallback[T]

	Entries map[common.JobID]T
	lock    sync.Mutex
	Timer   *time.Ticker

	ChainParams         *chaincfg.Params
	OnChainCommunicator OnChainCommunicator
	LnAPI               api.NewAPICall
	CryptoAPI           *crypto.CryptoAPI
}

type SwapDataGetter interface {
	GetSwapData() *common.SwapData
}

type RedeemedCallback[T SwapDataGetter] func(data T, success bool)

func NewRedeemer[T SwapDataGetter](ctx context.Context, t RedeemerType, chainParams *chaincfg.Params, OnChainCommunicator OnChainCommunicator, lnAPI api.NewAPICall,
	interval time.Duration, cryptoAPI *crypto.CryptoAPI) *Redeemer[T] {

	if t == 0 {
		glog.Warningf("Invalid nil redeemer does not make sense")
		return nil
	}

	r := &Redeemer[T]{
		Ctx:                 ctx,
		Type:                t,
		Callback:            nil,
		Entries:             make(map[common.JobID]T),
		lock:                sync.Mutex{},
		ChainParams:         chainParams,
		OnChainCommunicator: OnChainCommunicator,
		LnAPI:               lnAPI,
		CryptoAPI:           cryptoAPI,
	}

	r.Timer = time.NewTicker(interval)
	go r.eventLoop()

	return r
}

func (r *Redeemer[T]) SetCallback(callback RedeemedCallback[T]) {
	r.Callback = callback
}

func (r *Redeemer[T]) eventLoop() {
	func() {
		for {
			select {
			case <-r.Timer.C:
				if !r.redeem() {
					r.Timer.Stop()
					return
				}
			case <-r.Ctx.Done():
				return
			}
		}
	}()
}

func (r *Redeemer[T]) redeem() bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	if len(r.Entries) <= 0 {
		// Nothing to do
		return true
	}

	lnConnection, err := r.LnAPI()
	if err != nil {
		return true
	}
	if lnConnection == nil {
		return true
	}

	defer lnConnection.Cleanup()

	info, err := lnConnection.GetInfo(r.Ctx)
	if err != nil {
		return true
	}

	outputs := make([]boltz.OutputDetails, 0)
	used := make([]common.JobID, 0)
	del := make([]common.JobID, 0)

	for _, entry := range r.Entries {
		var output *boltz.OutputDetails
		sd := entry.GetSwapData()
		if sd.State == common.RedeemingLockedFunds {
			if info.BlockHeight < int(sd.TimoutBlockHeight) {
				continue
			}

			output = r.getRefundOutput(sd)
		} else if sd.State == common.ClaimReverseFunds {
			if info.BlockHeight > int(sd.TimoutBlockHeight) {
				if r.Callback != nil {
					r.Callback(entry, false)
				}
				del = append(del, entry.GetSwapData().JobID)
				output = nil
			} else {
				output = r.getClaimOutput(sd)
			}
		} else {
			continue
		}

		if output == nil {
			continue
		}
		outputs = append(outputs, *output)
		used = append(used, sd.JobID)
	}

	if len(del) > 0 {
		for _, one := range del {
			delete(r.Entries, one)
		}
	}

	if len(outputs) <= 0 {
		return true
	}
	_, err = r.doRedeem(outputs)
	if err == nil {
		for _, one := range used {
			if r.Callback != nil {
				data, ok := r.Entries[one]
				if ok {
					r.Callback(data, true)
				}
			}
			if _, ok := r.Entries[one]; ok {
				delete(r.Entries, one)
			}
		}
	} else {
		if strings.Contains(err.Error(), "bad-txns-inputs-missingorspent") {
			for _, one := range used {
				if r.Callback != nil {
					data, ok := r.Entries[one]
					if ok {
						r.Callback(data, false)
					}
				}
				if _, ok := r.Entries[one]; ok {
					delete(r.Entries, one)
				}
			}
		} else {
			glog.Warningf("Redeeming failed due to %v\n", err)
		}
	}

	return true
}

func (r *Redeemer[T]) AddEntry(data T) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	// Check whether we can handle the state
	ok := false
	sd := data.GetSwapData()
	if r.Type&RedeemForward == RedeemForward {
		if sd.State == common.RedeemingLockedFunds && sd.TransactionHex != "" {
			ok = true
		}
	}

	if r.Type&RedeemReverse == RedeemReverse {
		if sd.State == common.ClaimReverseFunds && sd.TransactionHex != "" {
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("trying to add non-redeemable thing (%v %v)", sd.State, sd.JobID)
	}

	r.Entries[sd.JobID] = data

	return nil
}

func (r *Redeemer[T]) doRedeem(outputs []boltz.OutputDetails) (string, error) {
	satsPerVbyte, err := r.OnChainCommunicator.GetFeeEstimation()
	if err != nil {
		return "", err
	}

	lnConnection, err := r.LnAPI()
	if err != nil {
		return "", err
	}
	if lnConnection == nil {
		return "", fmt.Errorf("error getting LNAPI")
	}

	defer lnConnection.Cleanup()
	addr, err := lnConnection.GetOnChainAddress(r.Ctx)
	if err != nil {
		return "", err
	}

	address, err := btcutil.DecodeAddress(addr, r.ChainParams)
	if err != nil {
		return "", err
	}

	transaction, err := boltz.ConstructTransaction(outputs, address, int64(satsPerVbyte))
	if err != nil {
		return "", err
	}

	err = r.OnChainCommunicator.BroadcastTransaction(transaction)
	if err != nil {
		return "", err
	}

	return transaction.TxHash().String(), nil
}

func (r *Redeemer[T]) getClaimOutput(data *common.SwapData) *boltz.OutputDetails {
	if data.State != common.ClaimReverseFunds {
		return nil
	}

	if data.TransactionHex == "" {
		return nil
	}

	lockupTransactionRaw, err := hex.DecodeString(data.TransactionHex)
	if err != nil {
		glog.Warningf("Could not decode transaction %v", err)
		return nil
	}

	lockupTransaction, err := btcutil.NewTxFromBytes(lockupTransactionRaw)
	if err != nil {
		glog.Warningf("Could not parse transaction %v", err)
		return nil
	}

	script, err := hex.DecodeString(data.Script)
	if err != nil {
		glog.Warningf("Could not decode script %v", err)
		return nil
	}

	lockupAddress, err := boltz.WitnessScriptHashAddress(r.ChainParams, script)
	if err != nil {
		glog.Warningf("Could not derive address %v", err)
		return nil
	}

	lockupVout, err := r.findLockupVout(lockupAddress, lockupTransaction.MsgTx().TxOut)
	if err != nil {
		glog.Warningf("Could not parse lockup vout %v", err)
		return nil
	}

	keys, err := r.CryptoAPI.GetKeys(data.GetUniqueJobID())
	if err != nil {
		glog.Warningf("Could not get keys %v", err)
		return nil
	}

	if lockupTransaction.MsgTx().TxOut[lockupVout].Value < int64(data.ExpectedSats) {
		glog.Warningf("Expected %v sats on chain but got just %v sats", lockupTransaction.MsgTx().TxOut[lockupVout].Value, data.ExpectedSats)
		return nil
	}

	return &boltz.OutputDetails{
		LockupTransaction: lockupTransaction,
		Vout:              lockupVout,
		OutputType:        boltz.SegWit,
		RedeemScript:      script,
		PrivateKey:        keys.Keys.PrivateKey,
		Preimage:          keys.Preimage.Raw,
	}
}

func (r *Redeemer[T]) getRefundOutput(data *common.SwapData) *boltz.OutputDetails {
	if data.State != common.RedeemingLockedFunds {
		return nil
	}

	if data.TransactionHex == "" {
		return nil
	}

	lockupTransactionRaw, err := hex.DecodeString(data.TransactionHex)

	if err != nil {
		glog.Warningf("Could not decode transaction %v", err)
		return nil
	}

	lockupTransaction, err := btcutil.NewTxFromBytes(lockupTransactionRaw)

	if err != nil {
		glog.Warningf("Could not parse transaction %v", err)
		return nil
	}

	lockupVout, err := r.findLockupVout(data.Address, lockupTransaction.MsgTx().TxOut)
	if err != nil {
		glog.Warningf("Could not parse lockup vout %v", err)
		return nil
	}

	keys, err := r.CryptoAPI.GetKeys(data.GetUniqueJobID())
	if err != nil {
		glog.Warningf("Could not get keys %v", err)
		return nil
	}

	script, err := hex.DecodeString(data.Script)
	if err != nil {
		glog.Warningf("Could not decode script %v", err)
		return nil
	}

	return &boltz.OutputDetails{
		LockupTransaction:  lockupTransaction,
		Vout:               lockupVout,
		OutputType:         boltz.Compatibility,
		RedeemScript:       script,
		PrivateKey:         keys.Keys.PrivateKey,
		Preimage:           []byte{},
		TimeoutBlockHeight: data.TimoutBlockHeight,
	}
}

func (r *Redeemer[T]) findLockupVout(addressToFind string, outputs []*wire.TxOut) (uint32, error) {
	for vout, output := range outputs {
		_, outputAddresses, _, err := txscript.ExtractPkScriptAddrs(output.PkScript, r.ChainParams)

		// Just ignore outputs we can't decode
		if err != nil {
			continue
		}

		for _, outputAddress := range outputAddresses {
			if outputAddress.EncodeAddress() == addressToFind {
				return uint32(vout), nil
			}
		}
	}

	return 0, fmt.Errorf("could not find lockup vout")
}
