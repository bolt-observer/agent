package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/cenkalti/backoff/v4"
	"github.com/golang/glog"
	cli "github.com/urfave/cli"
	"golang.org/x/sync/errgroup"

	"github.com/bolt-observer/agent/actions"
	"github.com/bolt-observer/agent/checkermonitoring"
	raw "github.com/bolt-observer/agent/data-upload"
	"github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/bolt-observer/agent/nodedata"
	"github.com/bolt-observer/agent/plugins"
	utils "github.com/bolt-observer/go_common/utils"

	agent_entities "github.com/bolt-observer/agent/entities"

	// we need this so init() is called
	_ "github.com/bolt-observer/agent/plugins/boltz"
)

const (
	defaultDataDir               = "data"
	defaultChainSubDir           = "chain"
	defaultTLSCertFilename       = "tls.cert"
	defaultReadMacaroonFilename  = "readonly.macaroon"
	defaultAdminMacaroonFilename = "admin.macaroon"
	whitelist                    = "channel-whitelist"

	periodicSend = true
)

var (
	defaultLndDir      = btcutil.AppDataDir("lnd", false)
	defaultTLSCertPath = filepath.Join(defaultLndDir, defaultTLSCertFilename)
	defaultRPCPort     = utils.GetEnvWithDefault("DEFAULT_GRPC_PORT", "10009")
	defaultRPCHostPort = "localhost:" + defaultRPCPort
	apiKey             string
	url                string
	// GitRevision is set with build
	GitRevision = "unknownVersion"
	private     bool
	timeout     = 15 * time.Second
	preferipv4  = false
	noplugins   = false
)

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
			Name:   whitelist,
			Usage:  "path to file containing a whitelist of channels",
			Hidden: false,
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
			Usage: "if you have the choice between IPv6 and IPv4 prefer IPv4 (default: false)",
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
		&cli.BoolFlag{
			Name:   "userest",
			Usage:  "use REST API when true instead of gRPC",
			Hidden: true,
		},
		&cli.DurationFlag{
			Name:   "keepalive",
			Usage:  "keepalive interval (if nothing changed after this time an empty message will be sent)",
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
			Usage:  "report URL",
			Value:  "https://ingress.bolt.observer/api/node-data-report/",
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "uniqueid",
			Usage:  "unique identifier",
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
		&cli.StringFlag{
			Name:   "datastore-url",
			Usage:  "datastore URL",
			Value:  "agent-api.bolt.observer:443",
			Hidden: true,
		},
		&cli.Int64Flag{
			Name:   "fetch-invoices",
			Usage:  "fetch invoices",
			Value:  0,
			Hidden: true,
		},
		&cli.Int64Flag{
			Name:   "fetch-forwards",
			Usage:  "fetch forwards",
			Value:  0,
			Hidden: true,
		},
		&cli.Int64Flag{
			Name:   "fetch-payments",
			Usage:  "fetch payments",
			Value:  0,
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:  "actions",
			Usage: "allow execution of actions defined in the bolt app (beta feature)",
		},
		&cli.BoolFlag{
			Name:  "dryrun",
			Usage: "allow execution of actions in dry run mode (no actions will actually be executed)",
		},
		&cli.BoolFlag{
			Name:   "insecure",
			Usage:  "allow insecure connections to the api. Can be useful for debugging purposes",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:   "plaintext",
			Usage:  "allow plaintext connections to the api. Can be useful for debugging purposes",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:   "noplugins",
			Usage:  "do not load any plugins",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:   "noonchainbalance",
			Usage:  "do not report on-chain balance",
			Hidden: true,
		},
	}

	app.Flags = append(app.Flags, entities.GlogFlags...)
	app.Flags = append(app.Flags, plugins.AllPluginFlags...)
	app.Commands = append(app.Commands, plugins.AllPluginCommands...)

	return app
}

