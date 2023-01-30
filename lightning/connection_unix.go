package lightning

import (
	r "net/rpc"
	"os"
	"time"

	"github.com/golang/glog"
	rpc "github.com/powerman/rpc-codec/jsonrpc2"
)

// ClnUnixConnection represents a UNIX domain socket
type ClnUnixConnection struct {
	Client *rpc.Client

	ClnConnection
}

// MakeUnixConnection create a new CLN connection
// "unix" corresponds to SOCK_STREAM
// "unixgram" corresponds to SOCK_DGRAM
// "unixpacket" corresponds to SOCK_SEQPACKET
func MakeUnixConnection(socketType string, address string, timeout time.Duration) *ClnUnixConnection {
	ret := &ClnUnixConnection{}

	client, err := rpc.Dial(socketType, address)
	if err != nil {
		glog.Warningf("Got error: %v", err)
		return nil
	}

	ret.Client = client
	ret.Timeout = timeout

	return ret
}

// CallWithTimeout - helper to call rpc method with a timeout
func (l *ClnUnixConnection) CallWithTimeout(serviceMethod string, args any, reply any) error {
	c := make(chan *r.Call, 1)
	go func() { l.Client.Go(serviceMethod, args, reply, c) }()
	select {
	case call := <-c:
		return call.Error
	case <-time.After(l.Timeout):
		return os.ErrDeadlineExceeded
	}
}
