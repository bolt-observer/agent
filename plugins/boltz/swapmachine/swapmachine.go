//go:build plugins
// +build plugins

package swapmachine

import (
	"context"
	"fmt"
	"time"

	"encoding/json"

	"github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightning"
	bapi "github.com/bolt-observer/agent/plugins/boltz/api"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	crypto "github.com/bolt-observer/agent/plugins/boltz/crypto"
	data "github.com/bolt-observer/agent/plugins/boltz/data"
	redeemer "github.com/bolt-observer/agent/plugins/boltz/redeemer"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/golang/glog"
)

func log(in common.FsmIn, msg string, data []byte) {
	ret := data
	if data != nil && !json.Valid(data) {
		ret = nil
	}

	glog.Infof("[%v] [%d] %s", in.SwapData.PluginName, in.GetJobID(), msg)
	if in.MsgCallback != nil {
		in.MsgCallback(entities.PluginMessage{
			JobID:      int64(in.GetJobID()),
			Message:    msg,
			IsError:    false,
			IsFinished: false,
			Data:       ret,
		})
	}
}

func (s *SwapMachine) FsmNone(in common.FsmIn) common.FsmOut {
	log(in, "Invalid state reached", nil)
	panic("Invalid state reached")
}

func (s *SwapMachine) FsmSwapFailed(in common.FsmIn) common.FsmOut {
	if in.MsgCallback != nil {
		in.MsgCallback(entities.PluginMessage{
			JobID:      int64(in.GetJobID()),
			Message:    fmt.Sprintf("Swap %d failed", in.GetJobID()),
			IsError:    true,
			IsFinished: true,
		})
	}

	// Make sure after swap latest data is sent
	if s.NodeDataInvalidator != nil {
		s.NodeDataInvalidator.Invalidate()
	}

	return common.FsmOut{}
}

func (s *SwapMachine) FsmSwapSuccess(in common.FsmIn) common.FsmOut {
	return common.FsmOut{}
}

func (s *SwapMachine) FsmSwapSuccessOne(in common.FsmIn) common.FsmOut {
	// TODO: it would be great if we could calculate how much the swap actually cost us, but it is hard to do precisely
	// because redeemer might have claimed multiple funds at once.

	message := fmt.Sprintf("Swap %d (attempt %d) succeeded", in.GetJobID(), in.SwapData.Attempt)
	if in.SwapData.IsDryRun {
		message = fmt.Sprintf("Swap %d finished in dry-run mode (no funds were used)", in.GetJobID())
	}

	glog.Infof("[%v] [%d] %s", in.SwapData.PluginName, in.GetJobID(), message)
	if in.MsgCallback != nil {
		in.MsgCallback(entities.PluginMessage{
			JobID:      int64(in.GetJobID()),
			Message:    message,
			IsError:    false,
			IsFinished: in.SwapData.IsDryRun,
		})
	}

	if in.SwapData.IsDryRun {
		return common.FsmOut{NextState: common.SwapSuccess}
	}

	//TODO: to prevent race-condition with channel status not getting update, despite swap finished
	time.Sleep(30 * time.Second)

	return s.nextRound(in)
}

