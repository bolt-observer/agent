package lightning

import (
	"bytes"
	"context"
	"encoding/json"
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

// MakeUnixConnection create a new CLN connection
func MakeUnixConnection(socketType string, address string) *ClnUnixConnection {
	ret := &ClnUnixConnection{}

	client, err := rpc.Dial(socketType, address)
	if err != nil {
		glog.Warningf("Got error: %v", err)
		return nil
	}

	ret.Client = client

	return ret
}

// Call helper to call rpc method
func (l *ClnUnixConnection) Call(ctx context.Context, serviceMethod string, args any, reply any) error {
	c := make(chan *r.Call, 1)

	go func() { l.Client.Go(serviceMethod, args, reply, c) }()
	select {
	case call := <-c:
		return call.Error
	case <-ctx.Done():
		return os.ErrDeadlineExceeded
	}
}

// StreamResponse streams the response
func (l *ClnUnixConnection) StreamResponse(ctx context.Context, serviceMethod string, args any) (io.Reader, error) {
	c := make(chan *r.Call, 1)
	var reply json.RawMessage
	go func() { l.Client.Go(serviceMethod, args, &reply, c) }()

	select {
	case call := <-c:
		if call.Error != nil {
			return nil, call.Error
		}

		// TODO: currently we are cheating a bit here by serializing and unserializing
		b, err := json.Marshal(reply)
		if err != nil {
			return nil, err
		}

		return bytes.NewReader(b), nil

	case <-ctx.Done():
		return nil, os.ErrDeadlineExceeded
	}
}
