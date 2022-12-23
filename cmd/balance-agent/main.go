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
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/golang/glog"
	cli "github.com/urfave/cli"

	channelchecker "github.com/bolt-observer/agent/channelchecker"
	"github.com/bolt-observer/agent/checkermonitoring"
	"github.com/bolt-observer/agent/filter"
	api "github.com/bolt-observer/agent/lightningapi"
	"github.com/bolt-observer/agent/nodeinfo"
	entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"

	agent_entities "github.com/bolt-observer/agent/entities"
)

const (
	defaultDataDir          = "data"
	defaultChainSubDir      = "chain"
	defaultTLSCertFilename  = "tls.cert"
	defaultMacaroonFilename = "readonly.macaroon"
	whitelist               = "channel-whitelist"
)

var (
	defaultLightningDir = btcutil.AppDataDir("lightning", false)
	defaultLndDir       = btcutil.AppDataDir("lnd", false)
	defaultTLSCertPath  = filepath.Join(defaultLndDir, defaultTLSCertFilename)
	defaultRPCPort      = utils.GetEnvWithDefault("DEFAULT_GRPC_PORT", "10009")
	defaultRPCHostPort  = "localhost:" + defaultRPCPort
	apiKey              string
	url                 string
	nodeurl             string
	// GitRevision is set with build
	GitRevision = "unknownVersion"

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
		&cli.IntFlag{
			Name:  "allowedentropy",
			Usage: "allowed entropy in bits for channel balances",
			Value: 64,
		},
		&cli.StringFlag{
			Name:  "apikey",
			Value: "",
			Usage: "api key",
		},
		&cli.StringFlag{
			Name:  "interval",
			Usage: "interval to poll - 10s, 1m, 10m or 1h",
			Value: "10s",
		},
		&cli.StringFlag{
			Name:  "lnddir",
			Value: defaultLndDir,
			Usage: "path to lnd's base directory",
		},
		&cli.StringFlag{
			Name:  "macaroonpath",
			Usage: "path to macaroon file",
		},
		&cli.BoolFlag{
			Name:  "private",
			Usage: "report private data as well (default: false)",
		},
		&cli.BoolFlag{
			Name:   "public",
			Usage:  fmt.Sprintf("report public data - useful with %s (default: false)", whitelist),
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:  "preferipv4",
			Usage: "If you have the choice between IPv6 and IPv4 prefer IPv4 (default: false)",
		},
		&cli.StringFlag{
			Name:  "rpcserver",
			Value: defaultRPCHostPort,
			Usage: "host:port of ln daemon",
		},
		&cli.StringFlag{
			Name:  "tlscertpath",
			Value: defaultTLSCertPath,
			Usage: "path to TLS certificate",
		},
		&cli.StringFlag{
			Name:   whitelist,
			Usage:  "Path to file containing a whitelist of channels",
			Hidden: true,
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
			Value:  "https://ingress.bolt.observer/api/agent-report/",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "nodeurl",
			Usage:  "Node report URL",
			Value:  "https://ingress.bolt.observer/api/private-node/",
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

		&cli.StringFlag{
			Name:   "chain, c",
			Usage:  "the chain lnd is running on e.g. bitcoin",
			Value:  "bitcoin",
			Hidden: true,
		},
		&cli.StringFlag{
			Name: "network, n",
			Usage: "the network lnd is running on e.g. mainnet, " +
				"testnet, etc.",
			Value:  "mainnet",
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
		return agent_entities.TenSeconds, fmt.Errorf("unknown interval specified %s", s)
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

func getHTTPClient() *http.Client {
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

func infoCallback(ctx context.Context, report *agent_entities.InfoReport) bool {
	rep, err := json.Marshal(report)
	if err != nil {
		glog.Warningf("Error marshalling report: %v", err)
		return false
	}

	n := agent_entities.NodeIdentifier{Identifier: report.Node.PubKey, UniqueID: report.UniqueId}

	if nodeurl == "" {
		if glog.V(2) {
			glog.V(2).Infof("Sent out nodeinfo callback %s", string(rep))
		} else {
			glog.V(1).Infof("Sent out nodeinfo callback")
		}
		nodeInfoReported.Store(n.GetID(), struct{}{})
		return true
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, nodeurl, strings.NewReader(string(rep)))
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", fmt.Sprintf("boltobserver-agent/%s", GitRevision))
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Content-Type", "application/json")

	client := getHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		glog.Warningf("Error when doing request, %v", err)
		glog.V(2).Infof("Failed to send out callback %s", string(rep))
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
		glog.V(2).Infof("Sent out nodeinfo callback %s", string(rep))
	} else {
		glog.V(1).Infof("Sent out nodeinfo callback")
	}

	nodeInfoReported.Store(n.GetID(), struct{}{})

	return true
}

func balanceCallback(ctx context.Context, report *agent_entities.ChannelBalanceReport) bool {
	// When nodeinfo was not reported fake as if balance report could not be delivered (because same data
	// will be eventually retried)

	n := agent_entities.NodeIdentifier{Identifier: report.PubKey, UniqueID: report.UniqueID}
	_, ok := nodeInfoReported.Load(n.GetID())
	if !ok {
		glog.V(3).Infof("Node data for %s was not reported yet", n.GetID())
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

	client := getHTTPClient()

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

func mkGetLndAPI(ctx *cli.Context) agent_entities.NewAPICall {
	return func() api.LightingApiCalls {
		return api.NewApi(api.LND_GRPC, func() (*entities.Data, error) {
			return getData(ctx)
		})
	}
}

func reloadConfig(ctx context.Context, f *filter.FileFilter) {
	glog.Info("Reloading configuration...")

	if f != nil {
		err := f.Reload()
		if err != nil {
			glog.Warningf("Error while reloading configuration %v", err)
		}
	}
}

func signalHandler(ctx context.Context, f filter.FilteringInterface) {
	ff, ok := f.(*filter.FileFilter)

	signalChan := make(chan os.Signal, 1)
	exitChan := make(chan int)

	signal.Notify(signalChan,
		syscall.SIGHUP)
	go func() {
		for {
			select {
			case s := <-signalChan:
				switch s {
				case syscall.SIGHUP:
					if ok {
						reloadConfig(ctx, ff)
					} else {
						reloadConfig(ctx, nil)
					}

				case syscall.SIGTERM:
					exitChan <- 0
				case syscall.SIGQUIT:
					exitChan <- 0
				default:
					fmt.Println("Unknown signal.")
					exitChan <- 1
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	code := <-exitChan
	os.Exit(code)
}

func checker(ctx *cli.Context) error {
	apiKey = utils.GetEnvWithDefault("API_KEY", "")
	if apiKey == "" {
		apiKey = ctx.String("apikey")
	}

	if apiKey == "" && (ctx.String("url") != "" || ctx.String("nodeurl") != "") {
		// We don't return error here since we don't want glog to handle it
		fmt.Fprintf(os.Stderr, "missing API key (use --apikey or set API_KEY environment variable)\n")
		os.Exit(1)
	}

	ct := context.Background()

	var err error

	f, _ := filter.NewAllowAllFilter()
	if ctx.String(whitelist) != "" {
		if _, err = os.Stat(ctx.String(whitelist)); err != nil {
			// We don't return error here since we don't want glog to handle it
			fmt.Fprintf(os.Stderr, "%s points to non-existing file", whitelist)
			os.Exit(1)
		}

		o := filter.None

		if ctx.Bool("private") {
			o |= filter.AllowAllPrivate
		}

		if ctx.Bool("public") {
			o |= filter.AllowAllPublic
		}

		f, err = filter.NewFilterFromFile(ct, ctx.String(whitelist), o)
		if err != nil {
			return err
		}
	}

	go signalHandler(ct, f)

	url = ctx.String("url")
	nodeurl = ctx.String("nodeurl")
	private = ctx.Bool("private") || ctx.String(whitelist) != ""

	interval, err := getInterval(ctx, "interval")
	if err != nil {
		return err
	}

	nodeinterval, err := getInterval(ctx, "nodeinterval")
	if err != nil {
		nodeinterval = interval
	}

	preferipv4 = ctx.Bool("preferipv4")

	infochecker := nodeinfo.NewNodeInfo(ct, checkermonitoring.NewNopCheckerMonitoring("nodeinfo"))
	c := channelchecker.NewDefaultChannelChecker(ct, ctx.Duration("keepalive"), ctx.Bool("smooth"), ctx.Bool("checkgraph"), checkermonitoring.NewNopCheckerMonitoring("channelchecker"))

	if interval == agent_entities.Second {
		// Second is just for testing purposes
		interval = agent_entities.TenSeconds
	}

	if nodeinterval == agent_entities.Second {
		// Second is just for testing purposes
		nodeinterval = agent_entities.TenSeconds
	}

	settings := agent_entities.ReportingSettings{PollInterval: interval, AllowedEntropy: ctx.Int("allowedentropy"), AllowPrivateChannels: private, Filter: f}

	if settings.PollInterval == agent_entities.ManualRequest {
		infochecker.GetState("", ctx.String("uniqueid"), private, agent_entities.ManualRequest, mkGetLndAPI(ctx), infoCallback, f)
		time.Sleep(1 * time.Second)
		c.GetState("", ctx.String("uniqueid"), mkGetLndAPI(ctx), settings, balanceCallback)
	} else {
		err := infochecker.Subscribe("", ctx.String("uniqueid"), private, nodeinterval, mkGetLndAPI(ctx), infoCallback, f)
		if err != nil {
			return err
		}

		err = c.Subscribe("", ctx.String("uniqueid"),
			mkGetLndAPI(ctx),
			settings,
			balanceCallback)

		if err != nil {
			return err
		}

		glog.Info("Waiting for events...")
		utils.WaitAll(infochecker, c)
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
