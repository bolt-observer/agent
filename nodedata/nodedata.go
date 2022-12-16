package nodedata

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	checkermonitoring "github.com/bolt-observer/agent/checkermonitoring"
	entities "github.com/bolt-observer/agent/entities"
	api "github.com/bolt-observer/agent/lightning_api"
	common_entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"
	"github.com/getsentry/sentry-go"
	"github.com/golang/glog"
	"github.com/mitchellh/hashstructure/v2"
)

type SetOfChanIds map[uint64]struct{}

type NodeData struct {
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
	monitoring        *checkermonitoring.CheckerMonitoring
	eventLoopInterval time.Duration
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
				glog.Infof("%v received!\n", sig)
				cancel()
			default:
				time.Sleep(10 * time.Millisecond)
				// nothing
			}
		}
	}()

	return ctx
}

func NewDefaultNodeData(ctx context.Context, keepAlive time.Duration, smooth bool, checkGraph bool, monitoring *checkermonitoring.CheckerMonitoring) *NodeData {
	return NewDefaultNodeData(ctx, NewInMemoryChannelCache(), keepAlive, smooth, checkGraph, monitoring)
}

func NewNodeData(ctx context.Context, cache ChannelCache, keepAlive time.Duration, smooth bool, checkGraph bool, monitoring *checkermonitoring.CheckerMonitoring) *NodeData {
	if ctx == nil {
		ctx = getContext()
	}

	if monitoring == nil {
		monitoring = checkermonitoring.NewNopCheckerMonitoring("NodeData")
	}

	return &NodeData{
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
		reentrancyBlock:   entities.NewReentrancyBlock(),
	}
}

// Check if we are subscribed for a certain public key
func (c *NodeData) IsSubscribed(pubKey, uniqueId string) bool {
	return utils.Contains(c.globalSettings.GetKeys(), pubKey+uniqueId)
}

// Subscribe to notifications about channel changes (if already subscribed this will force a callback, use IsSubscribed to check)
func (c *NodeData) Subscribe(
	pubKey string,
	uniqueId string,
	private bool,
	PollInterval entities.Interval,
	getApi entities.NewApiCall,
	settings entities.ReportingSettings,
	callback entities.BalanceReportCallback) error {

	if pubKey != "" && !utils.ValidatePubkey(pubKey) {
		return errors.New("invalid pubkey")
	}

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

	c.globalSettings.Set(info.IdentityPubkey+uniqueId, Settings{
		callback:       callback,
		getApi:         getApi,
		hash:           0,
		identifier:     entities.NodeIdentifier{Identifier: pubKey, UniqueId: uniqueId},
		lastCheck:      time.Time{},
		lastGraphCheck: time.Time{},
		lastReport:     time.Time{},
		private:        private,
		settings:       settings,
	})

	return nil
}

// Unsubscribe from a pubkey
func (c *NodeData) Unsubscribe(pubkey, uniqueId string) error {
	c.globalSettings.Delete(pubkey + uniqueId)
	return nil
}

// Get current state (settings.pollInterval is ignored)
func (c *NodeData) GetState(
	pubKey string,
	uniqueId string,
	getApi entities.NewApiCall,
	settings entities.ReportingSettings,
	optCallback entities.BalanceReportCallback) (*entities.ChannelBalanceReport, error) {

	if pubKey != "" && !utils.ValidatePubkey(pubKey) {
		return nil, errors.New("invalid pubkey")
	}

	resp, err := c.checkOne(entities.NodeIdentifier{Identifier: pubKey, UniqueId: uniqueId}, getApi, settings, true, false)
	if err != nil {
		return nil, err
	}

	resp.PollInterval = entities.MANUAL_REQUEST
	if optCallback != nil {
		optCallback(c.ctx, resp)
	}
	return resp, err
}

func (c *NodeData) getChannelList(
	api api.LightingApiCalls,
	info *api.InfoApi,
	precisionBits int,
	allowPrivateChans bool) ([]entities.ChannelBalance, SetOfChanIds, error) {

	defer c.monitoring.MetricsTimer(fmt.Sprintf("channellist.%s", info.IdentityPubkey))()

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
		localBalance := channel.LocalBalance
		capacity := uint64(channel.Capacity)

		// The first misunderstanding here is assumption that always when remoteBalance goes up
		// localBalance goes down and vice versa. That is NOT the case - imagine that remoteBalance gets
		// converted into (lower remoteBalance + HTLC) and localBalance stays the same (when you receive HTLC
		// but before it is "settled") and same thing in the opposite direction with localBalance.
		// However we track only remoteBalance as an approximation for the balance of the channel.
		if c.smooth {

			// Smooth out htlcs
			if channel.PendingHtlcs != nil {
				for _, htlc := range channel.PendingHtlcs {
					if htlc.Incoming {
						// In case of incoming HTLC remoteBalance was already decreased
						remoteBalance = utils.Min(capacity, remoteBalance+htlc.Amount)
					} else {
						localBalance = utils.Min(capacity, localBalance+htlc.Amount)
					}
				}
			}

			// Smooth out commit fee
			if !channel.Initiator {
				// When the other side is initiator of the channel remoteBalance will fluctuate with commitFee
				remoteBalance = utils.Min(capacity, remoteBalance+channel.CommitFee)
			} else {
				// Same for the local case
				localBalance = utils.Min(capacity, localBalance+channel.CommitFee)
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
			Active:          channel.Active,
			Private:         channel.Private,
			LocalPubkey:     info.IdentityPubkey,
			RemotePubkey:    channel.RemotePubkey,
			ChanId:          channel.ChanId,
			Capacity:        capacity,
			RemoteNominator: uint64(math.Round(float64(remoteBalance) / factor)),
			LocalNominator:  uint64(math.Round(float64(localBalance) / factor)),
			Denominator:     uint64(math.Round(float64(total) / factor)),

			ActiveRemote: !remotlyDisabled,
			ActiveLocal:  !locallyDisabled,
		})
	}

	return resp, chanIds, nil
}

