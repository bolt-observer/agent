package nodedata

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/marpaia/graphite-golang"
)

const PREFIX = "bolt.boltobserver"

type NodeDataMonitoring struct {
	graphite *graphite.Graphite
	env      string
	name     string
}

func NewNopNodeDataMonitoring(name string) *NodeDataMonitoring {
	g := graphite.NewGraphiteNop("", 2003)
	g.DisableLog = true
	return &NodeDataMonitoring{graphite: g, name: name}
}

func NewNodeDataMonitoring(name, env, graphiteHost, graphitePort string) *NodeDataMonitoring {
	port, err := strconv.Atoi(graphitePort)
	if err != nil {
		port = 0
	}

	if graphiteHost == "" {
		g := graphite.NewGraphiteNop(graphiteHost, port)
		g.DisableLog = true
		return &NodeDataMonitoring{graphite: g}
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

	return &NodeDataMonitoring{graphite: g, env: env, name: name}
}

func (c *NodeDataMonitoring) MetricsTimer(name string) func() {
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

func (c *NodeDataMonitoring) MetricsReport(name, val string) {
	g := c.graphite
	g.SendMetrics([]graphite.Metric{
		graphite.NewMetric(fmt.Sprintf("%s.%s.%s.%s.%s", PREFIX, c.name, c.env, name, val), "1", time.Now().Unix()),
	})
}
