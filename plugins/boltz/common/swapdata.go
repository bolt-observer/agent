//go:build plugins
// +build plugins

package common

import (
	"context"
	"fmt"
	"math"

	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/bolt-observer/agent/lightning"
	"github.com/golang/glog"
)

// SwapData struct.
type SwapData struct {
	JobID             JobID
	Attempt           int
	BoltzID           string
	State             State
	TimoutBlockHeight uint32
	Script            string
	Sats              uint64
	Address           string

	// Normal swap
	Invoice             string
	LockupTransactionId string

	// Reverse swap
	ReverseInvoice   string
	ReverseChannelId uint64 // 0 means node level
	//ReverseMinerInvoice string - not supported
	ChanIdsToUse []uint64
	ExpectedSats uint64

	SwapLimits SwapLimits

	IsDryRun        bool
	OriginalJobData JobData // original job

	TransactionHex string

	FeesSoFar   Fees
	FeesPending Fees
}

func (s *SwapData) CommitFees() {
	s.FeesSoFar.FeesPaid += s.FeesPending.FeesPaid
	s.FeesSoFar.SatsSwapped += s.FeesPending.SatsSwapped

	s.FeesPending.FeesPaid = 0
	s.FeesPending.SatsSwapped = 0
}

func (s *SwapData) RevertFees() {
	s.FeesPending.FeesPaid = 0
	s.FeesPending.SatsSwapped = 0
}

// Fees struct
type Fees struct {
	SatsSwapped uint64
	FeesPaid    uint64
}

type JobDataToSwapDataFn func(ctx context.Context, limits SwapLimits, jobData *JobData, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) (*SwapData, error)

func JobDataToSwapData(ctx context.Context, limits SwapLimits, jobData *JobData, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) (*SwapData, error) {
	if jobData == nil {
		return nil, fmt.Errorf("empty job data")
	}

	if jobData.MaxFeePercentage != 0 {
		limits.MaxFeePercentage = jobData.MaxFeePercentage
	}

	switch jobData.Target {
	case OutboundLiquidityNodePercent:
		liquidity := getLiquidity(ctx, jobData, msgCallback, lnAPI, filter)
		if liquidity == nil {
			return nil, fmt.Errorf("could not get liquidity")
		}
		return convertLiquidityNodePercent(jobData, limits, liquidity, msgCallback, true)
	case InboundLiquidityNodePercent:
		liquidity := getLiquidity(ctx, jobData, msgCallback, lnAPI, filter)
		if liquidity == nil {
			return nil, fmt.Errorf("could not get liquidity")
		}
		return convertLiquidityNodePercent(jobData, limits, liquidity, msgCallback, false)
	case InboundLiquidityChannelPercent:
		return convertInboundLiquidityChanPercent(ctx, jobData, limits, msgCallback, lnAPI, filter)
	case DummyTarget:
		return &SwapData{Attempt: limits.MaxAttempts + 1}, nil
	default:
		// Not supported yet
		return nil, fmt.Errorf("not supported")
	}
}

func (sd *SwapData) GetUniqueJobID() string {
	return fmt.Sprintf("%d-%d", sd.JobID, sd.Attempt)
}

func getLiquidity(ctx context.Context, jobData *JobData, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) *Liquidity {
	liquidity, err := GetNodeLiquidity(ctx, lnAPI, filter)

	if err != nil {
		glog.Infof("[Boltz] [%d] Could not get liquidity", jobData.ID)
		if msgCallback != nil {
			msgCallback(agent_entities.PluginMessage{
				JobID:      jobData.ID,
				Message:    "Could not get liquidity",
				IsError:    true,
				IsFinished: true,
			})
		}
		return nil
	}

	return liquidity
}

