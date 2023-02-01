package lnsocket

import (
	"testing"

	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/stretchr/testify/assert"
)

func TestFeatures(t *testing.T) {
	init := &lnwire.Init{}
	init.Features = lnwire.NewRawFeatureVector(lnwire.WumboChannelsOptional, lnwire.WumboChannelsRequired)
	init.GlobalFeatures = lnwire.NewRawFeatureVector()
	init.GlobalFeatures.Set(lnwire.AnchorsOptional)
	init.GlobalFeatures.Set(lnwire.GossipQueriesRequired)

	result := GetMandatoryFeatures(init)
	assert.NotNil(t, result)

	_, ok := result[lnwire.WumboChannelsRequired]
	assert.Equal(t, true, ok)
	_, ok = result[lnwire.GossipQueriesRequired]
	assert.Equal(t, true, ok)
	_, ok = result[lnwire.AnchorsOptional]
	assert.Equal(t, false, ok)
	_, ok = result[lnwire.WumboChannelsOptional]
	assert.Equal(t, false, ok)

	vector := ToRawFeatureVector(result)
	assert.NotNil(t, vector)

	assert.Equal(t, true, vector.IsSet(lnwire.WumboChannelsRequired))
	assert.Equal(t, true, vector.IsSet(lnwire.GossipQueriesRequired))
	assert.Equal(t, false, vector.IsSet(lnwire.AnchorsOptional))
}

func TestGenRandomBytes(t *testing.T) {
	const length = 10
	ret, err := GenRandomBytes(length)

	assert.NoError(t, err)
	assert.Equal(t, length, len(ret))
	cnt := 0
	for _, one := range ret {
		if one == 0 {
			cnt++
		}
	}
	assert.LessOrEqual(t, cnt, length-1, "Elements seem to be initialized with zero, bad randomness?!")
}