func getInterval(cmdCtx *cli.Context, name string) (agent_entities.Interval, error) {
	i := agent_entities.Interval(0)
	s := strings.ToLower(cmdCtx.String(name))

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

func getVersion() string {
	all := make([]string, 0)

	if !noplugins {
		for _, val := range plugins.RegisteredPlugins {
			all = append(all, val.Name)
		}
	}
	if len(all) > 0 {
		return fmt.Sprintf("bolt-agent/%s+%s", GitRevision, strings.Join(all, ","))
	} else {
		return fmt.Sprintf("bolt-agent/%s", GitRevision)
	}
}

func nodeDataCallback(ctx context.Context, report *agent_entities.NodeDataReport) bool {
	if report == nil {
		return true
	}
	if report.NodeDetails != nil {
		// a bit cumbersome but we don't want to put GitRevision into the checker
		report.NodeDetails.AgentVersion = getVersion()
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

	req.Header.Set("User-Agent", getVersion())
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
		bodyData, _ := io.ReadAll(resp.Body)

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

func runAgent(cmdCtx *cli.Context) error {
	apiKey = utils.GetEnvWithDefault("API_KEY", "")
	if apiKey == "" {
		apiKey = cmdCtx.String("apikey")
	}
	if apiKey == "" {
		// We don't return error here since we don't want glog to handle it
		fmt.Fprintf(os.Stderr, "missing API key (use --apikey or set API_KEY environment variable)\n")
		os.Exit(1)
	}

	noplugins = cmdCtx.Bool("noplugins")
	ct := context.Background()

	var err error

	f, _ := filter.NewAllowAllFilter()
	if cmdCtx.String(whitelist) != "" {
		if _, err = os.Stat(cmdCtx.String(whitelist)); err != nil {
			// We don't return error here since we don't want glog to handle it
			fmt.Fprintf(os.Stderr, "%s points to non-existing file", whitelist)
			os.Exit(1)
		}

		o := filter.None

		if cmdCtx.Bool("private") {
			o |= filter.AllowAllPrivate
		}

		if cmdCtx.Bool("public") {
			o |= filter.AllowAllPublic
		}

		f, err = filter.NewFilterFromFile(ct, cmdCtx.String(whitelist), o)
		if err != nil {
			return err
		}
	} else {
		if !cmdCtx.Bool("private") {
			x := f.(*filter.AllowAllFilter)
			x.Options = filter.AllowAllPublic
		}
	}
	go signalHandler(ct, f)

	url = cmdCtx.String("url")
	private = cmdCtx.Bool("private") || cmdCtx.String(whitelist) != ""
	preferipv4 = cmdCtx.Bool("preferipv4")

	interval, err := getInterval(cmdCtx, "interval")
	if err != nil {
		return err
	}

	if interval == agent_entities.Second {
		// Second is just for testing purposes
		interval = agent_entities.TenSeconds
	}

	g, gctx := errgroup.WithContext(ct)

	var nodeDataChecker *nodedata.NodeData

	if periodicSend {
		nodeDataChecker = nodedata.NewDefaultNodeData(ct, cmdCtx.Duration("keepalive"), cmdCtx.Bool("smooth"), cmdCtx.Bool("checkgraph"), cmdCtx.Bool("noonchainbalance"), checkermonitoring.NewNopCheckerMonitoring("nodedata checker"))
		settings := agent_entities.ReportingSettings{PollInterval: interval, AllowedEntropy: cmdCtx.Int("allowedentropy"), AllowPrivateChannels: private, Filter: f}

		if settings.PollInterval == agent_entities.ManualRequest {
			nodeDataChecker.GetState("", cmdCtx.String("uniqueid"), entities.MkGetLndAPI(cmdCtx), settings, nodeDataCallback)
		} else {
			err = nodeDataChecker.Subscribe(
				nodeDataCallback,
				entities.MkGetLndAPI(cmdCtx),
				"",
				settings,
				cmdCtx.String("uniqueid"),
			)

			if err != nil {
				return err
			}

			glog.Info("Waiting for events...")

			g.Go(func() error {
				nodeDataChecker.EventLoop()
				return nil
			})
		}
	}

	if cmdCtx.Int64("fetch-invoices") != 0 || cmdCtx.Int64("fetch-payments") != 0 || cmdCtx.Int64("fetch-transactions") != 0 {
		g.Go(func() error {
			return sender(gctx, cmdCtx, apiKey)
		})
	}

	if cmdCtx.Bool("actions") {
		fn := entities.MkGetLndAPI(cmdCtx)
		if !noplugins {
			// Need this due to https://stackoverflow.com/questions/43059653/golang-interfacenil-is-nil-or-not
			var invalidatable agent_entities.Invalidatable
			if nodeDataChecker == nil {
				invalidatable = nil
			} else {
				invalidatable = agent_entities.Invalidatable(nodeDataChecker)
			}
			plugins.InitPlugins(fn, f, cmdCtx, invalidatable)
		}
		g.Go(func() error {
			ac := &actions.Connector{
				Address:     cmdCtx.String("datastore-url"),
				APIKey:      apiKey,
				Plugins:     plugins.Plugins,
				LnAPI:       fn,
				IsPlaintext: cmdCtx.Bool("plaintext"),
				IsInsecure:  cmdCtx.Bool("insecure"),
				IsDryRun:    cmdCtx.Bool("dryrun"),
			}
			// exponential backoff is used to reconnect if api is down
			b := backoff.NewExponentialBackOff()
			b.MaxElapsedTime = 0
			return backoff.RetryNotify(func() error {
				return ac.Run(gctx, b.Reset)
			}, b, func(err error, d time.Duration) {
				glog.Errorf("Could not connect to actions gRPC: %v", err)
			})
		})
	}
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		glog.Errorf("Error while running agent: %v", err)
	}

	return nil
}

type fetchSettings struct {
	enabled                 bool
	time                    time.Time
	useLatestTimeFromServer bool
}

func convertTimeSetting(fetchSettingsValue int64) fetchSettings {
	fetchSettingsAbsValue := int64(0)
	if fetchSettingsValue < 0 {
		fetchSettingsAbsValue = -fetchSettingsValue
	} else {
		fetchSettingsAbsValue = fetchSettingsValue
	}

	return fetchSettings{
		enabled:                 fetchSettingsValue != 0,
		time:                    time.Unix(fetchSettingsAbsValue, 0),
		useLatestTimeFromServer: fetchSettingsValue > 0,
	}
}

func sender(ctx context.Context, cmdCtx *cli.Context, apiKey string) error {
	glog.Infof("Datastore server URL: %s", cmdCtx.String("datastore-url"))

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 0

	err := backoff.RetryNotify(func() error {
		return senderWithRetries(ctx, cmdCtx, apiKey)
	}, b, func(e error, d time.Duration) {
		glog.Warningf("Could not init GRPC sending %v\n", e)
	})
	if err != nil {
		glog.Warningf("Fatal error with GRPC sending init %v\n", err)
		return err
	}
	return nil
}

func senderWithRetries(ctx context.Context, cmdCtx *cli.Context, apiKey string) error {
	var permanent *backoff.PermanentError

	sender, err := raw.MakeSender(ctx, apiKey, cmdCtx.String("datastore-url"), entities.MkGetLndAPI(cmdCtx), cmdCtx.Bool("insecure"))
	if err != nil {
		if permanent.Is(err) {
			return backoff.Permanent(fmt.Errorf("get GRPC fetcher failure %v", err))
		}
		return fmt.Errorf("get GRPC fetcher failure %v", err)
	}

	s := convertTimeSetting(cmdCtx.Int64("fetch-invoices"))
	if s.enabled {
		glog.Infof("Fetching invoices after %v\n", s.time)
		go sender.SendInvoices(ctx, s.useLatestTimeFromServer, s.time)
	}

	s = convertTimeSetting(cmdCtx.Int64("fetch-forwards"))
	if s.enabled {
		glog.Infof("Fetching forwards after %v\n", s.time)
		go sender.SendForwards(ctx, s.useLatestTimeFromServer, s.time)
	}

	s = convertTimeSetting(cmdCtx.Int64("fetch-payments"))
	if s.enabled {
		glog.Infof("Fetching payments after %v\n", s.time)
		go sender.SendPayments(ctx, s.useLatestTimeFromServer, s.time)
	}

	return nil
}

func main() {
	app := getApp()
	app.Name = "bolt-agent"
	app.Usage = "Utility to monitor and manage lightning node"
	app.Action = func(cmdCtx *cli.Context) error {
		entities.GlogShim(cmdCtx)
		if err := runAgent(cmdCtx); err != nil {
			return err
		}

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		glog.Error(err)
	}
}
