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

	"github.com/bolt-observer/agent/entities"
	api "github.com/bolt-observer/agent/lightning"
	common_entities "github.com/bolt-observer/go_common/entities"
	"github.com/bolt-observer/go_common/utils"
	"github.com/getsentry/sentry-go"
	"github.com/golang/glog"
	"github.com/mitchellh/hashstructure/v2"
)

// SetOfChanIds is a set of channel IDs
type SetOfChanIds map[uint64]struct{}

// NodeData struct
type NodeData struct {
	ctx               context.Context
	perNodeSettings   *PerNodeSettings
	channelCache      ChannelCache
	nodeChanIds       map[string]SetOfChanIds
	nodeChanIdsNew    map[string]SetOfChanIds
	locallyDisabled   SetOfChanIds
	remotelyDisabled  SetOfChanIds
	smooth            bool          // Should we smooth out fluctuations due to HTLCs
	keepAliveInterval time.Duration // Keepalive interval
	checkGraph        bool          // Should we check gossip
	monitoring        *Monitoring
	eventLoopInterval time.Duration
	reentrancyBlock   *entities.ReentrancyBlock
}

// NewDefaultNodeData constructs a new NodeData
func NewDefaultNodeData(ctx context.Context, keepAlive time.Duration, smooth bool, checkGraph bool, monitoring *Monitoring) *NodeData {
	return NewNodeData(ctx, NewInMemoryChannelCache(), keepAlive, smooth, checkGraph, monitoring)
}

// NewNodeData constructs a new NodeData
func NewNodeData(ctx context.Context, cache ChannelCache, keepAlive time.Duration, smooth bool, checkGraph bool, monitoring *Monitoring) *NodeData {
	if ctx == nil {
		ctx = getContext()
	}

	if monitoring == nil {
		monitoring = NewNopNodeDataMonitoring("nodedata")
	}

	return &NodeData{
		ctx:               ctx,
		perNodeSettings:   NewPerNodeSettings(),
		channelCache:      cache,
		nodeChanIds:       make(map[string]SetOfChanIds),
		nodeChanIdsNew:    make(map[string]SetOfChanIds),
		locallyDisabled:   make(SetOfChanIds),
		remotelyDisabled:  make(SetOfChanIds),
		smooth:            smooth,
		checkGraph:        checkGraph,
		keepAliveInterval: keepAlive,
		monitoring:        monitoring,
		eventLoopInterval: 10 * time.Second,
		reentrancyBlock:   entities.NewReentrancyBlock(),
	}
}

// IsSubscribed - check if we are subscribed for a certain public key
func (c *NodeData) IsSubscribed(pubKey, uniqueID string) bool {
	return utils.Contains(c.perNodeSettings.GetKeys(), pubKey+uniqueID)
}

// Subscribe - subscribe to notifications about node changes (if already subscribed this will force a callback, use IsSubscribed to check)
func (c *NodeData) Subscribe(
	nodeDataCallback entities.NodeDataReportCallback,
	getAPI entities.NewAPICall,
	PollInterval entities.Interval,
	pubKey string,
	settings entities.ReportingSettings,
	uniqueID string) error {

	if pubKey != "" && !utils.ValidatePubkey(pubKey) {
		return errors.New("invalid pubkey")
	}

	api := getAPI()
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

	c.perNodeSettings.Set(info.IdentityPubkey+uniqueID, Settings{
		nodeDataCallback: nodeDataCallback,
		hash:             0,
		identifier:       entities.NodeIdentifier{Identifier: pubKey, UniqueID: uniqueID},
		lastGraphCheck:   time.Time{},
		lastReport:       time.Time{},
		settings:         settings,
		getAPI:           getAPI,
	})

	return nil
}

// Unsubscribe - unsubscribe from a pubkey
func (c *NodeData) Unsubscribe(pubkey, uniqueID string) error {
	c.perNodeSettings.Delete(pubkey + uniqueID)
	return nil
}

// GetState - get current state (settings.pollInterval is ignored)
func (c *NodeData) GetState(
	pubKey string,
	uniqueID string,
	getAPI entities.NewAPICall,
	settings entities.ReportingSettings,
	optCallback entities.NodeDataReportCallback) (*entities.NodeDataReport, error) {

	if pubKey != "" && !utils.ValidatePubkey(pubKey) {
		return nil, errors.New("invalid pubkey")
	}

	resp, err := c.checkOne(entities.NodeIdentifier{Identifier: pubKey, UniqueID: uniqueID}, getAPI, settings, true, false)
	if err != nil {
		return nil, err
	}

	resp.ChannelReport.PollInterval = entities.ManualRequest
	if optCallback != nil {
		optCallback(c.ctx, resp)
	}
	return resp, err
}

