package actions

import (
	"context"
	"errors"
	"fmt"
	"io"

	api "github.com/bolt-observer/agent/actions/bolt-observer-api"
	"github.com/golang/glog"

	"github.com/bolt-observer/agent/entities"
)

// type LnInfoApi interface {
// 	GetInfo(ctx context.Context) (*lnAPI.InfoAPI, error)
// }

type Connector struct {
	address string
	key     string
	client  api.ActionAPI_SubscribeClient
	lnApi   entities.NewAPICall
	plugins map[string]entities.Plugin
}

func NewConnector(url, key string, plugins map[string]entities.Plugin, lnApi entities.NewAPICall) Connector {
	return Connector{
		address: url,
		key:     key,
		lnApi:   lnApi,
		plugins: plugins,
	}
}

func (c *Connector) Run(ctx context.Context) error {
	pubkey, err := c.getPubKey(ctx)
	if err != nil {
		return err
	}
	client, connection, err := NewAPIClient(c.address, pubkey, c.key)
	if err != nil {
		return err
	}
	defer connection.Close()

	stream, err := client.Subscribe(ctx)
	if err != nil {
		return err
	}
	c.client = stream

	return c.communicate(ctx, stream)
}

type ActionStreamer interface {
	Send(*api.AgentReply) error
	Recv() (*api.Action, error)
}

func (c *Connector) communicate(ctx context.Context, stream ActionStreamer) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			msg, err := stream.Recv()
			switch {
			case errors.Is(err, io.EOF):
				glog.Info("Server requested graceful shutdown")
				return nil
			case err != nil:
				glog.Error("Error while receiving message: ", err)
				return err
			case msg.Sequence == api.Sequenece_CONNECT:
				// first request is validation of the connection and credentials
				glog.Info("Successfully connected to the server")
				break
			case msg.Sequence == api.Sequenece_EXECUTE:
				plugin, ok := c.plugins[msg.Action]
				if !ok {
					stream.Send(&api.AgentReply{
						JobId:    msg.JobId,
						Sequence: api.Sequenece_EXECUTE,
						Type:     api.ReplyType_ERROR,
						Message:  fmt.Sprintf("Plugin %s not found on agent", msg.Action),
					})
					break
				}

				if err = plugin.Execute(ctx, msg.Data, c.ForwardJobMessages); err != nil {
					stream.Send(&api.AgentReply{
						JobId:    msg.JobId,
						Sequence: api.Sequenece_EXECUTE,
						Type:     api.ReplyType_ERROR,
						Message:  err.Error(),
					})
					break
				} else {
					// ack
					stream.Send(&api.AgentReply{
						JobId:    msg.JobId,
						Sequence: api.Sequenece_EXECUTE,
						Type:     api.ReplyType_SUCCESS,
					})
				}
			}
		}
	}
}

func (c *Connector) ForwardJobMessages(msg entities.PluginMessage) error {
	replyType := api.ReplyType_SUCCESS
	if msg.IsError {
		replyType = api.ReplyType_ERROR
	}

	sequence := api.Sequenece_LOG
	if msg.IsFinished {
		sequence = api.Sequenece_FINISHED
	}

	if err := c.client.SendMsg(&api.AgentReply{
		JobId:    msg.JobID,
		Sequence: sequence,
		Type:     replyType,
		Message:  msg.Message,
		Data:     msg.Data,
	}); err != nil {
		glog.Error("Error while sending message: ", err)
		return err
	}

	return nil
}

func (c *Connector) getPubKey(ctx context.Context) (string, error) {
	lnApi := c.lnApi()
	if lnApi == nil {
		return "", fmt.Errorf("lightning API not obtained")
	}

	info, err := lnApi.GetInfo(ctx)
	if err != nil {
		return "", err
	}

	return info.IdentityPubkey, nil
}