// WARNING: this should not be used except for unit testing
func (c *NodeData) OverrideLoopInterval(duration time.Duration) {
	c.eventLoopInterval = duration
}

func (c *NodeData) EventLoop() {
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

func (c *NodeData) fetchGraph(
	pubKey string,
	getApi entities.NewApiCall,
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

func (c *NodeData) checkAll() bool {
	defer c.monitoring.MetricsTimer("checkall.global")()

	for _, one := range c.globalSettings.GetKeys() {
		now := time.Now()
		s := c.globalSettings.Get(one)
		if c.checkGraph && s.settings.GraphPollInterval != 0 {
			graphToBeCheckedBy := s.lastGraphCheck.Add(s.settings.GraphPollInterval)

			if graphToBeCheckedBy.Before(now) && s.identifier.Identifier != "" {
				// Beware: here the graph is fetched just per node
				err := c.fetchGraph(s.identifier.Identifier, s.getApi, s.settings)
				if err != nil {
					glog.Warningf("Could not fetch graph from %s: %v", s.identifier.Identifier, err)
				}

				s.lastGraphCheck = time.Now()
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
			resp, err := c.checkOne(s.identifier, s.getApi, s.settings, ignoreCache, reportAnyway)
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

			if resp != nil && s.identifier.Identifier != "" && !strings.EqualFold(resp.PubKey, s.identifier.Identifier) {
				sentry.CaptureMessage(fmt.Sprintf("PubKey mismatch %s vs %s", resp.PubKey, s.identifier.Identifier))
				glog.Warningf("PubKey mismatch %s vs %s", resp.PubKey, s.identifier.Identifier)
				continue
			}

			if resp != nil {
				go func(c *NodeData, one string, now time.Time, s Settings, resp *entities.ChannelBalanceReport, hash uint64) {
					if !c.reentrancyBlock.Enter(one) {
						glog.Warningf("Reentrancy of callback for %s not allowed", one)
						return
					}
					defer c.reentrancyBlock.Release(one)

					metricsName := fmt.Sprintf("checkdelivery.%s", s.identifier.GetId())
					timer := c.monitoring.MetricsTimer(metricsName)
					// NB: now can be old here
					if s.callback(c.ctx, resp) {
						// Update hash only upon success
						s.hash = hash
						s.lastCheck = time.Now()
						c.globalSettings.Set(one, s)
						s.lastReport = time.Now()
						c.commitAllChanges(one, time.Now(), s)
					} else {
						c.revertAllChanges()
					}
					timer()
				}(c, one, now, s, resp, hash)
			} else {
				s.lastCheck = time.Now()
				c.globalSettings.Set(one, s)
				c.commitAllChanges(one, time.Now(), s)
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

// checkOne checks one specific node
func (c *NodeData) checkOne(
	identifier entities.NodeIdentifier,
	getApi entities.NewApiCall,
	settings entities.ReportingSettings,
	ignoreCache bool,
	reportAnyway bool,
	private bool) (*entities.NodeDataReport, error) {

	name := identifier.GetId()
	if name == "" {
		name = "local"
	}
	metricsName := fmt.Sprintf("checkone.%s", identifier.GetId())
	defer c.monitoring.MetricsTimer(metricsName)()

	if getApi == nil {
		c.monitoring.MetricsReport(metricsName, "failure")
		return nil, fmt.Errorf("failed to get client - getApi was nil")
	}

	api := getApi()
	if api == nil {
		c.monitoring.MetricsReport(metricsName, "failure")
		return nil, fmt.Errorf("failed to get client - getApi returned nil")
	}
	defer api.Cleanup()

	info, err := api.GetNodeInfoFull(c.ctx, true, private)
	if err != nil {
		return nil, fmt.Errorf("failed to call GetNodeInfoFull %v", err)
	}

	// if identifier.Identifier != "" && !strings.EqualFold(info.IdentityPubkey, identifier.Identifier) {
	// 	c.monitoring.MetricsReport(metricsName, "failure")
	// 	return nil, fmt.Errorf("pubkey and reported pubkey are not the same")
	// }

	// if identifier.Identifier == "" {
	// 	identifier.Identifier = info.IdentityPubkey
	// }

	channelList, set, err := c.getChannelList(api, info, settings.AllowedEntropy, settings.AllowPrivateChannels)
	if err != nil {
		c.monitoring.MetricsReport(metricsName, "failure")
		return nil, err
	}

	closedChannels := make([]entities.ClosedChannel, 0)

	if len(c.nodeChanIds[identifier.GetId()]) > 0 {
		// We must have some old channel set

		// diff between new -> old
		closed := utils.SetDiff(set, c.nodeChanIds[identifier.GetId()])
		for k := range closed {
			closedChannels = append(closedChannels, entities.ClosedChannel{ChannelId: k})
		}
	}

	c.nodeChanIdsNew[identifier.GetId()] = set

	channelList = c.filterList(identifier, channelList, ignoreCache)

	resp := &entities.NodeDataReport{
		ReportingSettings: settings,
		// Chain:             info.Chain,
		// Network:           info.Network,
		PubKey:              identifier.Identifier,
		UniqueId:            identifier.UniqueId,
		Timestamp:           common_entities.JsonTime(time.Now()),
		ChangedChannels:     channelList,
		ClosedChannels:      closedChannels,
		NodeInfoApiExtended: *info,
	}

	if len(channelList) <= 0 && len(closedChannels) <= 0 && !reportAnyway {
		resp = nil
	}

	c.monitoring.MetricsReport(metricsName, "success")
	return resp, nil
}

// filterList will return just the changed channels
func (c *NodeData) filterList(
	identifier entities.NodeIdentifier,
	list []entities.ChannelBalance,
	noop bool) []entities.ChannelBalance {

	resp := make([]entities.ChannelBalance, 0)

	c.channelCache.Lock()
	defer c.channelCache.Unlock()

	for _, one := range list {
		// We need local pubkey in case same channel is monitored from two sides
		// which could trigger invalidations all the time
		id := fmt.Sprintf("%s-%d", identifier.GetId(), one.ChanId)
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

		current := fmt.Sprintf("%v-%d-%d-%d", one.Active, one.RemoteNominator, one.LocalNominator, one.Denominator)
		if noop || !ok || val != current {

			oldActive, oldRemoteNom, oldLocalNom, oldDenom, err := parseOldValLegacy(val)

			if err != nil {
				one.RemoteNominatorDiff = 0
				one.LocalNominatorDiff = 0
				one.DenominatorDiff = 0
				one.ActivePrevious = true
				glog.V(3).Infof("Error %v happened - val was %v\n", err, val)
			} else {
				one.RemoteNominatorDiff = int64(one.RemoteNominator) - int64(oldRemoteNom)
				one.LocalNominatorDiff = int64(one.LocalNominator) - int64(oldLocalNom)
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

func (c *NodeData) commitAllChanges(one string, now time.Time, s Settings) {
	c.commitChanIdChanges()
	c.channelCache.DeferredCommit()
	s.lastCheck = now
	c.globalSettings.Set(one, s)
}

func (c *NodeData) revertAllChanges() {
	c.revertChanIdChanges()
	c.channelCache.DeferredRevert()
}

func (c *NodeData) commitChanIdChanges() {
	for k, v := range c.nodeChanIdsNew {
		c.nodeChanIds[k] = v
	}

	for k := range c.nodeChanIdsNew {
		delete(c.nodeChanIdsNew, k)
	}
}

func (c *NodeData) revertChanIdChanges() {
	for k := range c.nodeChanIdsNew {
		delete(c.nodeChanIdsNew, k)
	}
}

// Deprecated
func parseOldValLegacy(val string) (bool, uint64, uint64, uint64, error) {
	parsed := strings.Split(val, "-")
	if len(parsed) != 4 {
		return true, 0, 0, 0, fmt.Errorf("bad old value: %v", val)
	}

	b, err := strconv.ParseBool(parsed[0])
	if err != nil {
		return true, 0, 0, 0, fmt.Errorf("bad old value: %v", val)
	}

	n1, err := strconv.ParseInt(parsed[1], 10, 64)
	if err != nil {
		return true, 0, 0, 0, fmt.Errorf("bad old value: %v", val)
	}

	n2, err := strconv.ParseInt(parsed[2], 10, 64)
	if err != nil {
		return true, 0, 0, 0, fmt.Errorf("bad old value: %v", val)
	}

	n3, err := strconv.ParseInt(parsed[3], 10, 64)
	if err != nil {
		return true, 0, 0, 0, fmt.Errorf("bad old value: %v", val)
	}

	return b, uint64(n1), uint64(n2), uint64(n3), nil
}