func convertInboundLiquidityChanPercent(ctx context.Context, jobData *JobData, limits SwapLimits, msgCallback agent_entities.MessageCallback, lnAPI lightning.LightingAPICalls, filter filter.FilteringInterface) (*SwapData, error) {
	liquidity, total, err := GetChanLiquidity(ctx, jobData.ChannelId, 0, false, lnAPI, filter)
	if err != nil {
		glog.Infof("[Boltz] [%d] Could not get liquidity", jobData.ID)
		if msgCallback != nil {
			msgCallback(agent_entities.PluginMessage{
				JobID:      jobData.ID,
				Message:    "Could not get liquidity",
				IsError:    true,
				IsFinished: true,
			})
		}
		return nil, fmt.Errorf("could not get liquidity %v", err)
	}

	ratio := float64(liquidity.Capacity) / float64(total)

	glog.Infof("[Boltz] [%d] Ratio for channel %d is %f", jobData.ID, jobData.ChannelId, ratio)

	if ratio*100 > jobData.Amount || jobData.Amount < 0 || jobData.Amount > 100 {
		glog.Infof("[Boltz] [%v] No need to do anything - current inbound liquidity %v %% for channel %v", jobData.ID, ratio*100, jobData.ChannelId)
		if msgCallback != nil {
			msgCallback(agent_entities.PluginMessage{
				JobID:      jobData.ID,
				Message:    fmt.Sprintf("No need to do anything - current inbound liquidity %v %% for channel %v", ratio*100, jobData.ChannelId),
				IsError:    false,
				IsFinished: true,
			})
		}
		return nil, ErrNoNeedToDoAnything
	}

	factor := ((jobData.Amount / 100) - ratio)
	sats := float64(total) * factor
	if sats < float64(limits.MinSwap) && limits.BelowMinAmountIsSuccess {
		glog.Infof("[Boltz] [%v] Cannot do anything - current inbound liquidity %v %% for channel %v we would swap %v which is below lower amount", jobData.ID, ratio*100, jobData.ChannelId, sats)
		if msgCallback != nil {
			msgCallback(agent_entities.PluginMessage{
				JobID:      jobData.ID,
				Message:    fmt.Sprintf("Cannot do anything - current inbound liquidity %v %% for channel %v we would swap %v which is below lower amount", ratio*100, jobData.ChannelId, sats),
				IsError:    false,
				IsFinished: true,
			})
		}
		return nil, ErrNoNeedToDoAnything
	}
	sats = math.Min(math.Max(sats, float64(limits.MinSwap)), float64(limits.MaxSwap))

	return &SwapData{
		JobID:            JobID(jobData.ID),
		Sats:             uint64(math.Round(sats)),
		State:            InitialReverse,
		ReverseChannelId: jobData.ChannelId,
		OriginalJobData:  *jobData,
		Attempt:          1,
		FeesSoFar:        Fees{},
		FeesPending:      Fees{},
		SwapLimits:       limits,
	}, nil
}

func convertLiquidityNodePercent(jobData *JobData, limits SwapLimits, liquidity *Liquidity, msgCallback agent_entities.MessageCallback, outbound bool) (*SwapData, error) {
	val := liquidity.OutboundPercentage
	name := "outbound"
	if !outbound {
		val = liquidity.InboundPercentage
		name = "inbound"
	}

	if val > jobData.Amount || jobData.Amount < 0 || jobData.Amount > 100 {
		glog.Infof("[Boltz] [%v] No need to do anything - current %s liquidity %v %%", jobData.ID, name, val)
		if msgCallback != nil {
			msgCallback(agent_entities.PluginMessage{
				JobID:      jobData.ID,
				Message:    fmt.Sprintf("No need to do anything - current %s liquidity %v %%", name, val),
				IsError:    false,
				IsFinished: true,
			})
		}
		return nil, ErrNoNeedToDoAnything
	}

	sats := float64(limits.DefaultSwap)
	if liquidity.Capacity != 0 {
		factor := (jobData.Amount - val) / float64(100)
		glog.Infof("[Boltz] [%d] Factor for node is %f", jobData.ID, factor)
		sats = float64(liquidity.Capacity) * factor
		if sats < float64(limits.MinSwap) && limits.BelowMinAmountIsSuccess {
			glog.Infof("[Boltz] [%v] Cannot do anything - we would swap %v sat which is below lower amount", jobData.ID, sats)
			if msgCallback != nil {
				msgCallback(agent_entities.PluginMessage{
					JobID:      jobData.ID,
					Message:    fmt.Sprintf("Cannot do anything - we would swap %v sats which is below lower amount", sats),
					IsError:    false,
					IsFinished: true,
				})
			}
			return nil, ErrNoNeedToDoAnything
		}

		sats = math.Min(math.Max(sats, float64(limits.MinSwap)), float64(limits.MaxSwap))
	}

	s := &SwapData{
		JobID:            JobID(jobData.ID),
		Sats:             uint64(math.Round(sats)),
		ReverseChannelId: 0,
		Attempt:          1,
		OriginalJobData:  *jobData,
		FeesSoFar:        Fees{},
		FeesPending:      Fees{},
		SwapLimits:       limits,
	}

	if outbound {
		s.State = InitialForward
	} else {
		s.State = InitialReverse
	}

	return s, nil
}