func (c *NodeData) getChannelList(
	api api.LightingAPICalls,
	info *api.InfoAPI,
	precisionBits int,
	allowPrivateChans bool) ([]entities.ChannelBalance, SetOfChanIds, error) {

	defer c.monitoring.MetricsTimer("channellist", map[string]string{"pubkey": info.IdentityPubkey})()

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
		chanIds[channel.ChanID] = struct{}{}

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

		total := uint64(channel.Capacity)

		factor := float64(1)
		if total > precision {
			factor = float64(total) / float64(precision)
		}

		_, locallyDisabled := c.locallyDisabled[channel.ChanID]
		_, remotelyDisabled := c.remotelyDisabled[channel.ChanID]

		resp = append(resp, entities.ChannelBalance{
			Active:          channel.Active,
			Private:         channel.Private,
			LocalPubkey:     info.IdentityPubkey,
			RemotePubkey:    channel.RemotePubkey,
			ChanID:          channel.ChanID,
			Capacity:        capacity,
			RemoteNominator: uint64(math.Round(float64(remoteBalance) / factor)),
			LocalNominator:  uint64(math.Round(float64(localBalance) / factor)),
			Denominator:     uint64(math.Round(float64(total) / factor)),

			ActiveRemote: !remotelyDisabled,
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

// OverrideLoopInterval - WARNING: this should not be used except for unit testing
func (c *NodeData) OverrideLoopInterval(duration time.Duration) {
	c.eventLoopInterval = duration
}

// EventLoop - invoke the event loop
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
	getAPI entities.NewAPICall,
	settings entities.ReportingSettings,
) error {

	api := getAPI()
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
			locallyDisabled[channel.ChannelID] = struct{}{}
		}

		if remotePolicy != nil && remotePolicy.Disabled {
			remotlyDisabled[channel.ChannelID] = struct{}{}
		}
	}

	c.locallyDisabled = locallyDisabled
	c.remotelyDisabled = remotlyDisabled

	return nil
}

