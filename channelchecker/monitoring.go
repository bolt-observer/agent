package channelchecker

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/marpaia/graphite-golang"
)

const PREFIX = "bolt.boltobserver.channelchecker"

type ChannelCheckerMonitoring struct {
	graphite *graphite.Graphite
	env      string
}

func NewNopChannelCheckerMonitoring() *ChannelCheckerMonitoring {
	g := graphite.NewGraphiteNop("", 2003)
	g.DisableLog = true
	return &ChannelCheckerMonitoring{graphite: g}
}

func NewChannelCheckerMonitoring(env, graphiteHost, graphitePort string) *ChannelCheckerMonitoring {
	port, err := strconv.Atoi(graphitePort)
	if err != nil {
		port = 0
	}

	if graphiteHost == "" {
		g := graphite.NewGraphiteNop(graphiteHost, port)
		g.DisableLog = true
		return &ChannelCheckerMonitoring{graphite: g}
	}

	g, err := graphite.NewGraphiteUDP(graphiteHost, port)

	if err != nil {
		g = graphite.NewGraphiteNop(graphiteHost, port)
		g.DisableLog = false
	}

	err = g.Connect()

	if err != nil {
		g = graphite.NewGraphiteNop(graphiteHost, port)
		g.DisableLog = false
	}

	return &ChannelCheckerMonitoring{graphite: g, env: env}
}

func (c *ChannelChecker) metricsTimer(name string) func() {
	// A simple way to time function execution ala
	// defer c.timer("checkall")()
	start := time.Now()
	return func() {
		duration := time.Since(start)
		g := c.monitoring.graphite
		glog.V(2).Infof("Method %s took %d milliseconds\n", name, duration)
		g.SendMetrics([]graphite.Metric{
			graphite.NewMetric(fmt.Sprintf("%s.%s.%s.duration", PREFIX, c.monitoring.env, name), fmt.Sprintf("%d", duration.Milliseconds()), time.Now().Unix()),
			graphite.NewMetric(fmt.Sprintf("%s.%s.%s.invocation", PREFIX, c.monitoring.env, name), "1", time.Now().Unix()),
		})
	}
}

func (c *ChannelChecker) metricsReport(name, val string) {
	g := c.monitoring.graphite
	g.SendMetrics([]graphite.Metric{
		graphite.NewMetric(fmt.Sprintf("%s.%s.%s.%s", PREFIX, c.monitoring.env, name, val), "1", time.Now().Unix()),
	})
}
