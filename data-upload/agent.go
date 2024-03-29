package raw

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"

	api "github.com/bolt-observer/agent/agent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Credentials implements PerRPCCredentials
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

// MakeCredentials creates new Credentials
func MakeCredentials(pubkey, authToken string) *Credentials {
	return &Credentials{
		Pubkey:    pubkey,
		AuthToken: authToken,
	}
}

func getConnection(endpoint, pubkey, authToken string, isInsecure bool) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithPerRPCCredentials(MakeCredentials(pubkey, authToken)),
		grpc.WithContextDialer(func(ctx context.Context,
			endpoint string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(
				ctx, "tcp", endpoint,
			)
		}),
	}

	if isInsecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		cp, _ := x509.SystemCertPool()
		minVersion := uint16(tls.VersionTLS11)
		conf := &tls.Config{RootCAs: cp, MinVersion: minVersion}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(conf)))
	}

	conn, err := grpc.Dial(endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to dial %s", err.Error())
	}

	return conn, nil

}

func getAgentAPI(endpoint, pubKey, authToken string, isInsecure bool) (api.AgentAPIClient, error) {
	itf, err := getConnection(endpoint, pubKey, authToken, isInsecure)
	if err != nil {
		return nil, err
	}

	return api.NewAgentAPIClient(itf), err
}
