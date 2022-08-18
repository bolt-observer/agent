package lightning_api

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	utils "github.com/bolt-observer/go_common/utils"
	"github.com/golang/glog"
	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

func GetConnection(getData GetDataCall) (*grpc.ClientConn, error) {
	var (
		creds    credentials.TransportCredentials
		macBytes []byte
	)

	if getData == nil {
		return nil, fmt.Errorf("getData is nil")
	}

	data, err := getData()
	if err != nil {
		return nil, fmt.Errorf("data could not be fetched %v", err)
	}

	certBytes, err := base64.StdEncoding.DecodeString(data.CertificateBase64)
	if err != nil {
		return nil, fmt.Errorf("base64 decoding failed %v", err)
	}

	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(certBytes) {
		return nil, fmt.Errorf("append cert failed %v", err)
	}
	creds = credentials.NewClientTLSFromCert(cp, "")

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}

	macBytes, err = hex.DecodeString(data.MacaroonHex)
	if err != nil {
		return nil, fmt.Errorf("unable to decode macaroon %v", err)
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macBytes); err != nil {
		return nil, fmt.Errorf("unable to unnmarshal macaroon %v", err)
	}

	cred, _ := macaroons.NewMacaroonCredential(mac)
	opts = append(opts, grpc.WithPerRPCCredentials(cred))

	genericDialer := lncfg.ClientAddressDialer(utils.GetEnvWithDefault("DEFAULT_GRPC_PORT", "10009"))
	opts = append(opts, grpc.WithContextDialer(genericDialer))

	maxMsg, err := strconv.Atoi(utils.GetEnvWithDefault("MAX_MSG_SIZE", "512"))
	if err != nil {
		return nil, fmt.Errorf("unable to decode maxMsg %s", err.Error())
	}
	maxSize := grpc.MaxCallRecvMsgSize(1024 * 1024 * maxMsg)

	opts = append(opts, grpc.WithDefaultCallOptions(maxSize))

	conn, err := grpc.Dial(data.Endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to dial %s", err.Error())
	}

	return conn, nil
}

func GetClient(getData GetDataCall) (lnrpc.LightningClient, func(), error) {
	conn, err := GetConnection(getData)
	if err != nil {
		return nil, nil, err
	}
	cleanUp := func() {
		conn.Close()
	}

	return lnrpc.NewLightningClient(conn), cleanUp, nil
}

func IsMacaroonValid(mac *macaroon.Macaroon) (bool, time.Duration) {
	minTime := time.Time{}

	for _, v := range mac.Caveats() {
		split := strings.Split(string(v.Id), " ")
		if len(split) != 2 {
			continue
		}
		if split[0] != "time-before" {
			continue
		}

		time, err := time.Parse("2006-01-02T15:04:05.999999999Z", split[1])
		if err != nil {
			glog.Warningf("Could not parse time: %v", err)
			continue
		}

		if minTime.IsZero() || time.Before(minTime) {
			minTime = time
		}
	}

	if minTime.IsZero() {
		return true, time.Duration(1<<63 - 1)
	}

	now := time.Now().UTC().Add(5 * time.Second)
	dur := minTime.Sub(now)
	return int64(dur) > 0, dur
}
