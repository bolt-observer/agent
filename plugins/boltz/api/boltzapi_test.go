//go:build plugins
// +build plugins

package api

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	return
	api := NewBoltzPrivateAPI("https://boltz.exchange/api", nil)
	invoice := "lntb1m1pjyz4h9pp5u4mp7nc8t0kvulg2yjr7y4lksx29nhl4lrqsjptqpg996j96fg5qdqqcqzpgxqyz5vqsp57eglpcgvd7yr95hx9zkykufhq984wf2y022ypwta3q3gz3mj5jcs9qyyssq7t385ahjzrfr25g690mkgnupevccej0z6ty8e78f53lfmktrye6ytlex83lfdghrnr2shxqkvegwd0zadv8w7rxry3xf9wkgvsjr0zsqtrgazs"

	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)

	require.NoError(t, err)

	response, err := api.CreateSwap(CreateSwapRequestOverride{
		CreateSwapRequest: boltz.CreateSwapRequest{
			Type:            "submarine",
			PairId:          common.BtcPair,
			OrderSide:       "buy",
			PreimageHash:    hex.EncodeToString(bytes),
			RefundPublicKey: hex.EncodeToString(bytes),
			Invoice:         invoice,
		},
		ReferralId: "",
		/*
			Channel: CreateSwapRequestChannel{
				Auto: true,
			},
		*/
	})

	require.NoError(t, err)
	t.Logf("Response: %v", response)
	t.Fail()
}
