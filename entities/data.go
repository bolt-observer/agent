package entities

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	api "github.com/bolt-observer/agent/lightning"
	entities "github.com/bolt-observer/go_common/entities"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/urfave/cli"
)

const (
	defaultDataDir               = "data"
	defaultChainSubDir           = "chain"
	defaultTLSCertFilename       = "tls.cert"
	defaultReadMacaroonFilename  = "readonly.macaroon"
	defaultAdminMacaroonFilename = "admin.macaroon"
)

var (
	defaultLightningDir = btcutil.AppDataDir("lightning", false)
)

func MkGetLndAPI(cmdCtx *cli.Context) api.NewAPICall {
	return func() (api.LightingAPICalls, error) {
		return api.NewAPI(api.LndGrpc, func() (*entities.Data, error) {
			return getData(cmdCtx)
		})
	}
}

func getData(cmdCtx *cli.Context) (*entities.Data, error) {
	resp := &entities.Data{}
	resp.Endpoint = cmdCtx.String("rpcserver")
	resp.ApiType = nil

	// Assume CLN first (shouldn't really matter unless you have CLN and LND on the same machine, then you can select LND through "ignorecln")
	if !cmdCtx.Bool("ignorecln") {
		network := strings.ToLower(cmdCtx.String("network"))
		if network == "" {
			network = "bitcoin"
		} else if network == "mainnet" {
			network = "bitcoin"
		}

		path := findUnixSocket(filepath.Join(defaultLightningDir, network, "lightning-rpc"), resp.Endpoint)
		if path != "" {
			resp.Endpoint = path
			resp.ApiType = intPtr(int(api.ClnSocket))
		}
	}

	if resp.ApiType == nil {
		if cmdCtx.Bool("userest") {
			resp.ApiType = intPtr(int(api.LndRest))
		} else {
			resp.ApiType = intPtr(int(api.LndGrpc))
		}
	}

	if resp.ApiType != nil && *resp.ApiType == int(api.ClnSocket) {
		// CLN socket connections do not need anything else
		return resp, nil
	}

	tlsCertPath, macPath, err := extractPathArgs(cmdCtx)
	if err != nil {
		return nil, fmt.Errorf("could not extractPathArgs %v", err)
	}

	content, err := os.ReadFile(tlsCertPath)
	if err != nil {
		return nil, fmt.Errorf("could not read certificate file %s", tlsCertPath)
	}

	resp.CertificateBase64 = base64.StdEncoding.EncodeToString(content)

	macBytes, err := os.ReadFile(macPath)
	if err != nil {
		return nil, fmt.Errorf("could not read macaroon file %s", macPath)
	}

	resp.MacaroonHex = hex.EncodeToString(macBytes)

	return resp, nil
}

func extractPathArgs(ctx *cli.Context) (string, string, error) {
	// We'll start off by parsing the active chain and network. These are
	// needed to determine the correct path to the macaroon when not
	// specified.
	chain := strings.ToLower(ctx.String("chain"))
	switch chain {
	case "bitcoin", "litecoin":
	default:
		return "", "", fmt.Errorf("unknown chain: %v", chain)
	}

	network := strings.ToLower(ctx.String("network"))
	switch network {
	case "mainnet", "testnet", "regtest", "simnet":
	default:
		return "", "", fmt.Errorf("unknown network: %v", network)
	}

	// We'll now fetch the lnddir so we can make a decision  on how to
	// properly read the macaroons (if needed) and also the cert. This will
	// either be the default, or will have been overwritten by the end
	// user.
	lndDir := CleanAndExpandPath(ctx.String("lnddir"))

	// If the macaroon path as been manually provided, then we'll only
	// target the specified file.
	var macPath string
	if ctx.String("macaroonpath") != "" {
		macPath = CleanAndExpandPath(ctx.String("macaroonpath"))
	} else {
		// Otherwise, we'll go into the path:
		// lnddir/data/chain/<chain>/<network> in order to fetch the
		// macaroon that we need.

		name := defaultReadMacaroonFilename
		if ctx.Bool("actions") {
			name = defaultAdminMacaroonFilename
		}

		macPath = filepath.Join(
			lndDir, defaultDataDir, defaultChainSubDir, chain,
			network, name,
		)
	}

	tlsCertPath := CleanAndExpandPath(ctx.String("tlscertpath"))

	// If a custom lnd directory was set, we'll also check if custom paths
	// for the TLS cert and macaroon file were set as well. If not, we'll
	// override their paths so they can be found within the custom lnd
	// directory set. This allows us to set a custom lnd directory, along
	// with custom paths to the TLS cert and macaroon file.
	if _, err := os.Stat(tlsCertPath); errors.Is(err, os.ErrNotExist) {
		tlsCertPath = filepath.Join(lndDir, defaultTLSCertFilename)
	}

	return tlsCertPath, macPath, nil
}

func findUnixSocket(paths ...string) string {
	for _, path := range paths {
		fs, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) || fs.Mode()&os.ModeSocket != os.ModeSocket {
			continue
		} else {
			return path
		}
	}

	return ""
}

func intPtr(i int) *int {
	return &i
}
