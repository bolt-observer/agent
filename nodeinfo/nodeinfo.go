package nodeinfo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	checkermonitoring "github.com/bolt-observer/agent/checkermonitoring"
	entities "github.com/bolt-observer/agent/entities"
	"github.com/bolt-observer/agent/filter"
	"github.com/bolt-observer/agent/lightning_api"
	common_entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"
	"github.com/golang/glog"
	"github.com/mitchellh/hashstructure/v2"
)

type NodeInfo struct {
	ctx               context.Context
	monitoring        *checkermonitoring.CheckerMonitoring
	eventLoopInterval time.Duration
	globalSettings    *GlobalSettings
	reentrancyBlock   *entities.ReentrancyBlock
}

func getContext() context.Context {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case sig := <-sigc:
				fmt.Printf("%v received!\n", sig)
				cancel()
			default:
				time.Sleep(10 * time.Millisecond)
				// nothing
			}
		}
	}()

	return ctx
}

func NewNodeInfo(ctx context.Context, monitoring *checkermonitoring.CheckerMonitoring) *NodeInfo {
	if ctx == nil {
		ctx = getContext()
	}

	if monitoring == nil {
		monitoring = checkermonitoring.NewNopCheckerMonitoring("nodeinfo")
	}

	return &NodeInfo{
		ctx:               ctx,
		monitoring:        monitoring,
		eventLoopInterval: 10 * time.Second,
		globalSettings:    NewGlobalSettings(),
		reentrancyBlock:   entities.NewReentrancyBlock(),
	}
}

// Check if we are subscribed for a certain public key
func (c *NodeInfo) IsSubscribed(pubKey, uniqueId string) bool {
	return utils.Contains(c.globalSettings.GetKeys(), pubKey+uniqueId)
}

func (c *NodeInfo) GetState(
	pubKey string,
	uniqueId string,
	private bool,
	PollInterval entities.Interval,
	getApi entities.NewApiCall,
	optCallback entities.InfoCallback,
	filter filter.FilterInterface,
) (*entities.InfoReport, error) {

	if pubKey != "" && !utils.ValidatePubkey(pubKey) {
		return nil, errors.New("invalid pubkey")
	}

	resp, err := c.checkOne(entities.NodeIdentifier{Identifier: pubKey, UniqueId: uniqueId}, getApi, private, filter)
	if err != nil {
		return nil, err
	}

	if optCallback != nil && resp != nil {
		optCallback(c.ctx, resp)
	}

	return resp, err
}

// Subscribe to notifications
func (c *NodeInfo) Subscribe(
	pubKey string,
	uniqueId string,
	private bool,
	PollInterval entities.Interval,
	getApi entities.NewApiCall,
	callback entities.InfoCallback,
	f filter.FilterInterface,
) error {

	if pubKey != "" && !utils.ValidatePubkey(pubKey) {
		return errors.New("invalid pubkey")
	}

	api := getApi()
	if api == nil {
		return fmt.Errorf("failed to get client")
	}
	defer api.Cleanup()

	info, err := api.GetInfo(c.ctx)

	if err != nil {
		return fmt.Errorf("failed to get info: %v", err)
	}
	if pubKey != "" && !strings.EqualFold(info.IdentityPubkey, pubKey) {
		return fmt.Errorf("pubkey and reported pubkey are not the same %s vs %s", info.IdentityPubkey, pubKey)
	}

	if f == nil {
		f, _ = filter.NewAllowAllFilter()
	}

	c.globalSettings.Set(info.IdentityPubkey+uniqueId, Settings{
		identifier: entities.NodeIdentifier{Identifier: pubKey, UniqueId: uniqueId},
		lastCheck:  time.Time{},
		callback:   callback,
		getApi:     getApi,
		hash:       0,
		private:    private,
		filter:     f,
	})

	return nil
}

// Unsubscribe from a pubkey
func (c *NodeInfo) Unsubscribe(pubkey, uniqueId string) error {
	c.globalSettings.Delete(pubkey + uniqueId)
	return nil
}

// WARNING: this should not be used except for unit testing
func (c *NodeInfo) OverrideLoopInterval(duration time.Duration) {
	c.eventLoopInterval = duration
}

