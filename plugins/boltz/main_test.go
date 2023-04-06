//go:build plugins
// +build plugins

package boltz

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

	agent_entities "github.com/bolt-observer/agent/entities"
	api "github.com/bolt-observer/agent/lightning"
	bapi "github.com/bolt-observer/agent/plugins/boltz/api"
	common "github.com/bolt-observer/agent/plugins/boltz/common"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func mkFakeLndAPI() api.NewAPICall {
	return func() (api.LightingAPICalls, error) {
		return &api.MockLightningAPI{}, nil
	}
}
func TestExecute(t *testing.T) {
	target := []byte(`{ "target": "DummyTarget" }`)

	p := &Plugin{
		BoltzAPI:    bapi.NewBoltzPrivateAPI("https://testapi.boltz.exchange", nil),
		ChainParams: &chaincfg.TestNet3Params,
		LnAPI:       mkFakeLndAPI(),
		jobs:        make(map[int32]interface{}),
		db: &TestDB{data: map[interface{}]interface{}{
			int32(42): target,
		}},
		Limits: common.SwapLimits{
			MaxAttempts: 10,
		},
	}

	t.Run("Error on invalid data", func(t *testing.T) {
		cs := &CallbackStore{}
		err := p.Execute(123, []byte("invalid data"), cs.Callback)
		require.ErrorIs(t, common.ErrCouldNotParseJobData, err)
		// We now report the error back
		assert.Equal(t, 1, len(cs.messages))
	})

	t.Run("Ack if message is running already", func(t *testing.T) {
		cs := &CallbackStore{}
		p.jobs[123] = make(chan agent_entities.PluginMessage)
		defer func() {
			delete(p.jobs, 123)
		}()

		err := p.Execute(123, target, cs.Callback)
		require.NoError(t, err)
		assert.Equal(t, 0, len(cs.messages))
	})

	t.Run("Add new job to database", func(t *testing.T) {
		cs := &CallbackStore{}
		defer func() {
			delete(p.jobs, 123)
		}()

		err := p.Execute(123, target, cs.Callback)
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

		err := p.Execute(42, target, cs.Callback)
		require.NoError(t, err)

		_, ok := p.jobs[int32(42)]
		assert.True(t, ok)
	})
}
