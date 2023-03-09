package boltz

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/bolt-observer/agent/entities"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/golang/glog"
)

// Redeemer is an abstraction that periodically gathers "stuck" UTXOs and spends them to a new lightning node address.
// It can combine failed forward submarine (swap-in) transactions (RedeemForward) and/or the outputs that need to be claimed for reverse submarine (swap-out) transactions (RedeemReverse)
// Redeemer is supposed to work with any type that wraps SwapData (implements SwapDataGetter interface - generics in Go are a bit weird).

type JobID int32

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

	Entries map[JobID]T
	lock    sync.Mutex
	Timer   *time.Timer

	ChainParams *chaincfg.Params
	BoltzAPI    *boltz.Boltz
	LnAPI       entities.NewAPICall
	CryptoAPI   *CryptoAPI
}

type SwapDataGetter interface {
	GetSwapData() *SwapData
}

type RedeemedCallback[T SwapDataGetter] func(data T, success bool)

func NewRedeemer[T SwapDataGetter](ctx context.Context, t RedeemerType, chainParams *chaincfg.Params, boltzAPI *boltz.Boltz, lnAPI entities.NewAPICall,
	interval time.Duration, cryptoAPI *CryptoAPI, callback RedeemedCallback[T]) *Redeemer[T] {

	if t == 0 {
		glog.Warningf("Invalid nil redeemer does not make sense")
		return nil
	}

	r := &Redeemer[T]{
		Ctx:         ctx,
		Type:        t,
		Callback:    callback,
		Entries:     make(map[JobID]T),
		lock:        sync.Mutex{},
		ChainParams: chainParams,
		BoltzAPI:    boltzAPI,
		LnAPI:       lnAPI,
		CryptoAPI:   cryptoAPI,
	}

	go r.eventLoop()
	r.Timer = time.NewTimer(interval)

	return r
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
	used := make([]JobID, 0)
	for _, entry := range r.Entries {
		var output *boltz.OutputDetails
		sd := entry.GetSwapData()
		if sd.State == RedeemingLockedFunds {
			if info.BlockHeight < int(sd.TimoutBlockHeight) {
				continue
			}

			output = r.getRefundOutput(sd)
		} else if sd.State == ClaimReverseFunds {
			if info.BlockHeight > int(sd.TimoutBlockHeight) {
				r.Callback(entry, false)
				continue
			}

			output = r.getClaimOutput(sd)

		} else {
			continue
		}

		if output == nil {
			continue
		}
		outputs = append(outputs, *output)
		used = append(used, sd.JobID)
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
			delete(r.Entries, one)
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
		if sd.State == RedeemingLockedFunds {
			ok = true
		}
	}

	if r.Type&RedeemReverse == RedeemReverse {
		if sd.State == ClaimReverseFunds {
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("trying to add non-redeemable thing (%v %v)", sd.State, sd.JobID)
	}

	// debug
	fmt.Printf("Adding %+v to redeemer\n", data)

	r.Entries[sd.JobID] = data

	return nil
}

func (r *Redeemer[T]) doRedeem(outputs []boltz.OutputDetails) (string, error) {
	fmt.Printf("Trying to redeem %d outputs\n", len(outputs))

	// TODO: fee estimation now depends on working boltz API - use lightning API for it
	feeResp, err := r.BoltzAPI.GetFeeEstimation()
	if err != nil {
		return "", err
	}

	fee := *feeResp

	satsPerVbyte, ok := fee[Btc]
	if !ok {
		return "", fmt.Errorf("error getting fee estimate")
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

	err = r.broadcastTransaction(transaction)
	if err != nil {
		return "", err
	}

	return transaction.TxHash().String(), nil
}

func (r *Redeemer[T]) getClaimOutput(data *SwapData) *boltz.OutputDetails {
	if data.State != ClaimReverseFunds {
		return nil
	}

	status, err := r.BoltzAPI.SwapStatus(data.BoltzID)
	if err != nil {
		glog.Warningf("Could not get swap status %v", err)
		return nil
	}

	lockupTransactionRaw, err := hex.DecodeString(status.Transaction.Hex)
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

	keys, err := r.CryptoAPI.GetKeys(fmt.Sprintf("%d", data.JobID))
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

func (r *Redeemer[T]) getRefundOutput(data *SwapData) *boltz.OutputDetails {
	if data.State != RedeemingLockedFunds {
		return nil
	}

	swapTransactionResponse, err := r.BoltzAPI.GetSwapTransaction(data.BoltzID)

	if err != nil {
		glog.Warningf("Could not get refund transaction %v", err)
		return nil
	}

	lockupTransactionRaw, err := hex.DecodeString(swapTransactionResponse.TransactionHex)

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

	keys, err := r.CryptoAPI.GetKeys(fmt.Sprintf("%d", data.JobID))
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

func (r *Redeemer[T]) broadcastTransaction(transaction *wire.MsgTx) error {
	transactionHex, err := boltz.SerializeTransaction(transaction)

	if err != nil {
		return fmt.Errorf("could not serialize transaction: %v", err)
	}

	_, err = r.BoltzAPI.BroadcastTransaction(transactionHex)

	if err != nil {
		return fmt.Errorf("could not broadcast transaction: %v", err)
	}

	return nil
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
