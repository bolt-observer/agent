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
func (b *Plugin) GetNodeLiquidity(ctx context.Context) (*Liquidity, error) {
	lnAPI, err := b.LnAPI()
	if err != nil {
		return nil, err
	}
	if lnAPI == nil {
		return nil, fmt.Errorf("error checking lightning")
	}
	defer lnAPI.Cleanup()

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

// GetByDescendingOutboundLiqudity - get channel in descreasing outbound capacity
func (b *Plugin) GetByDescendingOutboundLiqudity(ctx context.Context, limit uint64) ([]ChanCapacity, error) {
	lnAPI, err := b.LnAPI()
	if err != nil {
		return nil, err
	}
	if lnAPI == nil {
		return nil, fmt.Errorf("error checking lightning")
	}
	defer lnAPI.Cleanup()

	resp, err := lnAPI.GetChannels(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]ChanCapacity, 0)

	for _, channel := range resp.Channels {
		if !b.Filter.AllowChanID(channel.ChanID) && !b.Filter.AllowPubKey(channel.RemotePubkey) && !b.Filter.AllowSpecial(channel.Private) {
			continue
		}
		// Capacity is outbound here -> LocalBalance
		ret = append(ret, ChanCapacity{Capacity: channel.LocalBalance, Channel: channel})
	}

	// Sort by desceding capacity
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Capacity > ret[j].Capacity
	})

	idx := 0
	total := uint64(0)
	for _, one := range ret {
		total += one.Capacity
		if total >= limit {
			break
		}

		if one.Capacity == 0 {
			break
		}
		idx++
	}

	return ret[:idx], nil
}