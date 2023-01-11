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

func getConnection(endpoint, pubkey, authToken string) (*grpc.ClientConn, error) {

	const Insecure = true
	var conf *tls.Config

	if Insecure {
		conf = &tls.Config{InsecureSkipVerify: true, ServerName: "",
			VerifyConnection: func(cs tls.ConnectionState) error { return nil }}
	} else {
		cp, _ := x509.SystemCertPool()
		minVersion := uint16(tls.VersionTLS11)
		conf = &tls.Config{RootCAs: cp, MinVersion: minVersion}
	}

	creds := credentials.NewTLS(conf)
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}

	opts = append(opts, grpc.WithPerRPCCredentials(MakeCredentials(pubkey, authToken)))

	genericDialer := func(ctx context.Context,
		endpoint string) (net.Conn, error) {
		d := net.Dialer{}
		return d.DialContext(
			ctx, "tcp", endpoint,
		)
	}

	opts = append(opts, grpc.WithContextDialer(genericDialer))

	conn, err := grpc.Dial(endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to dial %s", err.Error())
	}

	return conn, nil

}

// GetAgentAPI retrieves the go API
func GetAgentAPI(endpoint, pubKey, authToken string) api.AgentAPIClient {
	itf, err := getConnection(endpoint, pubKey, authToken)
	if err != nil {
		return nil
	}

	return api.NewAgentAPIClient(itf)
}
