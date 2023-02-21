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
)

func NewAPIClient(endpoint, pubKey, authToken string) (api.ActionAPIClient, *grpc.ClientConn, error) {
	var conf *tls.Config

	cp, _ := x509.SystemCertPool()
	minVersion := uint16(tls.VersionTLS11)
	conf = &tls.Config{RootCAs: cp, MinVersion: minVersion}

	genericDialer := func(ctx context.Context,
		endpoint string) (net.Conn, error) {
		d := net.Dialer{}
		return d.DialContext(
			ctx, "tcp", endpoint,
		)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(conf)),
		grpc.WithPerRPCCredentials(&Credentials{
			Pubkey:    pubKey,
			AuthToken: authToken,
		}),
		grpc.WithContextDialer(genericDialer),
	}

	conn, err := grpc.Dial(endpoint, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to dial %s", err.Error())
	}

	return api.NewActionAPIClient(conn), conn, nil
}

// Credentials
type Credentials struct {
	Pubkey    string
	AuthToken string
}

// GetRequestMetadata implements PerRPCCredentials
func (c *Credentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	resp := make(map[string]string, 0)
	resp["pubkey"] = c.Pubkey
	resp["authtoken"] = c.AuthToken

	return resp, nil
}

// RequireTransportSecurity implements PerRPCCredentials
func (c *Credentials) RequireTransportSecurity() bool {
	return true
}
