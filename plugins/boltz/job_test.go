//go:build plugins
// +build plugins

package boltz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseJobData(t *testing.T) {
	jd, err := ParseJobData(1337, []byte(`{ "id": 1338, "target": "OutboundLiquidityNodePercent", "percentage": 12.3}`))
	assert.NoError(t, err)
	assert.Equal(t, 1337, int(jd.ID))

	_, err = ParseJobData(1337, []byte(`{ "target": "SpecialTargetType", "percentage": 12.3}`))
	assert.ErrorIs(t, err, ErrUnsupportedTargetType)

	_, err = ParseJobData(1337, []byte(`{ "target": "InboundLiquidityNodePercent", "percentage": 100}`))
	assert.ErrorIs(t, err, ErrCouldNotParseJobData)

	_, err = ParseJobData(2001, []byte(`{ "target": "InboundLiquidityChannelPercent", "percentage": 55}`))
	assert.ErrorIs(t, err, ErrCouldNotParseJobData)

	jd, err = ParseJobData(2001, []byte(`{ "target": "InboundLiquidityChannelPercent", "percentage": 55, "channel_id": 12345 }`))
	assert.NoError(t, err)
	assert.Equal(t, uint64(12345), jd.ChannelId)
}
