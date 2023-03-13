//go:build plugins
// +build plugins

package boltz

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	agent_entities "github.com/bolt-observer/agent/entities"
	lnapi "github.com/bolt-observer/agent/lightning"
	common_entities "github.com/bolt-observer/go_common/entities"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

type CallbackStore struct {
	messages []agent_entities.PluginMessage
}

func (c *CallbackStore) Callback(msg agent_entities.PluginMessage) error {
	c.messages = append(c.messages, msg)
	return nil
}

type TestDB struct {
	data map[interface{}]interface{}
}

func (t *TestDB) Get(key, value interface{}) error {
	data, ok := t.data[key]
	if !ok {
		return fmt.Errorf("Key not found")
	}

	var buff bytes.Buffer
	de := gob.NewDecoder(&buff)

	_, err := buff.Write(data.([]byte))
	if err != nil {
		return err
	}

	return de.Decode(value)
}

func (t *TestDB) Insert(key, data interface{}) error {
	t.data[key] = data
	return nil
}

func (t *TestDB) Update(key, data interface{}) error {
	t.data[key] = data
	return nil
}

func (t *TestDB) Connect(path string) error {
	return nil
}

func TestExecute(t *testing.T) {
	p := &Plugin{
		BoltzAPI:    &boltz.Boltz{URL: "https://testapi.boltz.exchange"},
		ChainParams: &chaincfg.TestNet3Params,
		LnAPI:       mkGetLndAPI(&cli.Context{}),
		jobs:        make(map[int32]interface{}),
		db: &TestDB{data: map[interface{}]interface{}{
			int32(42): []byte(`{"target":"Swap"}`),
		}},
	}

	t.Run("Error on invalid data", func(t *testing.T) {
		cs := &CallbackStore{}
		err := p.Execute(123, []byte("invalid data"), cs.Callback)
		require.ErrorIs(t, ErrCouldNotParseJobData, err)
		assert.Equal(t, 0, len(cs.messages))
	})

	t.Run("Ack if message is running already", func(t *testing.T) {
		cs := &CallbackStore{}
		p.jobs[123] = make(chan agent_entities.PluginMessage)
		defer func() {
			delete(p.jobs, 123)
		}()

		err := p.Execute(123, []byte(`{"target":"Swap"}`), cs.Callback)
		require.NoError(t, err)
		assert.Equal(t, 0, len(cs.messages))
	})

	t.Run("Add new job to database", func(t *testing.T) {
		cs := &CallbackStore{}
		defer func() {
			delete(p.jobs, 123)
		}()

		err := p.Execute(123, []byte(`{"target":"Swap"}`), cs.Callback)
		require.NoError(t, err)

		_, ok := p.jobs[123]
		assert.True(t, ok)

		fmt.Printf("%+v\n", p.db.(*TestDB).data)
		_, ok = p.db.(*TestDB).data[int32(123)]
		assert.True(t, ok)
	})

	t.Run("Continue executing existing job from the database", func(t *testing.T) {
		cs := &CallbackStore{}
		defer func() {
			delete(p.jobs, 42)
		}()

		err := p.Execute(42, []byte(`{"target":"Swap"}`), cs.Callback)
		require.NoError(t, err)

		_, ok := p.jobs[int32(42)]
		assert.True(t, ok)
	})

}

func mkGetLndAPI(cmdCtx *cli.Context) agent_entities.NewAPICall {
	return func() (lnapi.LightingAPICalls, error) {
		return lnapi.NewAPI(lnapi.LndGrpc, func() (*common_entities.Data, error) {
			return &common_entities.Data{
				PubKey: "test-pubkey",
			}, nil
		})
	}
}

func TestConvertOutBoundLiqudityNodePercent(t *testing.T) {
	p := &Plugin{}
	limits := &SwapLimits{
		MinSwap:       100_000,
		DefaultSwap:   100_000,
		MaxSwap:       1_000_000,
		AllowZeroConf: true,
	}

	t.Run("Everything on inbound side want everyting outbound", func(t *testing.T) {
		jd := &JobData{
			Target:     OutboundLiquidityNodePercent,
			Percentage: 100,
			ID:         1337,
		}

		liquidity := &Liquidity{
			InboundSats:        200000,
			OutboundSats:       0,
			Capacity:           200000,
			InboundPercentage:  100,
			OutboundPercentage: 0,
		}

		result := p.convertLiquidityNodePercent(jd, limits, liquidity, nil, true)

		assert.Equal(t, JobID(1337), result.JobID)
		assert.Equal(t, InitialForward, result.State)
		assert.Equal(t, 200000, int(result.Sats))
	})

	t.Run("Everything on inbound side want half outbound", func(t *testing.T) {
		jd := &JobData{
			Target:     OutboundLiquidityNodePercent,
			Percentage: 50,
			ID:         1338,
		}

		liquidity := &Liquidity{
			InboundSats:        200000,
			OutboundSats:       0,
			Capacity:           200000,
			InboundPercentage:  100,
			OutboundPercentage: 0,
		}

		result := p.convertLiquidityNodePercent(jd, limits, liquidity, nil, true)

		assert.Equal(t, JobID(1338), result.JobID)
		assert.Equal(t, InitialForward, result.State)
		assert.Equal(t, 100000, int(result.Sats))
	})

	t.Run("Empty node", func(t *testing.T) {
		jd := &JobData{
			Target:     OutboundLiquidityNodePercent,
			Percentage: 50,
			ID:         1339,
		}

		liquidity := &Liquidity{
			InboundSats:        0,
			OutboundSats:       0,
			Capacity:           0,
			InboundPercentage:  0,
			OutboundPercentage: 0,
		}

		result := p.convertLiquidityNodePercent(jd, limits, liquidity, nil, true)

		assert.Equal(t, JobID(1339), result.JobID)
		assert.Equal(t, InitialForward, result.State)
		assert.Equal(t, 100000, int(result.Sats))
	})
}
