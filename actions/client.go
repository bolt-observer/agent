package actions

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"

	api "github.com/bolt-observer/agent/actions/bolt-observer-api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	insecureGRPC "google.golang.org/grpc/credentials/insecure"
)

// NewAPIClient creates a new gRPC ActionAPIClient
func NewAPIClient(ctx context.Context, endpoint, pubKey, authToken string, isInsecure bool) (api.ActionAPIClient, *grpc.ClientConn, error) {
	opts := []grpc.DialOption{}
	if isInsecure {
		opts = append(opts, grpc.WithTransportCredentials(insecureGRPC.NewCredentials()))
	} else {
		cp, _ := x509.SystemCertPool()
		minVersion := uint16(tls.VersionTLS11)
		conf := &tls.Config{RootCAs: cp, MinVersion: minVersion}
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

	conn, err := grpc.DialContext(ctx, endpoint, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to dial %s", err.Error())
	}

	return api.NewActionAPIClient(conn), conn, nil
}
