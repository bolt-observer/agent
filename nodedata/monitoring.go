package nodedata

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/marpaia/graphite-golang"
)

// PREFIX is the prefix for all metrics
const PREFIX = "bolt.boltobserver"

// Monitoring struct
type Monitoring struct {
	graphite *graphite.Graphite
	env      string
	name     string
}

// NewNopNodeDataMonitoring constructs a new Monitoring that does nothing
func NewNopNodeDataMonitoring(name string) *Monitoring {
	g := graphite.NewGraphiteNop("", 2003)
	g.DisableLog = true
	return &Monitoring{graphite: g, name: name}
}

// NewNodeDataMonitoring constructs a new Monitoring instance
func NewNodeDataMonitoring(name, env, graphiteHost, graphitePort string) *Monitoring {
	port, err := strconv.Atoi(graphitePort)
	if err != nil {
		port = 0
	}

	if graphiteHost == "" {
		g := graphite.NewGraphiteNop(graphiteHost, port)
		g.DisableLog = true
		return &Monitoring{graphite: g}
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

	return &Monitoring{graphite: g, env: env, name: name}
}

// MetricsTimer - is used to time function executions
func (c *Monitoring) MetricsTimer(name string) func() {
	// A simple way to time function execution ala
	// defer c.timer("checkall")()
	start := time.Now()
	return func() {
		duration := time.Since(start)
		g := c.graphite
		glog.V(2).Infof("Method %s took %d milliseconds\n", name, duration.Milliseconds())
		g.SendMetrics([]graphite.Metric{
			graphite.NewMetric(fmt.Sprintf("%s.%s.%s.%s.duration", PREFIX, c.name, c.env, name), fmt.Sprintf("%d", duration.Milliseconds()), time.Now().Unix()),
			graphite.NewMetric(fmt.Sprintf("%s.%s.%s.%s.invocation", PREFIX, c.name, c.env, name), "1", time.Now().Unix()),
		})
	}
}

// MetricsReport - is used to report a metric
func (c *Monitoring) MetricsReport(name, val string) {
	g := c.graphite
	g.SendMetrics([]graphite.Metric{
		graphite.NewMetric(fmt.Sprintf("%s.%s.%s.%s.%s", PREFIX, c.name, c.env, name, val), "1", time.Now().Unix()),
	})
}
