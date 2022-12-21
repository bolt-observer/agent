package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/golang/glog"
	cli "github.com/urfave/cli"

	"github.com/bolt-observer/agent/nodedata"
	api "github.com/bolt-observer/agent/lightning_api"
	entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"

	agent_entities "github.com/bolt-observer/agent/entities"
)

const (
	defaultDataDir          = "data"
	defaultChainSubDir      = "chain"
	defaultTLSCertFilename  = "tls.cert"
	defaultMacaroonFilename = "readonly.macaroon"
)

var (
	defaultLightningDir = btcutil.AppDataDir("lightning", false)
	defaultLndDir       = btcutil.AppDataDir("lnd", false)
	defaultTLSCertPath  = filepath.Join(defaultLndDir, defaultTLSCertFilename)
	defaultRPCPort      = utils.GetEnvWithDefault("DEFAULT_GRPC_PORT", "10009")
	defaultRPCHostPort  = "localhost:" + defaultRPCPort
	apiKey              string
	url                 string
	GitRevision         = "unknownVersion"

	nodeInfoReported sync.Map
	private          bool
	timeout          = 15 * time.Second
	preferipv4       = false
)

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

func getData(ctx *cli.Context) (*entities.Data, error) {
	resp := &entities.Data{}
	resp.Endpoint = ctx.String("rpcserver")
	resp.ApiType = nil

	// Assume CLN first (shouldn't really matter unless you have CLN and LND on the same machine, then you can select LND through "ignorecln")
	if !ctx.Bool("ignorecln") {
		path := findUnixSocket(filepath.Join(defaultLightningDir, ctx.String("chain"), "lightning-rpc"), resp.Endpoint)
		if path != "" {
			resp.Endpoint = path
			v := int(api.CLN_SOCKET)
			resp.ApiType = &v
		}
	}

	if resp.ApiType == nil {
		if ctx.Bool("userest") {
			v := int(api.LND_REST)
			resp.ApiType = &v
		} else {
			v := int(api.LND_GRPC)
			resp.ApiType = &v
		}
	}

	if resp.ApiType != nil && *resp.ApiType == int(api.CLN_SOCKET) {
		// CLN socket connections do not need anything else
		return resp, nil
	}

	tlsCertPath, macPath, err := extractPathArgs(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not extractPathArgs %v", err)
	}

	content, err := ioutil.ReadFile(tlsCertPath)
	if err != nil {
		return nil, fmt.Errorf("could not read certificate file %s", tlsCertPath)
	}

	resp.CertificateBase64 = base64.StdEncoding.EncodeToString(content)

	macBytes, err := ioutil.ReadFile(macPath)
	if err != nil {
		return nil, fmt.Errorf("could not read macaroon file %s", macPath)
	}

	resp.MacaroonHex = hex.EncodeToString(macBytes)

	return resp, nil
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
// This function is taken from https://github.com/btcsuite/btcd
func cleanAndExpandPath(path string) string {
	if path == "" {
		return ""
	}

	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		var homeDir string
		user, err := user.Current()
		if err == nil {
			homeDir = user.HomeDir
		} else {
			homeDir = os.Getenv("HOME")
		}

		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but the variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
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
	lndDir := cleanAndExpandPath(ctx.String("lnddir"))

	// If the macaroon path as been manually provided, then we'll only
	// target the specified file.
	var macPath string
	if ctx.String("macaroonpath") != "" {
		macPath = cleanAndExpandPath(ctx.String("macaroonpath"))
	} else {
		// Otherwise, we'll go into the path:
		// lnddir/data/chain/<chain>/<network> in order to fetch the
		// macaroon that we need.
		macPath = filepath.Join(
			lndDir, defaultDataDir, defaultChainSubDir, chain,
			network, defaultMacaroonFilename,
		)
	}

	tlsCertPath := cleanAndExpandPath(ctx.String("tlscertpath"))

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

func getApp() *cli.App {
	app := cli.NewApp()
	app.Version = GitRevision

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "apikey",
			Value: "",
			Usage: "api key",
		},
		&cli.StringFlag{
			Name:  "rpcserver",
			Value: defaultRPCHostPort,
			Usage: "host:port of ln daemon",
		},
		&cli.StringFlag{
			Name:  "lnddir",
			Value: defaultLndDir,
			Usage: "path to lnd's base directory",
		},
		&cli.StringFlag{
			Name:  "tlscertpath",
			Value: defaultTLSCertPath,
			Usage: "path to TLS certificate",
		},
		&cli.StringFlag{
			Name:  "chain, c",
			Usage: "the chain lnd is running on e.g. bitcoin",
			Value: "bitcoin",
		},
		&cli.StringFlag{
			Name: "network, n",
			Usage: "the network lnd is running on e.g. mainnet, " +
				"testnet, etc.",
			Value: "mainnet",
		},
		&cli.StringFlag{
			Name:  "macaroonpath",
			Usage: "path to macaroon file",
		},
		&cli.IntFlag{
			Name:  "allowedentropy",
			Usage: "allowed entropy in bits for channel balances",
			Value: 64,
		},
		&cli.StringFlag{
			Name:  "interval",
			Usage: "interval to poll - 10s, 1m, 10m or 1h",
			Value: "10s",
		},
		&cli.BoolFlag{
			Name:  "private",
			Usage: "report private data as well (default: false)",
		},
		&cli.BoolFlag{
			Name:  "preferipv4",
			Usage: "If you have the choice between IPv6 and IPv4 prefer IPv4 (default: false)",
		},
		&cli.BoolFlag{
			Name:   "userest",
			Usage:  "Use REST API when true instead of gRPC",
			Hidden: true,
		},
		&cli.DurationFlag{
			Name:   "keepalive",
			Usage:  "Keepalive interval (if nothing changed after this time an empty message will be sent)",
			Value:  60 * time.Second,
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:   "checkgraph",
			Usage:  "check gossip data as well",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "url",
			Usage:  "Report URL",
			Value:  "https://bolt.observer/api/agent-report/",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "nodeurl",
			Usage:  "Node report URL",
			Value:  "https://bolt.observer/api/private-node/",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "nodeinterval",
			Usage:  "interval to poll - 10s, 1m, 10m or 1h",
			Value:  "1m",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "uniqueid",
			Usage:  "Unique identifier",
			Value:  "",
			Hidden: true,
		},
		// TODO: userest and ignorecln should be merged into some sort of API type
		&cli.BoolFlag{
			Name:   "ignorecln",
			Usage:  "Ignore CLN socket",
			Hidden: true,
		},
	}

	app.Flags = append(app.Flags, glogFlags...)

	return app
}

