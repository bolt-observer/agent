package channelchecker

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	entities "github.com/bolt-observer/agent/entities"
	api "github.com/bolt-observer/agent/lightning_api"
	utils "github.com/bolt-observer/go_common/utils"
	"github.com/golang/glog"
	"github.com/marpaia/graphite-golang"
)

type SetOfChanIds map[uint64]struct{}

type NewApiCall func() api.LightingApiCalls

type ChannelChecker struct {
	ctx               context.Context
	globalSettings    *GlobalSettings
	channelCache      ChannelCache
	nodeChanIds       map[string]SetOfChanIds
	nodeChanIdsNew    map[string]SetOfChanIds
	locallyDisabled   SetOfChanIds
	remotlyDisabled   SetOfChanIds
	smooth            bool          // Should we smooth out fluctuations due to HTLCs
	keepAliveInterval time.Duration // Keepalive interval
	checkGraph        bool          // Should we check gossip
	monitoring        *ChannelCheckerMonitoring
	eventLoopInterval time.Duration
}

func NewDefaultChannelChecker(ctx context.Context, keepAlive time.Duration, smooth bool, checkGraph bool, monitoring *ChannelCheckerMonitoring) *ChannelChecker {
	return NewChannelChecker(ctx, NewInMemoryChannelCache(), keepAlive, smooth, checkGraph, monitoring)
}

func NewChannelChecker(ctx context.Context, cache ChannelCache, keepAlive time.Duration, smooth bool, checkGraph bool, monitoring *ChannelCheckerMonitoring) *ChannelChecker {
	if ctx == nil {
		ctx = getContext()
	}

	if monitoring == nil {
		monitoring = NewNopChannelCheckerMonitoring()
	}

	return &ChannelChecker{
		ctx:               ctx,
		globalSettings:    NewGlobalSettings(),
		channelCache:      cache,
		nodeChanIds:       make(map[string]SetOfChanIds),
		nodeChanIdsNew:    make(map[string]SetOfChanIds),
		locallyDisabled:   make(SetOfChanIds),
		remotlyDisabled:   make(SetOfChanIds),
		smooth:            smooth,
		checkGraph:        checkGraph,
		keepAliveInterval: keepAlive,
		monitoring:        monitoring,
		eventLoopInterval: 10 * time.Second,
	}
}

// Check if we are subscribed for a certain public key
func (c *ChannelChecker) IsSubscribed(pubKey string) bool {
	return utils.Contains(c.globalSettings.GetKeys(), pubKey)
}

// Subscribe to notifications about channel changes (if already subscribed this will force a callback, use IsSubscribed to check)
func (c *ChannelChecker) Subscribe(
	pubKey string,
	getApi NewApiCall,
	settings entities.ReportingSettings,
	callback entities.BalanceReportCallback) error {

	api := getApi()
	if api == nil {
		return fmt.Errorf("failed to get client")
	}
	defer api.Cleanup()

	if settings.GraphPollInterval == 0 {
		settings.GraphPollInterval = 30 * time.Minute
	}

	info, err := api.GetInfo(c.ctx)

	if err != nil {
		return fmt.Errorf("failed to get info: %v", err)
	}
	if pubKey != "" && !strings.EqualFold(info.IdentityPubkey, pubKey) {
		return fmt.Errorf("pubkey and reported pubkey are not the same %s vs %s", info.IdentityPubkey, pubKey)
	}

	if settings.NoopInterval == 0 {
		settings.NoopInterval = c.keepAliveInterval
	}

	c.globalSettings.Set(info.IdentityPubkey, Settings{
		settings:       settings,
		lastCheck:      time.Time{},
		lastGraphCheck: time.Time{},
		lastReport:     time.Time{},
		callback:       callback,
		getApi:         getApi,
	})

	return nil
}

// Unsubscribe from a pubkey
func (c *ChannelChecker) Unsubscribe(pubkey string) error {
	c.globalSettings.Delete(pubkey)
	return nil
}

// Get current state (settings.pollInterval is ignored)
func (c *ChannelChecker) GetState(
	pubKey string,
	getApi NewApiCall,
	settings entities.ReportingSettings,
	optCallback entities.BalanceReportCallback) (*entities.ChannelBalanceReport, error) {

	resp, err := c.checkOne(pubKey, getApi, settings, true, false)
	if err != nil {
		return nil, err
	}

	resp.PollInterval = entities.MANUAL_REQUEST
	if optCallback != nil {
		optCallback(c.ctx, resp)
	}
	return resp, err
}

// CustomMetric is a method to send custom metrics via graphite
func (c *ChannelChecker) CustomMetric(name, value string) {
	c.monitoring.graphite.SendMetrics([]graphite.Metric{
		graphite.NewMetric(fmt.Sprintf("%s.%s.%s.%s", PREFIX, c.monitoring.env, name, value), "1", time.Now().Unix()),
	})
}