func (s *SwapMachine) nextRound(in common.FsmIn) common.FsmOut {
	ctx := context.Background()

	lnConnection, err := s.LnAPI()
	if err != nil {
		return common.FsmOut{Error: err}
	}
	if lnConnection == nil {
		return common.FsmOut{Error: fmt.Errorf("error getting lightning API")}
	}

	defer lnConnection.Cleanup()

	// When we do another round automatically treat a swap < min limit as success
	in.SwapData.SwapLimits.BelowMinAmountIsSuccess = true
	sd, err := s.JobDataToSwapData(ctx, in.SwapData.SwapLimits, &in.SwapData.OriginalJobData, in.MsgCallback, lnConnection, s.Filter, in.SwapData.PluginName)

	if err == common.ErrNoNeedToDoAnything {
		if in.MsgCallback != nil {
			message := fmt.Sprintf("Swap %d overall finished in attempt %d", in.GetJobID(), in.SwapData.Attempt)
			in.MsgCallback(entities.PluginMessage{
				JobID:      int64(in.GetJobID()),
				Message:    message,
				IsError:    false,
				IsFinished: true,
			})
		}

		// Make sure after swap latest data is sent
		if s.NodeDataInvalidator != nil {
			s.NodeDataInvalidator.Invalidate()
		}

		return common.FsmOut{NextState: common.SwapSuccess}
	} else if err != nil {
		return common.FsmOut{Error: err}
	}

	// TODO: does this make sense? Maybe we should start multiple swaps in parallel at the begining
	sd.Attempt = in.SwapData.Attempt + 1
	if sd.Attempt > in.SwapData.SwapLimits.MaxAttempts {
		if in.MsgCallback != nil {
			message := fmt.Sprintf("Swap %d aborted after attempt %d/%d", in.GetJobID(), in.SwapData.Attempt, in.SwapData.SwapLimits.MaxAttempts)
			in.MsgCallback(entities.PluginMessage{
				JobID:      int64(in.GetJobID()),
				Message:    message,
				IsError:    false,
				IsFinished: true,
			})
		}

		// Make sure after swap latest data is sent
		if s.NodeDataInvalidator != nil {
			s.NodeDataInvalidator.Invalidate()
		}

		return common.FsmOut{NextState: common.SwapSuccess}
	}

	in.SwapData = sd
	go s.Eval(in, sd.State)

	return common.FsmOut{}
}

func (s *SwapMachine) RedeemedCallback(data common.FsmIn, success bool) {
	sd := data.GetSwapData()
	if sd.State == common.RedeemingLockedFunds {
		// If we are redeeming locked funds this means by definition our swap failed
		// so when redeemer was able to recover the funds we can transition to final state
		// else we stay in RedeemingLockedFunds and continue with it until eternity
		if success {
			log(data, fmt.Sprintf("Successfully redeemed locked funds"), nil)
			go s.Eval(data, common.SwapFailed)
		} else {
			go func() {
				time.Sleep(s.GetSleepTimeFn(data))
				s.Eval(data, common.RedeemingLockedFunds)
			}()
		}
	} else if sd.State == common.ClaimReverseFunds {
		if success {
			log(data, fmt.Sprintf("Successfully claimed swap"), nil)
			go s.Eval(data, common.SwapClaimed)
		} else {
			go s.Eval(data, common.SwapFailed)
		}
	} else {
		log(data, fmt.Sprintf("Received redeemed callback in wrong state %v", sd.State), nil)
	}
}

// FsmWrap will just wrap a normal state machine function and give it the ability to transition states based on return values
func FsmWrap[I common.FsmInGetter, O common.FsmOutGetter](f func(data I) O, ChangeStateFn data.ChangeStateFn) func(data I) O {
	return func(in I) O {

		realIn := in.Get()
		out := f(in)
		realOut := out.Get()

		logger := NewLogEntry(in.Get().SwapData)

		if realOut.Error != nil {
			realOut.NextState = common.SwapFailed

			glog.Infof("[%v] [%d] Error %v happened", realIn.SwapData.PluginName, realIn.GetJobID(), realOut.Error)
			if realIn.MsgCallback != nil {
				realIn.MsgCallback(entities.PluginMessage{
					JobID:      int64(realIn.GetJobID()),
					Message:    fmt.Sprintf("Error %v happened", realOut.Error),
					Data:       logger.Get("error", realOut.Error.Error()),
					IsError:    true,
					IsFinished: true,
				})
			}
			return out
		}

		if realIn.SwapData.State.IsFinal() {
			return out
		}
		if realOut.NextState != common.None {
			err := ChangeStateFn(realIn, realOut.NextState)
			glog.Infof("[%v] [%d] Transitioning to state %v", realIn.SwapData.PluginName, realIn.GetJobID(), realOut.NextState)
			if realIn.MsgCallback != nil {
				realIn.MsgCallback(entities.PluginMessage{
					JobID:      int64(realIn.GetJobID()),
					Message:    fmt.Sprintf("Transitioning to state %v", realOut.NextState),
					Data:       logger.Get(),
					IsError:    false,
					IsFinished: false,
				})
			}
			if err != nil {
				realOut.NextState = common.SwapFailed
			}
		}

		return out
	}
}

