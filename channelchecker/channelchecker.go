package channelchecker

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
)

type SetOfChanIds map[uint64]struct{}

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
	monitoring        *checkermonitoring.CheckerMonitoring
	eventLoopInterval time.Duration
}

func NewDefaultChannelChecker(ctx context.Context, keepAlive time.Duration, smooth bool, checkGraph bool, monitoring *checkermonitoring.CheckerMonitoring) *ChannelChecker {
	return NewChannelChecker(ctx, NewInMemoryChannelCache(), keepAlive, smooth, checkGraph, monitoring)
}

func NewChannelChecker(ctx context.Context, cache ChannelCache, keepAlive time.Duration, smooth bool, checkGraph bool, monitoring *checkermonitoring.CheckerMonitoring) *ChannelChecker {
	if ctx == nil {
		ctx = getContext()
	}

	if monitoring == nil {
		monitoring = checkermonitoring.NewNopCheckerMonitoring("channelchecker")
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
func (c *ChannelChecker) IsSubscribed(pubKey, uniqueId string) bool {
	return utils.Contains(c.globalSettings.GetKeys(), pubKey+uniqueId)
}

// Subscribe to notifications about channel changes (if already subscribed this will force a callback, use IsSubscribed to check)
func (c *ChannelChecker) Subscribe(
	pubKey string,
	uniqueId string,
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
		identifier:     entities.NodeIdentifier{Identifier: pubKey, UniqueId: uniqueId},
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
func (c *ChannelChecker) Unsubscribe(pubkey, uniqueId string) error {
	c.globalSettings.Delete(pubkey + uniqueId)
	return nil
}

// Get current state (settings.pollInterval is ignored)
func (c *ChannelChecker) GetState(
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

func (c *ChannelChecker) getChannelList(
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
			for _, htlc := range channel.PendingHtlcs {
				if htlc.Incoming {
					// In case of incoming HTLC remoteBalance was already decreased
					remoteBalance = utils.Min(capacity, remoteBalance+htlc.Amount)
				} else {
					localBalance = utils.Min(capacity, localBalance+htlc.Amount)
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

// WARNING: this should not be used except for unit testing
func (c *ChannelChecker) OverrideLoopInterval(duration time.Duration) {
	c.eventLoopInterval = duration
}

func (c *ChannelChecker) EventLoop() {
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

func (c *ChannelChecker) fetchGraph(
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

func (c *ChannelChecker) checkAll() bool {
	defer c.monitoring.MetricsTimer("checkall.global")()

	now := time.Now()
	for _, one := range c.globalSettings.GetKeys() {
		s := c.globalSettings.Get(one)

		if c.checkGraph && s.settings.GraphPollInterval != 0 {
			graphToBeCheckedBy := s.lastGraphCheck.Add(s.settings.GraphPollInterval)

			if graphToBeCheckedBy.Before(now) && s.identifier.Identifier != "" {
				// Beware: here the graph is fetched just per node
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
			resp, err := c.checkOne(s.identifier, s.getApi, s.settings, ignoreCache, reportAnyway)
			if err != nil {
				sentry.CaptureException(err)
				glog.Warningf("Check failed: %v", err)
				continue
			}

			if resp != nil && s.identifier.Identifier != "" && !strings.EqualFold(resp.PubKey, s.identifier.Identifier) {
				sentry.CaptureMessage(fmt.Sprintf("PubKey mismatch %s vs %s", resp.PubKey, s.identifier.Identifier))
				glog.Warningf("PubKey mismatch %s vs %s", resp.PubKey, s.identifier.Identifier)
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

	c.monitoring.MetricsReport("checkall.global", "success")
	return true
}

// checkOne checks one specific node
func (c *ChannelChecker) checkOne(
	identifier entities.NodeIdentifier,
	getApi entities.NewApiCall,
	settings entities.ReportingSettings,
	ignoreCache bool,
	reportAnyway bool) (*entities.ChannelBalanceReport, error) {

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

	info, err := api.GetInfo(c.ctx)
	if err != nil {
		c.monitoring.MetricsReport(metricsName, "failure")
		return nil, fmt.Errorf("failed to get info: %v", err)
	}

	if identifier.Identifier != "" && !strings.EqualFold(info.IdentityPubkey, identifier.Identifier) {
		c.monitoring.MetricsReport(metricsName, "failure")
		return nil, fmt.Errorf("pubkey and reported pubkey are not the same")
	}

	if identifier.Identifier == "" {
		identifier.Identifier = info.IdentityPubkey
	}

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

	channelList = c.filterList(channelList, ignoreCache)

	resp := &entities.ChannelBalanceReport{
		ReportingSettings: settings,
		Chain:             info.Chain,
		Network:           info.Network,
		PubKey:            identifier.Identifier,
		UniqueId:          identifier.UniqueId,
		Timestamp:         common_entities.JsonTime(time.Now()),
		ChangedChannels:   channelList,
		ClosedChannels:    closedChannels,
	}

	if len(channelList) <= 0 && len(closedChannels) <= 0 && !reportAnyway {
		resp = nil
	}

	c.monitoring.MetricsReport(metricsName, "success")
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

			c.fillLegacyFields(&one)

			resp = append(resp, one)

			c.channelCache.DeferredSet(id, val, current)
		}

	}

	return resp
}

// fillLegacyFields is deprecated and will be removed
func (c *ChannelChecker) fillLegacyFields(balance *entities.ChannelBalance) {
	balance.Nominator = balance.RemoteNominator
	balance.NominatorDiff = balance.RemoteNominatorDiff
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