func (c *NodeInfo) EventLoop() {
	// nosemgrep
	ticker := time.NewTicker(c.eventLoopInterval)

	// Imediately call checkAll()
	if !c.checkAll() {
		ticker.Stop()
		return
	}

	func() {
		for {
			select {
			case <-ticker.C:
				if !c.checkAll() {
					ticker.Stop()
					return
				}
			case <-c.ctx.Done():
				return
			}
		}
	}()
}

func (c *NodeInfo) checkAll() bool {
	defer c.monitoring.MetricsTimer("checkall.global")()

	for _, one := range c.globalSettings.GetKeys() {
		now := time.Now()
		s := c.globalSettings.Get(one)

		toBeCheckedBy := s.lastCheck.Add(s.interval.Duration())
		if toBeCheckedBy.Before(now) {
			resp, err := c.checkOne(s.identifier, s.getApi, s.private, s.filter)
			if err != nil {
				glog.Warningf("Failed to check %v: %v", s.identifier.GetId(), err)
				continue
			}

			hash, err := hashstructure.Hash(resp, hashstructure.FormatV2, nil)
			if err != nil {
				glog.Warning("Hash could not be determined")
				hash = 1
			}

			if hash == s.hash {
				resp = nil
			}

			if resp != nil {
				go func(c *NodeInfo, resp *entities.InfoReport, s Settings, one string, hash uint64) {
					if !c.reentrancyBlock.Enter(one) {
						glog.Warningf("Reentrancy of node callback for %s not allowed", one)
						return
					}
					defer c.reentrancyBlock.Release(one)

					metricsName := fmt.Sprintf("checkdelivery.%s", s.identifier.GetId())
					timer := c.monitoring.MetricsTimer(metricsName)
					if s.callback(c.ctx, resp) {
						// Update hash only upon success
						s.hash = hash
						s.lastCheck = time.Now()
						c.globalSettings.Set(one, s)
					}
					timer()
				}(c, resp, s, one, hash)
			} else {
				s.lastCheck = time.Now()
				c.globalSettings.Set(one, s)
			}
		}

		select {
		case <-c.ctx.Done():
			return false
		default:
			continue
		}
	}

	c.monitoring.MetricsReport("checkall.global", "success")
	return true
}

func applyFilter(info *lightning_api.NodeInfoApiExtended, filter filter.FilterInterface) *lightning_api.NodeInfoApiExtended {
	ret := &lightning_api.NodeInfoApiExtended{
		NodeInfoApi: info.NodeInfoApi,
		Channels:    make([]lightning_api.NodeChannelApiExtended, 0),
	}

	for _, c := range info.Channels {
		nodeAllowed := (filter.AllowPubKey(c.Node1Pub) && info.Node.PubKey != c.Node1Pub) || (filter.AllowPubKey(c.Node2Pub) && info.Node.PubKey != c.Node2Pub)
		chanAllowed := filter.AllowChanId(c.ChannelId)

		if nodeAllowed || chanAllowed {
			ret.Channels = append(ret.Channels, c)
		} else {
			ret.NumChannels -= 1
			ret.TotalCapacity -= c.Capacity
		}
	}

	return ret
}

// checkOne checks one specific node
func (c *NodeInfo) checkOne(
	identifier entities.NodeIdentifier,
	getApi entities.NewApiCall,
	private bool,
	filter filter.FilterInterface,
) (*entities.InfoReport, error) {

	name := identifier.GetId()
	if name == "" {
		name = "local"
	}
	metricsName := fmt.Sprintf("checkone.%s", name)
	defer c.monitoring.MetricsTimer(metricsName)()

	api := getApi()
	if api == nil {
		c.monitoring.MetricsReport(metricsName, "failure")
		return nil, fmt.Errorf("failed to get client")
	}
	defer api.Cleanup()

	info, err := api.GetNodeInfoFull(c.ctx, true, private)
	if err != nil {
		return nil, fmt.Errorf("failed to call GetNodeInfoFull %v", err)
	}

	info = applyFilter(info, filter)

	if len(info.Channels) != int(info.NumChannels) {
		return nil, fmt.Errorf("bad NodeInfo obtained %d channels vs. num_channels %d - %v", len(info.Channels), info.NumChannels, info)
	}

	ret := &entities.InfoReport{
		UniqueId:            identifier.UniqueId,
		Timestamp:           common_entities.JsonTime(time.Now()),
		NodeInfoApiExtended: *info,
	}

	return ret, nil
}
