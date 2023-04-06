//go:build plugins
// +build plugins

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJobData(t *testing.T) {
	jd, err := ParseJobData(1337, []byte(`{ "id": 1338, "target": "OutboundLiquidityNodePercent", "amount": 12.3}`))
	require.NoError(t, err)
	assert.Equal(t, 1337, int(jd.ID))

	_, err = ParseJobData(1337, []byte(`{ "target": "SpecialTargetType", "amount": 12.3}`))
	assert.ErrorIs(t, err, ErrUnsupportedTargetType)

	_, err = ParseJobData(1337, []byte(`{ "target": "InboundLiquidityNodePercent", "amount": 100}`))
	assert.ErrorIs(t, err, ErrCouldNotParseJobData)

	_, err = ParseJobData(2001, []byte(`{ "target": "InboundLiquidityChannelPercent", "amount": 55}`))
	assert.ErrorIs(t, err, ErrCouldNotParseJobData)

	jd, err = ParseJobData(2001, []byte(`{ "target": "InboundLiquidityChannelPercent", "amount": 55, "channel_id": 12345 }`))
	require.NoError(t, err)
	assert.Equal(t, uint64(12345), jd.ChannelId)

	jd, err = ParseJobData(1, []byte(`{"amount": 50, "target": "InboundLiquidityChannelPercent", "channel_id": 128642860515328}`))
	require.NoError(t, err)
	assert.Equal(t, uint64(128642860515328), jd.ChannelId)
	assert.Equal(t, float64(50), jd.Amount)
}
