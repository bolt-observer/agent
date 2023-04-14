package actions

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"

	api "github.com/bolt-observer/agent/actions/bolt-observer-api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	insecureGRPC "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// NewAPIClient creates a new gRPC ActionAPIClient
func NewAPIClient(ctx context.Context, endpoint, pubKey, authToken string, isPlaintext bool, isInsecure bool) (api.ActionAPIClient, *grpc.ClientConn, error) {
	opts := []grpc.DialOption{}
	if isPlaintext {
		opts = append(opts, grpc.WithTransportCredentials(insecureGRPC.NewCredentials()))
	} else {
		cp, _ := x509.SystemCertPool()
		minVersion := uint16(tls.VersionTLS11)
		conf := &tls.Config{RootCAs: cp, MinVersion: minVersion, InsecureSkipVerify: isInsecure}
		opts = append(opts, []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(conf)),
			grpc.WithContextDialer(func(ctx context.Context,
				endpoint string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(
					ctx, "tcp", endpoint,
				)
			}),
		}...)
	}

	kasp := keepalive.ClientParameters{
		Time:    30 * time.Second,
		Timeout: 1 * time.Hour,
	}

	opts = append(opts, grpc.WithKeepaliveParams(kasp))

	conn, err := grpc.DialContext(ctx, endpoint, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to dial %s", err.Error())
	}

	return api.NewActionAPIClient(conn), conn, nil
}
