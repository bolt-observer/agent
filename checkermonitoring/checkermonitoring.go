package checkermonitoring

import (
	"fmt"
	"strconv"
	"time"

	"github.com/bolt-observer/graphite-golang"
	"github.com/golang/glog"
)

// PREFIX is the prefix for all metrics.
const PREFIX = "bolt.boltobserver"

// CheckerMonitoring struct.
type CheckerMonitoring struct {
	graphite *graphite.Graphite
	env      string
	name     string
}

// NewNopCheckerMonitoring constructs a new CheckerMonitoring that does nothing.
func NewNopCheckerMonitoring(name string) *CheckerMonitoring {
	g := graphite.NewGraphiteNop("", 2003)
	g.DisableLog = true
	return &CheckerMonitoring{graphite: g, name: name}
}

// NewCheckerMonitoring constructs a new CheckerMonitoring instance.
func NewCheckerMonitoring(name, env, graphiteHost, graphitePort string) *CheckerMonitoring {
	port, err := strconv.Atoi(graphitePort)
	if err != nil {
		port = 0
	}

	if graphiteHost == "" {
		g := graphite.NewGraphiteNop(graphiteHost, port)
		g.DisableLog = true
		return &CheckerMonitoring{graphite: g}
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

	return &CheckerMonitoring{graphite: g, env: env, name: name}
}

// MetricsTimer - is used to time function executions.
func (c *CheckerMonitoring) MetricsTimer(name string, tags map[string]string) func() {
	// A simple way to time function execution ala
	// defer c.timer("checkall")()

	if tags == nil {
		tags = make(map[string]string)
	}

	tags["env"] = c.env

	start := time.Now()
	return func() {
		duration := time.Since(start)
		g := c.graphite
		glog.V(2).Infof("Method %s took %d milliseconds\n", name, duration.Milliseconds())
		g.SendMetrics([]graphite.Metric{
			graphite.NewMetricWithTags(fmt.Sprintf("%s.%s.%s.duration", PREFIX, c.name, name), fmt.Sprintf("%d", duration.Milliseconds()), time.Now().Unix(), tags),
			graphite.NewMetricWithTags(fmt.Sprintf("%s.%s.%s.invocation", PREFIX, c.name, name), "1", time.Now().Unix(), tags),
		})
	}
}

// MetricsReport - is used to report a metric.
func (c *CheckerMonitoring) MetricsReport(name, val string, tags map[string]string) {
	g := c.graphite

	if tags == nil {
		tags = make(map[string]string)
	}

	tags["env"] = c.env

	g.SendMetrics([]graphite.Metric{
		graphite.NewMetricWithTags(fmt.Sprintf("%s.%s.%s.%s", PREFIX, c.name, name, val), "1", time.Now().Unix(), tags),
	})
}
