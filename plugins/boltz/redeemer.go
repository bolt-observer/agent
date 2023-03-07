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

// It can combine failed normal submarine transactions (RedeemNormal) and/or the outputs that need to be claimed for reverse submarine transactions (RedeemReverse)
type JobID int32
type RedeemerType int

const (
	RedeemNormal RedeemerType = 1 << iota
	RedeemReverse
)

type Redeemer struct {
	Type     RedeemerType
	Ctx      context.Context
	Callback RedeemedCallback

	Entries map[JobID]SwapData
	lock    sync.Mutex
	Timer   *time.Timer

	ChainParams *chaincfg.Params
	BoltzAPI    *boltz.Boltz
	LnAPI       entities.NewAPICall
	CryptoAPI   *CryptoAPI
}

type RedeemedCallback func(id JobID)

func NewRedeemer(ctx context.Context, t RedeemerType, chainParams *chaincfg.Params, boltzAPI *boltz.Boltz, lnAPI entities.NewAPICall,
	interval time.Duration, cryptoAPI *CryptoAPI, callback RedeemedCallback) *Redeemer {
	r := &Redeemer{
		Ctx:         ctx,
		Type:        t,
		Callback:    callback,
		Entries:     make(map[JobID]SwapData),
		lock:        sync.Mutex{},
		ChainParams: chainParams,
		BoltzAPI:    boltzAPI,
		LnAPI:       lnAPI,
		CryptoAPI:   cryptoAPI,
		Timer:       time.NewTimer(interval),
	}

	go r.eventLoop()
	return r
}

func (r *Redeemer) eventLoop() {
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

func (r *Redeemer) redeem() bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	if len(r.Entries) <= 0 {
		// Nothing to do
		return true
	}

	outputs := make([]boltz.OutputDetails, 0)
	used := make([]JobID, 0)
	for _, entry := range r.Entries {
		output := r.getRefundOutput(&entry)
		if output == nil {
			continue
		}
		outputs = append(outputs, *output)
		used = append(used, entry.JobID)
	}
	if len(outputs) <= 0 {
		return true
	}
	_, err := r.doRedemption(outputs)
	if err == nil {
		for _, one := range used {
			if r.Callback != nil {
				r.Callback(one)
			}
			delete(r.Entries, one)
		}
	}

	return true
}

func (r *Redeemer) AddEntry(data SwapData) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	// Check whether we can handle the state
	ok := false
	if r.Type&RedeemNormal == RedeemNormal {
		if data.State == RedeemingLockedFunds {
			ok = true
		}
	}

	if r.Type&RedeemReverse == RedeemReverse {
		if data.State == ClaimReverseFunds {
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("trying to add non-redeemable thing (%v)", data.State)
	}

	r.Entries[data.JobID] = data

	return nil
}

func (r *Redeemer) doRedemption(outputs []boltz.OutputDetails) (string, error) {
	feeResp, err := r.BoltzAPI.GetFeeEstimation()
	if err != nil {
		return "", err
	}

	fee := *feeResp

	satsPerVbyte, ok := fee[Btc]
	if !ok {
		return "", fmt.Errorf("error getting fee estimate")
	}

	lnAPI, err := r.LnAPI()
	if err != nil {
		return "", err
	}
	if lnAPI == nil {
		return "", fmt.Errorf("error getting LNAPI")
	}

	defer lnAPI.Cleanup()
	addr, err := lnAPI.GetOnChainAddress(r.Ctx)
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

func (r *Redeemer) getRefundOutput(data *SwapData) *boltz.OutputDetails {
	if data.State != RedeemingLockedFunds {
		return nil
	}

	swapTransactionResponse, err := r.BoltzAPI.GetSwapTransaction(data.BoltzID)

	if err != nil {
		glog.Warningf("Could not get refund transaction %v\n", err)
		return nil
	}

	lockupTransactionRaw, err := hex.DecodeString(swapTransactionResponse.TransactionHex)

	if err != nil {
		glog.Warningf("Could not decode transaction %v\n", err)
		return nil
	}

	lockupTransaction, err := btcutil.NewTxFromBytes(lockupTransactionRaw)

	if err != nil {
		glog.Warningf("Could not parse transaction %v\n", err)
		return nil
	}

	lockupVout, err := r.findLockupVout(data.Address, lockupTransaction.MsgTx().TxOut)
	if err != nil {
		glog.Warningf("Could not parse lockup vout %v\n", err)
		return nil
	}

	keys, err := r.CryptoAPI.GetKeys(fmt.Sprintf("%d", data.JobID))
	if err != nil {
		glog.Warningf("Could not get keys %v\n", err)
		return nil
	}

	script, err := hex.DecodeString(data.Script)
	if err != nil {
		glog.Warningf("Could not decode script %v\n", err)
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

func (r *Redeemer) broadcastTransaction(transaction *wire.MsgTx) error {
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

func (r *Redeemer) findLockupVout(addressToFind string, outputs []*wire.TxOut) (uint32, error) {
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