// Swapmachine is a finite state machine used for swaps.
type SwapMachine struct {
	Machine *common.Fsm[common.FsmIn, common.FsmOut, common.State]

	ReferralCode    string
	ChainParams     *chaincfg.Params
	Filter          filter.FilteringInterface
	CryptoAPI       *crypto.CryptoAPI
	Redeemer        *redeemer.Redeemer[common.FsmIn]
	ReverseRedeemer *redeemer.Redeemer[common.FsmIn]
	BoltzAPI        *bapi.BoltzPrivateAPI
	ChangeStateFn   data.ChangeStateFn
	GetSleepTimeFn  data.GetSleepTimeFn
	DeleteJobFn     data.DeleteJobFn

	NodeDataInvalidator entities.Invalidatable
	JobDataToSwapData   common.JobDataToSwapDataFn
	LnAPI               api.NewAPICall

	AllowChanCreation   bool
	PrivateChanCreation bool
}

func NewSwapMachine(plugin data.PluginData, nodeDataInvalidator entities.Invalidatable, jobDataToSwapData common.JobDataToSwapDataFn, lnAPI api.NewAPICall) *SwapMachine {
	fn := plugin.ChangeStateFn

	s := &SwapMachine{Machine: &common.Fsm[common.FsmIn, common.FsmOut, common.State]{States: make(map[common.State]func(data common.FsmIn) common.FsmOut)},
		NodeDataInvalidator: nodeDataInvalidator,
		JobDataToSwapData:   jobDataToSwapData,
		LnAPI:               lnAPI,
		ReferralCode:        plugin.ReferralCode,
		ChainParams:         plugin.ChainParams,
		Filter:              plugin.Filter,
		CryptoAPI:           plugin.CryptoAPI,
		Redeemer:            plugin.Redeemer,
		ReverseRedeemer:     plugin.ReverseRedeemer,
		BoltzAPI:            plugin.BoltzAPI,
		ChangeStateFn:       fn,
		GetSleepTimeFn:      plugin.GetSleepTimeFn,
		DeleteJobFn:         plugin.DeleteJobFn,
		AllowChanCreation:   plugin.AllowChanCreation,
		PrivateChanCreation: plugin.PrivateChanCreation,
	}

	s.Machine.States[common.SwapFailed] = FsmWrap(s.FsmSwapFailed, fn)
	s.Machine.States[common.SwapSuccess] = FsmWrap(s.FsmSwapSuccess, fn)
	s.Machine.States[common.SwapSuccessOne] = FsmWrap(s.FsmSwapSuccessOne, fn)

	s.Machine.States[common.InitialForward] = FsmWrap(s.FsmInitialForward, fn)
	s.Machine.States[common.OnChainFundsSent] = FsmWrap(s.FsmOnChainFundsSent, fn)
	s.Machine.States[common.RedeemLockedFunds] = FsmWrap(s.FsmRedeemLockedFunds, fn)
	s.Machine.States[common.RedeemingLockedFunds] = FsmWrap(s.FsmRedeemingLockedFunds, fn)
	s.Machine.States[common.VerifyFundsReceived] = FsmWrap(s.FsmVerifyFundsReceived, fn)

	s.Machine.States[common.InitialReverse] = FsmWrap(s.FsmInitialReverse, fn)
	s.Machine.States[common.ReverseSwapCreated] = FsmWrap(s.FsmReverseSwapCreated, fn)
	s.Machine.States[common.SwapInvoiceCouldNotBePaid] = FsmWrap(s.FsmSwapInvoiceCouldNotBePaid, fn)
	s.Machine.States[common.ClaimReverseFunds] = FsmWrap(s.FsmClaimReverseFunds, fn)
	s.Machine.States[common.SwapClaimed] = FsmWrap(s.FsmSwapClaimed, fn)

	return s
}

func (s *SwapMachine) Eval(in common.FsmIn, initial common.State) common.FsmOut {
	initF, ok := s.Machine.States[initial]
	if !ok {
		return common.FsmOut{Error: fmt.Errorf("invalid initial state: %v", initial)}
	}
	fail := s.FsmNone
	return s.Machine.FsmEval(in, initF, fail)
}
