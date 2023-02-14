package lightning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	r "net/rpc"
	"os"

	"github.com/golang/glog"
	rpc "github.com/powerman/rpc-codec/jsonrpc2"
)

// ClnUnixConnection represents a UNIX domain socket
type ClnUnixConnection struct {
	Client *rpc.Client

	ClnConnection
}

// Compile time check for the interface
var _ ClnConnectionAPI = &ClnUnixConnection{}

// NewUnixConnection create a new CLN connection
func NewUnixConnection(socketType string, address string) *ClnUnixConnection {
	ret := &ClnUnixConnection{}

	client, err := rpc.Dial(socketType, address)
	if err != nil {
		glog.Warningf("Got error: %v", err)
		return nil
	}
	if client == nil {
		glog.Warningf("Got nil client")
		return nil
	}

	ret.Client = client

	return ret
}

// Call calls serviceMethod with args and fills reply with response
func (l *ClnUnixConnection) Call(ctx context.Context, serviceMethod string, args any, reply any) error {
	if l.Client == nil {
		return fmt.Errorf("no client")
	}

	c := make(chan *r.Call, 1)

	go l.Client.Go(serviceMethod, args, reply, c)
	select {
	case call := <-c:
		return call.Error
	case <-ctx.Done():
		return os.ErrDeadlineExceeded
	}
}

// StreamResponse is meant for streaming responses it calls serviceMethod with args and returns an io.Reader
func (l *ClnUnixConnection) StreamResponse(ctx context.Context, serviceMethod string, args any) (io.Reader, error) {
	if l.Client == nil {
		return nil, fmt.Errorf("no client")
	}

	c := make(chan *r.Call, 1)
	var reply json.RawMessage
	go l.Client.Go(serviceMethod, args, &reply, c)

	select {
	case call := <-c:
		if call.Error != nil {
			return nil, call.Error
		}

		// TODO: currently we are cheating a bit here by serializing and unserializing
		// since rpc.Client does not allow us to directly get response as io.Reader
		b, err := json.Marshal(reply)
		if err != nil {
			return nil, err
		}

		return bytes.NewReader(b), nil

	case <-ctx.Done():
		return nil, os.ErrDeadlineExceeded
	}
}

// Cleanup does the cleanup
func (l *ClnUnixConnection) Cleanup() {
	// Noop
	return
}
