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

func (tp *testPlugin) Execute(jobID int32, data []byte, msg entities.MessageCallback) error {
	if tp.shouldFail {
		return errors.New("Could not execute")
	}
	return nil
}

type blockingStream struct {
	msg          *api.Action
	sent         []*api.AgentReply
	receiveError error
}

func (b *blockingStream) Recv() (*api.Action, error) {
	return b.msg, b.receiveError
}

func (b *blockingStream) Send(r *api.AgentReply) error {
	b.sent = append(b.sent, r)
	return nil
}

func mkGetLndAPI(cmdCtx *cli.Context) lnapi.NewAPICall {
	return func() (lnapi.LightingAPICalls, error) {
		return lnapi.NewAPI(lnapi.LndGrpc, func() (*common_entities.Data, error) {
			return &common_entities.Data{
				PubKey: "test-pubkey",
			}, nil
		})
	}
}

type spyCaller struct {
	called bool
}

func (s *spyCaller) call() {
	s.called = true
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
		IsPlaintext: true,
		IsDryRun:   false,
	}

	t.Run("Test execute unknown plugin", func(t *testing.T) {
		sc := spyCaller{}
		ctx, cancel := context.WithCancel(context.Background())
		bs := blockingStream{
			msg: &api.Action{
				JobId:    5,
				Action:   "unknown",
				Sequence: api.Sequence_EXECUTE,
			},
			sent: []*api.AgentReply{},
		}

		go cc.communicate(ctx, &bs, sc.call)

		time.Sleep(10 * time.Millisecond)
		cancel()

		assert.Equal(t, &api.AgentReply{
			JobId:    5,
			Sequence: api.Sequence_EXECUTE,
			Type:     api.ReplyType_ERROR,
			Message:  "Plugin unknown not found on agent",
		}, bs.sent[0])
		assert.True(t, sc.called)
	})

	t.Run("Test execute test plugin", func(t *testing.T) {
		sc := spyCaller{}
		ctx, cancel := context.WithCancel(context.Background())
		bs := blockingStream{
			msg: &api.Action{
				JobId:    5,
				Action:   "test",
				Sequence: api.Sequence_EXECUTE,
			},
			sent: []*api.AgentReply{},
		}

		go cc.communicate(ctx, &bs, sc.call)

		time.Sleep(10 * time.Millisecond)
		cancel()

		assert.Equal(t, &api.AgentReply{
			JobId:    5,
			Sequence: api.Sequence_EXECUTE,
			Type:     api.ReplyType_SUCCESS,
		}, bs.sent[0])
		assert.True(t, sc.called)
	})

	t.Run("Test execute test plugin fails", func(t *testing.T) {
		sc := spyCaller{}
		ctx, cancel := context.WithCancel(context.Background())
		bs := blockingStream{
			msg: &api.Action{
				JobId:    5,
				Action:   "fail",
				Sequence: api.Sequence_EXECUTE,
			},
			sent: []*api.AgentReply{},
		}

		go cc.communicate(ctx, &bs, sc.call)

		time.Sleep(10 * time.Millisecond)
		cancel()

		assert.Equal(t, &api.AgentReply{
			JobId:    5,
			Sequence: api.Sequence_EXECUTE,
			Type:     api.ReplyType_ERROR,
			Message:  "Could not execute",
		}, bs.sent[0])
		assert.True(t, sc.called)
	})

	t.Run("Error receiveing from stream", func(t *testing.T) {
		sc := spyCaller{}
		ctx, cancel := context.WithCancel(context.Background())
		bs := blockingStream{
			msg: &api.Action{
				JobId:    5,
				Action:   "test",
				Sequence: api.Sequence_EXECUTE,
			},
			sent:         []*api.AgentReply{},
			receiveError: errors.New("some error"),
		}
		c := Connector{
			Address:    "http://some.url/",
			APIKey:     "test-key",
			Plugins:    plugins,
			LnAPI:      mkGetLndAPI(&cli.Context{}),
			IsPlaintext: true,
			IsDryRun:   false,
		}

		go c.communicate(ctx, &bs, sc.call)

		time.Sleep(10 * time.Millisecond)
		cancel()

		assert.Len(t, bs.sent, 0)
		assert.False(t, sc.called)
	})
}
