package actions

import (
	"context"
	"errors"
	"fmt"
	"io"

	api "github.com/bolt-observer/agent/actions/bolt-observer-api"
	"github.com/bolt-observer/agent/lightning"
	"github.com/cenkalti/backoff/v4"
	"github.com/golang/glog"
	"google.golang.org/grpc/metadata"

	"github.com/bolt-observer/agent/entities"
)

// Connector handles gRPC connection and communication with the server and route message to/from the plugins
type Connector struct {
	Address     string
	APIKey      string
	LnAPI       lightning.NewAPICall
	Plugins     map[string]entities.Plugin
	IsPlaintext bool
	IsInsecure  bool
	IsDryRun    bool
	client      api.ActionAPI_SubscribeClient
}

// Run connects to the server and start communication.
// It is blocking and should be run in a goroutine
func (c *Connector) Run(ctx context.Context, resetBackOffFn func()) error {
	if len(c.Plugins) == 0 {
		return backoff.Permanent(errors.New("no plugins are initialized, no sense to run actions"))
	}

	lnAPI, err := c.LnAPI()
	if err != nil {
		return err
	}
	if lnAPI == nil {
		return errors.New("lightning API not obtained")
	}
	defer lnAPI.Cleanup()

	pubkey, err := c.getPubKey(ctx, lnAPI)
	if err != nil {
		return fmt.Errorf("could not get pubkey %w", err)
	}

	clientType := fmt.Sprint(lnAPI.GetAPIType())

	client, connection, err := NewAPIClient(ctx, c.Address, pubkey, c.APIKey, c.IsPlaintext, c.IsInsecure)
	if err != nil {
		return err
	}
	defer connection.Close()

	ctx = metadata.AppendToOutgoingContext(ctx, "pubkey", pubkey, "clientType", clientType, "key", c.APIKey)

	stream, err := client.Subscribe(ctx)
	if err != nil {
		return err
	}
	c.client = stream

	return c.communicate(ctx, stream, resetBackOffFn)
}

type actionStreamer interface {
	Send(*api.AgentReply) error
	Recv() (*api.Action, error)
}

// Receive messages from the server and execute the requested actions
func (c *Connector) communicate(ctx context.Context, stream actionStreamer, resetBackOffFn func()) error {
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
				glog.Errorf("Error while receiving message: %v", err)
				return err
			case msg.Sequence == api.Sequence_CONNECT:
				// first request is validation of the connection and credentials
				glog.Info("Successfully connected to the server")
				break
			case msg.Sequence == api.Sequence_EXECUTE:
				plugin, ok := c.Plugins[msg.Action]
				if !ok {
					err = stream.Send(&api.AgentReply{
						JobId:    msg.JobId,
						Sequence: api.Sequence_EXECUTE,
						Type:     api.ReplyType_ERROR,
						Message:  fmt.Sprintf("Plugin %s not found on agent", msg.Action),
					})
				} else if err = plugin.Execute(int64(msg.JobId), msg.Data, c.ForwardJobMessages); err != nil {
					glog.Errorf("Could not execute action: %v", err)
					err = stream.Send(&api.AgentReply{
						JobId:    msg.JobId,
						Sequence: api.Sequence_EXECUTE,
						Type:     api.ReplyType_ERROR,
						Message:  err.Error(),
						Data:     msg.Data,
					})
				} else { // ack
					err = stream.Send(&api.AgentReply{
						JobId:    msg.JobId,
						Sequence: api.Sequence_EXECUTE,
						Type:     api.ReplyType_SUCCESS,
					})
				}
				if err != nil {
					glog.Errorf("Error while sending message: %v", err)
				}
				break
			case msg.Sequence == api.Sequence_PING:
				glog.V(4).Info("Received keep-alive ping from server")
				err = stream.Send(&api.AgentReply{Sequence: api.Sequence_PING})
				if err != nil {
					glog.Errorf("Error while sending ping message: %v", err)
				}
				break
			default:
				glog.Errorf("Ignoring received message: %v", msg)
			}
		}
		resetBackOffFn()
	}
}

// ForwardJobMessages is a callback which is called by the plugins to forward messages to the server
func (c *Connector) ForwardJobMessages(msg entities.PluginMessage) error {
	replyType := api.ReplyType_SUCCESS
	if msg.IsError {
		replyType = api.ReplyType_ERROR
	}

	sequence := api.Sequence_LOG
	if msg.IsFinished {
		sequence = api.Sequence_FINISHED
	}

	if err := c.client.SendMsg(&api.AgentReply{
		JobId:    msg.JobID,
		Sequence: sequence,
		Type:     replyType,
		Message:  msg.Message,
		Data:     msg.Data,
	}); err != nil {
		glog.Errorf("Error while sending message: %v", err)
		return err
	}

	return nil
}

func (c *Connector) getPubKey(ctx context.Context, lnAPI lightning.LightingAPICalls) (string, error) {
	info, err := lnAPI.GetInfo(ctx)
	if err != nil {
		return "", err
	}

	return info.IdentityPubkey, nil
}