func (c *ChannelChecker) getChannelList(
	api api.LightingApiCalls,
	info *api.InfoApi,
	precisionBits int,
	allowPrivateChans bool) ([]entities.ChannelBalance, SetOfChanIds, error) {

	defer c.metricsTimer(fmt.Sprintf("channellist.%s", info.IdentityPubkey))()

	resp := make([]entities.ChannelBalance, 0)

	if precisionBits < 1 {
		return nil, nil, fmt.Errorf("wrong precision bits")
	}

	precision := ^uint64(0)
	if precisionBits < 64 {
		precision = uint64(1 << uint64(precisionBits))
	}

	channels, err := api.GetChannels(c.ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get channel list: %v", err)
	}

	chanIds := make(SetOfChanIds)

	for _, channel := range channels.Channels {
		if channel.Private && !allowPrivateChans {
			continue
		}

		// Save channel ID
		chanIds[channel.ChanId] = struct{}{}

		remoteBalance := channel.RemoteBalance
		capacity := uint64(channel.Capacity)

		// The first misunderstanding here is assumption that always when remoteBalance goes up
		// localBalance goes down and vice versa. That is NOT the case - imagine that remoteBalance gets
		// converted into (lower remoteBalance + HTLC) and localBalance stays the same (when you receive HTLC
		// but before it is "settled") and same thing in the opposite direction with localBalance.
		// However we track only remoteBalance as an approximation for the balance of the channel.
		if c.smooth {

			// Smooth out htlcs
			for _, htlc := range channel.PendingHtlcs {
				if htlc.Incoming {
					// In case of incoming HTLC remoteBalance was already decreased
					remoteBalance = utils.Min(capacity, remoteBalance+htlc.Amount)
				}
				// in the other case localBalance was already decreased
				// but this has no effect on remoteBalance!
			}

			// Smooth out commit fee
			if !channel.Initiator {
				// When the other side is initiator of the channel remoteBalance will fluctuate with commitFee
				remoteBalance = utils.Min(capacity, remoteBalance+channel.CommitFee)
			}
		}

		// total != capacity due to channel reserve
		//total := uint64(channel.LocalBalance + channel.RemoteBalance)
		total := uint64(channel.Capacity)

		factor := float64(1)
		if total > precision {
			factor = float64(total) / float64(precision)
		}

		_, locallyDisabled := c.locallyDisabled[channel.ChanId]
		_, remotlyDisabled := c.remotlyDisabled[channel.ChanId]

		resp = append(resp, entities.ChannelBalance{
			Active:       channel.Active,
			Private:      channel.Private,
			LocalPubkey:  info.IdentityPubkey,
			RemotePubkey: channel.RemotePubkey,
			ChanId:       channel.ChanId,
			Capacity:     capacity,
			Nominator:    uint64(math.Round(float64(remoteBalance) / factor)),
			Denominator:  uint64(math.Round(float64(total) / factor)),

			ActiveRemote: !remotlyDisabled,
			ActiveLocal:  !locallyDisabled,
		})
	}

	return resp, chanIds, nil
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

// WARNING: this should not be used except for unit testing
func (c *ChannelChecker) OverrideLoopInterval(duration time.Duration) {
	c.eventLoopInterval = duration
}

func (c *ChannelChecker) EventLoop() {
	ticker := time.NewTicker(c.eventLoopInterval)

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

func (c *ChannelChecker) fetchGraph(
	pubKey string,
	getApi NewApiCall,
	settings entities.ReportingSettings,
) error {

	api := getApi()
	if api == nil {
		return fmt.Errorf("failed to get client")
	}
	defer api.Cleanup()

	graph, err := api.DescribeGraph(c.ctx, settings.AllowPrivateChannels)
	if err != nil {
		return fmt.Errorf("failed to get graph: %v", err)
	}

	locallyDisabled := make(SetOfChanIds)
	remotlyDisabled := make(SetOfChanIds)

	for _, channel := range graph.Channels {
		if channel.Node1Pub != pubKey && channel.Node2Pub != pubKey {
			continue
		}

		localPolicy := channel.Node1Policy
		remotePolicy := channel.Node2Policy

		if channel.Node2Pub == pubKey {
			localPolicy = channel.Node2Policy
			remotePolicy = channel.Node1Policy
		}

		if localPolicy != nil && localPolicy.Disabled {
			locallyDisabled[channel.ChannelId] = struct{}{}
		}

		if remotePolicy != nil && remotePolicy.Disabled {
			remotlyDisabled[channel.ChannelId] = struct{}{}
		}
	}

	c.locallyDisabled = locallyDisabled
	c.remotlyDisabled = remotlyDisabled

	return nil
}

func (c *ChannelChecker) checkAll() bool {
	defer c.metricsTimer("checkall.global")()

	now := time.Now()
	for _, one := range c.globalSettings.GetKeys() {
		s := c.globalSettings.Get(one)

		if c.checkGraph && s.settings.GraphPollInterval != 0 {
			graphToBeCheckedBy := s.lastGraphCheck.Add(s.settings.GraphPollInterval)

			if graphToBeCheckedBy.Before(now) {
				err := c.fetchGraph(s.identifier.Identifier, s.getApi, s.settings)
				if err != nil {
					glog.Warningf("Could not fetch graph from %s: %v", s.identifier.Identifier, err)
				}

				s.lastGraphCheck = now
			}
		}

		toBeCheckedBy := s.lastCheck.Add(s.settings.PollInterval.Duration())
		reportAnyway := false

		if s.settings.NoopInterval != 0 {
			reportAnyway = s.lastReport.Add(s.settings.NoopInterval).Before(now)
		}

		if toBeCheckedBy.Before(now) {

			// Subscribe will set lastCheck to min value and you expect update in such a case
			ignoreCache := toBeCheckedBy.Year() <= 1
			resp, err := c.checkOne(s.identifier.Identifier, s.getApi, s.settings, ignoreCache, reportAnyway)
			if err != nil {
				glog.Warningf("Check failed: %v", err)
				continue
			}

			if resp != nil && !strings.EqualFold(resp.PubKey, one) {
				glog.Warningf("PubKey mismatch %s vs %s", resp.PubKey, one)
				continue
			}

			if resp != nil {
				go func(c *ChannelChecker, one string, now time.Time, s Settings, resp *entities.ChannelBalanceReport) {
					if s.callback(c.ctx, resp) {
						s.lastReport = now
						c.commitAllChanges(one, now, s)
					} else {
						c.revertAllChanges()
					}
				}(c, one, now, s, resp)
			} else {
				c.commitAllChanges(one, now, s)
			}
		}

		select {
		case <-c.ctx.Done():
			return false
		default:
			continue
		}
	}

	c.metricsReport("checkall.global", "success")
	return true
}

// checkOne checks one specific node
func (c *ChannelChecker) checkOne(
	pubKey string,
	getApi NewApiCall,
	settings entities.ReportingSettings,
	ignoreCache bool,
	reportAnyway bool) (*entities.ChannelBalanceReport, error) {

	metricsName := fmt.Sprintf("checkone.%s", pubKey)
	defer c.metricsTimer(metricsName)()

	api := getApi()
	if api == nil {
		c.metricsReport(metricsName, "failure")
		return nil, fmt.Errorf("failed to get client")
	}
	defer api.Cleanup()

	info, err := api.GetInfo(c.ctx)
	if err != nil {
		c.metricsReport(metricsName, "failure")
		return nil, fmt.Errorf("failed to get info: %v", err)
	}

	if pubKey != "" && !strings.EqualFold(info.IdentityPubkey, pubKey) {
		c.metricsReport(metricsName, "failure")
		return nil, fmt.Errorf("pubkey and reported pubkey are not the same")
	}

	channelList, set, err := c.getChannelList(api, info, settings.AllowedEntropy, settings.AllowPrivateChannels)
	if err != nil {
		c.metricsReport(metricsName, "failure")
		return nil, err
	}

	closedChannels := make([]entities.ClosedChannel, 0)

	if len(c.nodeChanIds[info.IdentityPubkey]) > 0 {
		// We must have some old channel set

		// diff between new -> old
		closed := utils.SetDiff(set, c.nodeChanIds[info.IdentityPubkey])
		for k := range closed {
			closedChannels = append(closedChannels, entities.ClosedChannel{ChannelId: k})
		}
	}

	c.nodeChanIdsNew[info.IdentityPubkey] = set

	channelList = c.filterList(channelList, ignoreCache)

	resp := &entities.ChannelBalanceReport{
		ReportingSettings: settings,
		Chain:             info.Chain,
		Network:           info.Network,
		PubKey:            info.IdentityPubkey,
		Timestamp:         entities.JsonTime(time.Now()),
		ChangedChannels:   channelList,
		ClosedChannels:    closedChannels,
	}

	if len(channelList) <= 0 && len(closedChannels) <= 0 && !reportAnyway {
		resp = nil
	}

	c.metricsReport(metricsName, "success")
	return resp, nil
}

// filterList will return just the changed channels
func (c *ChannelChecker) filterList(
	list []entities.ChannelBalance,
	noop bool) []entities.ChannelBalance {

	resp := make([]entities.ChannelBalance, 0)

	c.channelCache.Lock()
	defer c.channelCache.Unlock()

	for _, one := range list {
		// We need local pubkey in case same channel is monitored from two sides
		// which could trigger invalidations all the time
		id := fmt.Sprintf("%s-%d", one.LocalPubkey, one.ChanId)
		val, ok := c.channelCache.Get(id)
		// Note that denominator changes based on channel reserve (might use capacity here)

		/*
			current := fmt.Sprintf("%v-%d-%d-%v-%v", one.Active, one.Nominator, one.Denominator, one.Active, one.Active)
			if noop || !ok || val != current {

				oldActive, oldNom, oldDenom, oldActiveLocal, oldActiveRemote, err := parseOldVal(val)

				if err != nil {
					one.NominatorDiff = 0
					one.DenominatorDiff = 0
					one.ActivePrevious = true
					one.ActiveLocalPrevious = true
					one.ActiveRemotePrevious = true
				} else {
					one.NominatorDiff = int64(one.Nominator) - int64(oldNom)
					one.DenominatorDiff = int64(one.Denominator) - int64(oldDenom)
					one.ActivePrevious = oldActive
					one.ActiveLocalPrevious = oldActiveLocal
					one.ActiveRemotePrevious = oldActiveRemote
				}

				resp = append(resp, one)

				c.channelCache.DeferredSet(id, val, current)
			}
		*/

		current := fmt.Sprintf("%v-%d-%d", one.Active, one.Nominator, one.Denominator)
		if noop || !ok || val != current {

			oldActive, oldNom, oldDenom, err := parseOldValLegacy(val)

			if err != nil {
				one.NominatorDiff = 0
				one.DenominatorDiff = 0
				one.ActivePrevious = true
			} else {
				one.NominatorDiff = int64(one.Nominator) - int64(oldNom)
				one.DenominatorDiff = int64(one.Denominator) - int64(oldDenom)
				one.ActivePrevious = oldActive
			}

			one.ActiveLocalPrevious = true
			one.ActiveRemotePrevious = true

			resp = append(resp, one)

			c.channelCache.DeferredSet(id, val, current)
		}

	}

	return resp
}

func (c *ChannelChecker) commitAllChanges(one string, now time.Time, s Settings) {
	c.commitChanIdChanges()
	c.channelCache.DeferredCommit()
	s.lastCheck = now
	c.globalSettings.Set(one, s)
}

func (c *ChannelChecker) revertAllChanges() {
	c.revertChanIdChanges()
	c.channelCache.DeferredRevert()
}

func (c *ChannelChecker) commitChanIdChanges() {
	for k, v := range c.nodeChanIdsNew {
		c.nodeChanIds[k] = v
	}

	for k := range c.nodeChanIdsNew {
		delete(c.nodeChanIdsNew, k)
	}
}

func (c *ChannelChecker) revertChanIdChanges() {
	for k := range c.nodeChanIdsNew {
		delete(c.nodeChanIdsNew, k)
	}
}

// parseOldVal is just used for determining diff (deprecated)
/*
func parseOldVal(val string) (bool, uint64, uint64, bool, bool, error) {
	parsed := strings.Split(val, "-")
	if len(parsed) != 5 {
		return true, 0, 0, true, true, fmt.Errorf("bad old value: %v", val)
	}

	b, err := strconv.ParseBool(parsed[0])
	if err != nil {
		return true, 0, 0, true, true, fmt.Errorf("bad old value: %v", val)
	}

	n1, err := strconv.ParseInt(parsed[1], 10, 64)
	if err != nil {
		return true, 0, 0, true, true, fmt.Errorf("bad old value: %v", val)
	}

	n2, err := strconv.ParseInt(parsed[2], 10, 64)
	if err != nil {
		return true, 0, 0, true, true, fmt.Errorf("bad old value: %v", val)
	}

	b1, err := strconv.ParseBool(parsed[3])
	if err != nil {
		return true, 0, 0, true, true, fmt.Errorf("bad old value: %v", val)
	}

	b2, err := strconv.ParseBool(parsed[4])
	if err != nil {
		return true, 0, 0, true, true, fmt.Errorf("bad old value: %v", val)
	}

	return b, uint64(n1), uint64(n2), b1, b2, nil
}
*/

// Deprecated
func parseOldValLegacy(val string) (bool, uint64, uint64, error) {
	parsed := strings.Split(val, "-")
	if len(parsed) != 3 {
		return true, 0, 0, fmt.Errorf("bad old value: %v", val)
	}

	b, err := strconv.ParseBool(parsed[0])
	if err != nil {
		return true, 0, 0, fmt.Errorf("bad old value: %v", val)
	}

	n1, err := strconv.ParseInt(parsed[1], 10, 64)
	if err != nil {
		return true, 0, 0, fmt.Errorf("bad old value: %v", val)
	}

	n2, err := strconv.ParseInt(parsed[2], 10, 64)
	if err != nil {
		return true, 0, 0, fmt.Errorf("bad old value: %v", val)
	}

	return b, uint64(n1), uint64(n2), nil
}
