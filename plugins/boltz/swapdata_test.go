//go:build plugins
// +build plugins

package boltz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertOutBoundLiqudityNodePercent(t *testing.T) {
	limits := &SwapLimits{
		MinSwap:       100_000,
		DefaultSwap:   100_000,
		MaxSwap:       1_000_000,
		AllowZeroConf: true,
	}

	t.Run("Everything on inbound side want everyting outbound", func(t *testing.T) {
		jd := &JobData{
			Target: OutboundLiquidityNodePercent,
			Amount: 100,
			ID:     1337,
		}

		liquidity := &Liquidity{
			InboundSats:        200000,
			OutboundSats:       0,
			Capacity:           200000,
			InboundPercentage:  100,
			OutboundPercentage: 0,
		}

		result, err := convertLiquidityNodePercent(jd, limits, liquidity, nil, true)
		assert.NoError(t, err)

		assert.Equal(t, JobID(1337), result.JobID)
		assert.Equal(t, InitialForward, result.State)
		assert.Equal(t, 200000, int(result.Sats))
		assert.Equal(t, *jd, result.OriginalJobData)
	})

	t.Run("Everything on inbound side want half outbound", func(t *testing.T) {
		jd := &JobData{
			Target: OutboundLiquidityNodePercent,
			Amount: 50,
			ID:     1338,
		}

		liquidity := &Liquidity{
			InboundSats:        200000,
			OutboundSats:       0,
			Capacity:           200000,
			InboundPercentage:  100,
			OutboundPercentage: 0,
		}

		result, err := convertLiquidityNodePercent(jd, limits, liquidity, nil, true)
		assert.NoError(t, err)

		assert.Equal(t, JobID(1338), result.JobID)
		assert.Equal(t, InitialForward, result.State)
		assert.Equal(t, 100000, int(result.Sats))
		assert.Equal(t, *jd, result.OriginalJobData)
	})

	t.Run("Empty node", func(t *testing.T) {
		jd := &JobData{
			Target: OutboundLiquidityNodePercent,
			Amount: 50,
			ID:     1339,
		}

		liquidity := &Liquidity{
			InboundSats:        0,
			OutboundSats:       0,
			Capacity:           0,
			InboundPercentage:  0,
			OutboundPercentage: 0,
		}

		result, err := convertLiquidityNodePercent(jd, limits, liquidity, nil, true)
		assert.NoError(t, err)

		assert.Equal(t, JobID(1339), result.JobID)
		assert.Equal(t, InitialForward, result.State)
		assert.Equal(t, 100000, int(result.Sats))
		assert.Equal(t, *jd, result.OriginalJobData)
	})
}
