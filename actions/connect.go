package actions

import (
	"context"
	"errors"
	"fmt"
	"io"

	api "github.com/bolt-observer/agent/actions/bolt-observer-api"
	"github.com/bolt-observer/agent/lightning"
	"github.com/golang/glog"
	"google.golang.org/grpc/metadata"

	"github.com/bolt-observer/agent/entities"
)

// Connector handles gRPC connection and communication with the server and route message to/from the plugins
type Connector struct {
	Address    string
	APIKey     string
	LnAPI      entities.NewAPICall
	Plugins    map[string]entities.Plugin
	IsInsecure bool
	IsDryRun   bool
	client     api.ActionAPI_SubscribeClient
}

// Run connects to the server and start communication.
// It is blocking and should be run in a goroutine
func (c *Connector) Run(ctx context.Context) error {
	lnAPI, err := c.LnAPI()
	if err != nil {
		return err
	}
	if lnAPI == nil {
		return errors.New("lightning API not obtained")
	}

	pubkey, err := c.getPubKey(ctx, lnAPI)
	if err != nil {
		return fmt.Errorf("Could not get pubkey %w", err)
	}

	clientType := fmt.Sprint(lnAPI.GetAPIType())

	client, connection, err := NewAPIClient(ctx, c.Address, pubkey, c.APIKey, c.IsInsecure)
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

	return c.communicate(ctx, stream)
}

type actionStreamer interface {
	Send(*api.AgentReply) error
	Recv() (*api.Action, error)
}

// Receive messages from the server and execute the requested actions
func (c *Connector) communicate(ctx context.Context, stream actionStreamer) error {
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
			case msg.Sequence == api.Sequenece_CONNECT:
				// first request is validation of the connection and credentials
				glog.Info("Successfully connected to the server")
				break
			case msg.Sequence == api.Sequenece_EXECUTE:
				plugin, ok := c.Plugins[msg.Action]
				if !ok {
					stream.Send(&api.AgentReply{
						JobId:    msg.JobId,
						Sequence: api.Sequenece_EXECUTE,
						Type:     api.ReplyType_ERROR,
						Message:  fmt.Sprintf("Plugin %s not found on agent", msg.Action),
					})
					break
				}

				if err = plugin.Execute(msg.JobId, msg.Data, c.ForwardJobMessages, c.IsDryRun); err != nil {
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
			default:
				glog.Errorf("Ignoring received message: %v", msg)
			}
		}
	}
}

// ForwardJobMessages is a callback which is called by the plugins to forward messages to the server
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
