//go:build plugins
// +build plugins

package boltz

import (
	"context"
	"fmt"
	"sort"

	"github.com/bolt-observer/agent/lightning"
)

// Liquidity struct.
type Liquidity struct {
	InboundSats        uint64
	OutboundSats       uint64
	Capacity           uint64
	InboundPercentage  float64
	OutboundPercentage float64
}

// ChanCapacity struct.
type ChanCapacity struct {
	Channel  lightning.ChannelAPI
	Capacity uint64
}

// GetNodeLiquidity - gets current node liquidity
func (b *Plugin) GetNodeLiquidity(ctx context.Context, optLnAPI lightning.LightingAPICalls) (*Liquidity, error) {
	var (
		lnAPI lightning.LightingAPICalls
		err   error
	)

	if optLnAPI != nil {
		lnAPI = optLnAPI
	} else {
		lnAPI, err = b.LnAPI()
		if err != nil {
			return nil, err
		}
		if lnAPI == nil {
			return nil, fmt.Errorf("could not get lightning api")
		}
		defer lnAPI.Cleanup()
	}

	resp, err := lnAPI.GetChannels(ctx)
	if err != nil {
		return nil, err
	}

	ret := &Liquidity{}
	ret.InboundSats = 0
	ret.OutboundSats = 0
	ret.Capacity = 0
	ret.InboundPercentage = 0.0
	ret.OutboundPercentage = 0.0

	for _, channel := range resp.Channels {
		if !b.Filter.AllowChanID(channel.ChanID) && !b.Filter.AllowPubKey(channel.RemotePubkey) && !b.Filter.AllowSpecial(channel.Private) {
			continue
		}

		ret.Capacity += channel.Capacity
		ret.OutboundSats += channel.LocalBalance
		ret.InboundSats += channel.RemoteBalance
	}

	if ret.Capacity > 0 {
		ret.InboundPercentage = float64(ret.InboundSats) / float64(ret.Capacity)
		ret.OutboundPercentage = float64(ret.OutboundSats) / float64(ret.Capacity)
	}

	return ret, nil
}

// GetByDescendingOutboundLiquidity - get channels in descreasing outbound liqudity so that sum >= limit satoshis
func (b *Plugin) GetByDescendingOutboundLiquidity(ctx context.Context, limit uint64, optLnAPI lightning.LightingAPICalls) ([]ChanCapacity, error) {
	var (
		lnAPI lightning.LightingAPICalls
		err   error
	)

	if optLnAPI != nil {
		lnAPI = optLnAPI
	} else {
		lnAPI, err = b.LnAPI()
		if err != nil {
			return nil, err
		}
		if lnAPI == nil {
			return nil, fmt.Errorf("could not get lightning api")
		}
		defer lnAPI.Cleanup()
	}

	resp, err := lnAPI.GetChannels(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]ChanCapacity, 0)

	for _, channel := range resp.Channels {
		if !b.Filter.AllowChanID(channel.ChanID) && !b.Filter.AllowPubKey(channel.RemotePubkey) && !b.Filter.AllowSpecial(channel.Private) {
			continue
		}
		// Capacity is outbound liquidity here -> LocalBalance
		ret = append(ret, ChanCapacity{Capacity: channel.LocalBalance, Channel: channel})
	}

	// Sort by descending capacity
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Capacity > ret[j].Capacity
	})

	idx := 0
	total := uint64(0)
	for _, one := range ret {
		total += one.Capacity
		if total >= limit {
			idx++
			break
		}

		if one.Capacity == 0 {
			break
		}
		idx++
	}

	if total < limit {
		return nil, fmt.Errorf("not enough capacity")
	}

	return ret[:idx], nil
}

// GetChanLiquidity
func (b *Plugin) GetChanLiquidity(ctx context.Context, chanID uint64, limit uint64, outbound bool, optLnAPI lightning.LightingAPICalls) (*ChanCapacity, uint64, error) {
	var (
		lnAPI lightning.LightingAPICalls
		err   error
	)

	fmt.Printf("DEBUG GetChanLiquidity %v %v", chanID, outbound)

	if optLnAPI != nil {
		lnAPI = optLnAPI
	} else {
		lnAPI, err = b.LnAPI()
		if err != nil {
			return nil, 0, err
		}
		if lnAPI == nil {
			return nil, 0, fmt.Errorf("could not get lightning api")
		}
		defer lnAPI.Cleanup()
	}

	resp, err := lnAPI.GetChannels(ctx)
	if err != nil {
		return nil, 0, err
	}
	for _, channel := range resp.Channels {
		if !b.Filter.AllowChanID(channel.ChanID) && !b.Filter.AllowPubKey(channel.RemotePubkey) && !b.Filter.AllowSpecial(channel.Private) {
			continue
		}
		if channel.ChanID != chanID {
			continue
		}
		liq := channel.LocalBalance
		if !outbound {
			liq = channel.RemoteBalance
		}
		fmt.Printf("DEBUG LIQ %v", liq)
		if limit > 0 && liq < limit {
			return nil, 0, fmt.Errorf("not enough capacity")
		}
		return &ChanCapacity{Capacity: liq, Channel: channel}, channel.Capacity, nil
	}

	return nil, 0, fmt.Errorf("channel %d not found", chanID)
}