func getInterval(ctx *cli.Context, name string) (agent_entities.Interval, error) {
	i := agent_entities.Interval(0)
	s := strings.ToLower(ctx.String(name))

	err := i.UnmarshalJSON([]byte(s))
	if err != nil {
		return agent_entities.TEN_SECONDS, fmt.Errorf("unknown interval specified %s", s)
	}

	return i, nil
}

func redirectPost(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}

	lastReq := via[len(via)-1]
	if (req.Response.StatusCode == 301 || req.Response.StatusCode == 302) && lastReq.Method == http.MethodPost {
		req.Method = http.MethodPost

		// Get the body of the original request, set here, since req.Body will be nil if a 302 was returned
		if via[0].GetBody != nil {
			var err error
			req.Body, err = via[0].GetBody()
			if err != nil {
				return err
			}
			req.ContentLength = via[0].ContentLength
		}
	}

	return nil
}

func getHttpClient() *http.Client {
	var zeroDialer net.Dialer

	httpClient := &http.Client{
		Timeout: timeout,
	}

	if preferipv4 {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return zeroDialer.DialContext(ctx, "tcp4", addr)
		}
		httpClient.Transport = transport
	}
	httpClient.CheckRedirect = redirectPost

	return httpClient
}

func shouldCrash(status int, body string) {
	if status != http.StatusUnauthorized {
		return
	}

	if strings.Contains(body, "Invalid token") {
		glog.Warningf("API key is invalid")
		os.Exit(1)
	}
}

