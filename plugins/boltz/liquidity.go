package boltz

import (
	"context"
	"fmt"
)

// Liquidity struct.
type Liquidity struct {
	InboundSats        uint64
	OutboundSats       uint64
	Capacity           uint64
	InboundPercentage  float64
	OutboundPercentage float64
}

// GetNodeLiquidity - gets current node liquidity
func (b *Plugin) GetNodeLiquidity() (*Liquidity, error) {
	lnAPI, err := b.LnAPI()
	if err != nil {
		return nil, err
	}
	if lnAPI == nil {
		return nil, fmt.Errorf("error checking lightning")
	}
	defer lnAPI.Cleanup()

	resp, err := lnAPI.GetChannels(context.Background())
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
