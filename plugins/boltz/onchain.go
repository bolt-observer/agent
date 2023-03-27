//go:build plugins
// +build plugins

package boltz

import (
	"fmt"

	"github.com/btcsuite/btcd/wire"

	"github.com/BoltzExchange/boltz-lnd/boltz"
)

type OnChainCommunicator interface {
	BroadcastTransaction(transaction *wire.MsgTx) error
	GetFeeEstimation() (uint64, error)
}

type BoltzOnChainCommunicator struct {
	API *boltz.Boltz
}

func NewBoltzOnChainCommunicator(api *boltz.Boltz) *BoltzOnChainCommunicator {
	return &BoltzOnChainCommunicator{
		API: api,
	}
}

func (c *BoltzOnChainCommunicator) BroadcastTransaction(transaction *wire.MsgTx) error {
	transactionHex, err := boltz.SerializeTransaction(transaction)

	if err != nil {
		return fmt.Errorf("could not serialize transaction: %v", err)
	}

	_, err = c.API.BroadcastTransaction(transactionHex)

	if err != nil {
		return fmt.Errorf("could not broadcast transaction: %v", err)
	}

	return nil
}

func (c *BoltzOnChainCommunicator) GetFeeEstimation() (uint64, error) {
	feeResp, err := c.API.GetFeeEstimation()
	if err != nil {
		return 0, err
	}

	fee := *feeResp

	satsPerVbyte, ok := fee[Btc]
	if !ok {
		return 0, fmt.Errorf("error getting fee estimate")
	}

	return satsPerVbyte, nil
}