func (c *NodeData) checkAll() bool {
	defer c.monitoring.MetricsTimer("checkall.global", nil)()

	for _, one := range c.perNodeSettings.GetKeys() {
		s := c.perNodeSettings.Get(one)
		now := time.Now()
		if c.checkGraph && s.settings.GraphPollInterval != 0 {
			graphToBeCheckedBy := s.lastGraphCheck.Add(s.settings.GraphPollInterval)

			if graphToBeCheckedBy.Before(now) && s.identifier.Identifier != "" {
				// Beware: here the graph is fetched just per node
				err := c.fetchGraph(s.identifier.Identifier, s.getAPI, s.settings)
				if err != nil {
					glog.Warningf("Could not fetch graph from %s: %v", s.identifier.Identifier, err)
				}

				s.lastGraphCheck = time.Now()
			}
		}

		toBeCheckedBy := s.lastReport.Add(s.settings.PollInterval.Duration())
		reportAnyway := false

		if s.settings.NoopInterval != 0 {
			reportAnyway = s.lastReport.Add(s.settings.NoopInterval).Before(now)
		}

		if toBeCheckedBy.Before(now) {

			// Subscribe will set lastCheck to min value and you expect update in such a case
			ignoreCache := toBeCheckedBy.Year() <= 1
			resp, err := c.checkOne(s.identifier, s.getAPI, s.settings, ignoreCache, reportAnyway)
			if err != nil {
				glog.Warningf("Failed to check %v: %v", s.identifier.GetID(), err)
				continue
			}

			if resp.ChannelReport != nil && s.identifier.Identifier != "" && resp.ChannelReport.PubKey != "" && !strings.EqualFold(resp.ChannelReport.PubKey, s.identifier.Identifier) {
				sentry.CaptureMessage(fmt.Sprintf("PubKey mismatch %s vs %s", resp.ChannelReport.PubKey, s.identifier.Identifier))
				glog.Warningf("PubKey mismatch %s vs %s", resp.ChannelReport.PubKey, s.identifier.Identifier)
				continue
			}

			if resp != nil {
				go func(c *NodeData, resp *entities.NodeDataReport, s Settings, one string) {
					if !c.reentrancyBlock.Enter(one) {
						glog.Warningf("Reentrancy of node callback for %s not allowed", one)
						return
					}
					defer c.reentrancyBlock.Release(one)

					timer := c.monitoring.MetricsTimer("checkdelivery", map[string]string{"pubkey": s.identifier.GetID()})
					if s.nodeDataCallback(c.ctx, resp) {
						c.commitAllChanges(one, time.Now(), s)
					} else {
						c.revertAllChanges()
					}
					timer()
				}(c, resp, s, one)
			} else {
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

	c.monitoring.MetricsReport("checkall.global", "success", nil)
	return true
}

// checkOne checks one specific node
func (c *NodeData) checkOne(
	identifier entities.NodeIdentifier,
	getAPI entities.NewAPICall,
	settings entities.ReportingSettings,
	ignoreCache bool,
	reportAnyway bool) (*entities.NodeDataReport, error) {
	s := c.perNodeSettings.Get(identifier.GetID())

	pubkey := identifier.GetID()
	if pubkey == "" {
		pubkey = "local"
	}

	defer c.monitoring.MetricsTimer("checkone", map[string]string{"pubkey": pubkey})()

	if getAPI == nil {
		c.monitoring.MetricsReport("checkone", "failure", map[string]string{"pubkey": pubkey})
		return nil, fmt.Errorf("failed to get client - getAPI was nil")
	}

	api := getAPI()
	if api == nil {
		c.monitoring.MetricsReport("checkone", "failure", map[string]string{"pubkey": pubkey})
		return nil, fmt.Errorf("failed to get client - getAPI returned nil")
	}
	defer api.Cleanup()

	// Get channel info
	info, err := api.GetInfo(c.ctx)
	if err != nil {
		c.monitoring.MetricsReport("checkone", "failure", map[string]string{"pubkey": pubkey})
		return nil, fmt.Errorf("failed to get info: %v", err)
	}

	if identifier.Identifier != "" && !strings.EqualFold(info.IdentityPubkey, identifier.Identifier) {
		c.monitoring.MetricsReport("checkone", "failure", map[string]string{"pubkey": pubkey})
		return nil, fmt.Errorf("pubkey and reported pubkey are not the same")
	}

	identifier.Identifier = info.IdentityPubkey

	channelList, set, err := c.getChannelList(api, info, settings.AllowedEntropy, settings.AllowPrivateChannels)
	if err != nil {
		c.monitoring.MetricsReport("checkone", "failure", map[string]string{"pubkey": pubkey})
		return nil, err
	}

	closedChannels := make([]entities.ClosedChannel, 0)

	if len(c.nodeChanIds[identifier.GetID()]) > 0 {
		// We must have some old channel set

		// diff between new -> old
		closed := utils.SetDiff(set, c.nodeChanIds[identifier.GetID()])
		for k := range closed {
			closedChannels = append(closedChannels, entities.ClosedChannel{ChannelID: k})
		}
	}

	c.nodeChanIdsNew[identifier.GetID()] = set

	channelList = c.filterList(identifier, channelList, ignoreCache)

	channelBalanceReport := &entities.ChannelBalanceReport{
		ReportingSettings: settings,
		Chain:             info.Chain,
		Network:           info.Network,
		PubKey:            identifier.Identifier,
		UniqueID:          identifier.UniqueID,
		Timestamp:         common_entities.JsonTime(time.Now()),
		ChangedChannels:   channelList,
		ClosedChannels:    closedChannels,
	}

	if len(channelList) <= 0 && len(closedChannels) <= 0 && !reportAnyway {
		channelBalanceReport = nil
	}

	// Get node info
	nodeInfo, err := api.GetNodeInfoFull(c.ctx, true, settings.AllowPrivateChannels)
	var nodeReport *entities.InfoReport
	if err != nil {
		fmt.Printf("failed to call GetNodeInfoFull %v", err)
		nodeReport = nil
	}

	if len(nodeInfo.Channels) != int(nodeInfo.NumChannels) {
		fmt.Printf("bad NodeInfo obtained %d channels vs. num_channels %d - %v", len(nodeInfo.Channels), nodeInfo.NumChannels, nodeInfo)
		nodeReport = nil
	}

	hash, err := hashstructure.Hash(nodeInfo, hashstructure.FormatV2, nil)
	if err != nil {
		glog.Warning("Hash could not be determined")
		hash = 1
	}

	if hash != s.hash {
		nodeReport = &entities.InfoReport{
			UniqueID:            identifier.UniqueID,
			Timestamp:           common_entities.JsonTime(time.Now()),
			NodeInfoAPIExtended: *nodeInfo,
		}
		s.hash = hash
	} else {
		nodeReport = nil
	}

	nodeData := &entities.NodeDataReport{
		NodeReport:    nodeReport,
		ChannelReport: channelBalanceReport,
	}

	c.monitoring.MetricsReport("checkone", "success", map[string]string{"pubkey": pubkey})
	return nodeData, nil
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
		id := fmt.Sprintf("%s-%d", identifier.GetID(), one.ChanID)
		val, ok := c.channelCache.Get(id)
		// Note that denominator changes based on channel reserve (might use capacity here)

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
	c.commitChanIDChanges()
	c.channelCache.DeferredCommit()
	s.lastReport = now
	c.perNodeSettings.Set(one, s)
}

func (c *NodeData) revertAllChanges() {
	c.revertChanIDChanges()
	c.channelCache.DeferredRevert()
}

func (c *NodeData) commitChanIDChanges() {
	for k, v := range c.nodeChanIdsNew {
		c.nodeChanIds[k] = v
	}

	for k := range c.nodeChanIdsNew {
		delete(c.nodeChanIdsNew, k)
	}
}

func (c *NodeData) revertChanIDChanges() {
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