func nodeDataCallback(ctx context.Context, report *agent_entities.NodeDataReport) bool {
	n := agent_entities.NodeIdentifier{Identifier: report.ChannelReport.PubKey, UniqueId: report.ChannelReport.UniqueId}
	_, ok := nodeInfoReported.Load(n.GetId())
	if !ok {
		glog.V(3).Infof("Node data for %s was not reported yet", n.GetId())
		return false
	}

	rep, err := json.Marshal(report)
	if err != nil {
		glog.Warningf("Error marshalling report: %v", err)
		return false
	}

	if url == "" {
		if glog.V(2) {
			glog.V(2).Infof("Sent out callback %s", string(rep))
		} else {
			glog.V(1).Infof("Sent out callback")
		}
		return true
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(rep)))
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", fmt.Sprintf("boltobserver-agent/%s", GitRevision))
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Content-Type", "application/json")

	client := getHttpClient()

	resp, err := client.Do(req)
	if err != nil {
		glog.Warningf("Error when doing request, %v", err)
		if glog.V(2) {
			glog.V(2).Infof("Failed to send out callback %s", string(rep))
		}
		return false
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		glog.Warningf("Status was not OK but, %d", resp.StatusCode)
		defer resp.Body.Close()
		bodyData, _ := ioutil.ReadAll(resp.Body)

		glog.V(2).Infof("Failed to send out callback %s, server said %s", string(rep), string(bodyData))
		shouldCrash(resp.StatusCode, string(bodyData))

		return false
	}

	if glog.V(2) {
		glog.V(2).Infof("Sent out callback %s", string(rep))
	} else {
		glog.V(1).Infof("Sent out callback")
	}

	return true
}

func mkGetLndApi(ctx *cli.Context) agent_entities.NewApiCall {
	return func() api.LightingApiCalls {
		return api.NewApi(api.LND_GRPC, func() (*entities.Data, error) {
			return getData(ctx)
		})
	}
}

func checker(ctx *cli.Context) error {
	apiKey = utils.GetEnvWithDefault("API_KEY", "")
	if apiKey == "" {
		apiKey = ctx.String("apikey")
	}

	if apiKey == "" && (ctx.String("url") != "" || ctx.String("nodeurl") != "") {
		return fmt.Errorf("missing API key")
	}

	url = ctx.String("url")
	private = ctx.Bool("private")

	interval, err := getInterval(ctx, "interval")
	if err != nil {
		return err
	}

	nodeinterval, err := getInterval(ctx, "nodeinterval")
	if err != nil {
		nodeinterval = interval
	}

	preferipv4 = ctx.Bool("preferipv4")

	ct := context.Background()
	nodeData := nodedata.NewDefaultNodeData(ct, ctx.Duration("keepalive"), ctx.Bool("smooth"), ctx.Bool("checkgraph"), nodedata.NewNopNodeDataMonitoring("nodedatachecker"))

	if interval == agent_entities.SECOND {
		// Second is just for testing purposes
		interval = agent_entities.TEN_SECONDS
	}

	if nodeinterval == agent_entities.SECOND {
		// Second is just for testing purposes
		nodeinterval = agent_entities.TEN_SECONDS
	}

	settings := agent_entities.ReportingSettings{PollInterval: interval, AllowedEntropy: ctx.Int("allowedentropy"), AllowPrivateChannels: ctx.Bool("private")}

	if settings.PollInterval == agent_entities.MANUAL_REQUEST {
		nodeData.GetState("", ctx.String("uniqueid"), mkGetLndApi(ctx), settings, nodeDataCallback)
	} else {
		err = nodeData.Subscribe(
			nodeDataCallback,
			mkGetLndApi(ctx),
			nodeinterval,
			"",
			settings,
			ctx.String("uniqueid"),
		)

		if err != nil {
			return err
		}

		glog.Info("Waiting for events...")
		utils.WaitAll(nodeData)
	}

	return nil
}

func main() {
	app := getApp()
	app.Name = "balance-agent"
	app.Usage = "Utility to monitor channel balances"
	app.Action = func(c *cli.Context) error {
		glogShim(c)
		if err := checker(c); err != nil {
			return err
		}

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		glog.Error(err)
	}
}
