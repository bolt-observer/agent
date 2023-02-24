package actions

import (
	"context"
	"errors"
	"testing"
	"time"

	api "github.com/bolt-observer/agent/actions/bolt-observer-api"
	lnapi "github.com/bolt-observer/agent/lightning"

	"github.com/bolt-observer/agent/entities"
	common_entities "github.com/bolt-observer/go_common/entities"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"
)

type testPlugin struct {
	shouldFail bool
}

func (tp *testPlugin) Execute(jobID int32, data []byte, msg func(entities.PluginMessage) error) error {
	if tp.shouldFail {
		return errors.New("Could not execute")
	}
	return nil
}

type blockingStream struct {
	msg  *api.Action
	sent []*api.AgentReply
}

func (b *blockingStream) Recv() (*api.Action, error) {
	return b.msg, nil
}

func (b *blockingStream) Send(r *api.AgentReply) error {
	b.sent = append(b.sent, r)
	return nil
}

func mkGetLndAPI(cmdCtx *cli.Context) entities.NewAPICall {
	return func() lnapi.LightingAPICalls {
		return lnapi.NewAPI(lnapi.LndGrpc, func() (*common_entities.Data, error) {
			return &common_entities.Data{
				PubKey: "test-pubkey",
			}, nil
		})
	}
}

func TestCommunicate(t *testing.T) {
	plugins := map[string]entities.Plugin{
		"test": &testPlugin{},
		"fail": &testPlugin{shouldFail: true},
	}
	cc := Connector{
		Address:    "http://some.url/",
		APIKey:     "test-key",
		Plugins:    plugins,
		LnAPI:      mkGetLndAPI(&cli.Context{}),
		IsInsecure: true,
		IsDryRun:   false,
	}

	t.Run("Test execute unknown plugin", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		bs := blockingStream{
			msg: &api.Action{
				JobId:    5,
				Action:   "unknown",
				Sequence: api.Sequenece_EXECUTE,
			},
			sent: []*api.AgentReply{},
		}

		go cc.communicate(ctx, &bs)

		time.Sleep(10 * time.Millisecond)
		cancel()

		assert.Equal(t, &api.AgentReply{
			JobId:    5,
			Sequence: api.Sequenece_EXECUTE,
			Type:     api.ReplyType_ERROR,
			Message:  "Plugin unknown not found on agent",
		}, bs.sent[0])
	})

	t.Run("Test execute test plugin", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		bs := blockingStream{
			msg: &api.Action{
				JobId:    5,
				Action:   "test",
				Sequence: api.Sequenece_EXECUTE,
			},
			sent: []*api.AgentReply{},
		}

		go cc.communicate(ctx, &bs)

		time.Sleep(10 * time.Millisecond)
		cancel()

		assert.Equal(t, &api.AgentReply{
			JobId:    5,
			Sequence: api.Sequenece_EXECUTE,
			Type:     api.ReplyType_SUCCESS,
		}, bs.sent[0])
	})

	t.Run("Test execute test plugin fails", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		bs := blockingStream{
			msg: &api.Action{
				JobId:    5,
				Action:   "fail",
				Sequence: api.Sequenece_EXECUTE,
			},
			sent: []*api.AgentReply{},
		}

		go cc.communicate(ctx, &bs)

		time.Sleep(10 * time.Millisecond)
		cancel()

		assert.Equal(t, &api.AgentReply{
			JobId:    5,
			Sequence: api.Sequenece_EXECUTE,
			Type:     api.ReplyType_ERROR,
			Message:  "Could not execute",
		}, bs.sent[0])
	})
	t.Run("Test execute plugin in dry-run mode", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		bs := blockingStream{
			msg: &api.Action{
				JobId:    5,
				Action:   "test",
				Sequence: api.Sequenece_EXECUTE,
			},
			sent: []*api.AgentReply{},
		}
		c := Connector{
			Address:    "http://some.url/",
			APIKey:     "test-key",
			Plugins:    plugins,
			LnAPI:      mkGetLndAPI(&cli.Context{}),
			IsInsecure: true,
			IsDryRun:   true,
		}

		go c.communicate(ctx, &bs)

		time.Sleep(10 * time.Millisecond)
		cancel()

		assert.Equal(t, &api.AgentReply{
			JobId:    5,
			Sequence: api.Sequenece_EXECUTE,
			Type:     api.ReplyType_SUCCESS,
			Message:  `Agent received action "test" in dry-run mode. No action really taken`,
		}, bs.sent[0])
	})
}
