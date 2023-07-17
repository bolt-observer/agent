//go:build plugins
// +build plugins

package data

import (
	"time"

	"github.com/bolt-observer/agent/filter"
	"github.com/btcsuite/btcd/chaincfg"

	bapi "github.com/bolt-observer/agent/plugins/boltz/api"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	crypto "github.com/bolt-observer/agent/plugins/boltz/crypto"
	redeemer "github.com/bolt-observer/agent/plugins/boltz/redeemer"
)

type ChangeStateFn func(in common.FsmIn, state common.State) error
type GetSleepTimeFn func(in common.FsmIn) time.Duration
type DeleteJobFn func(jobID int64) error

type PluginData struct {
	ReferralCode        string
	ChainParams         *chaincfg.Params
	Filter              filter.FilteringInterface
	CryptoAPI           *crypto.CryptoAPI
	Redeemer            *redeemer.Redeemer[common.FsmIn]
	ReverseRedeemer     *redeemer.Redeemer[common.FsmIn]
	Limits              common.SwapLimits
	BoltzAPI            *bapi.BoltzPrivateAPI
	ChangeStateFn       ChangeStateFn
	GetSleepTimeFn      GetSleepTimeFn
	DeleteJobFn         DeleteJobFn
	AllowChanCreation   bool
	PrivateChanCreation bool
	Name                string
}
