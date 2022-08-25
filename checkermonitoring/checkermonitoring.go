package checkermonitoring

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/marpaia/graphite-golang"
)

const PREFIX = "bolt.boltobserver"

type CheckerMonitoring struct {
	graphite *graphite.Graphite
	env      string
	name     string
}

func NewNopCheckerMonitoring(name string) *CheckerMonitoring {
	g := graphite.NewGraphiteNop("", 2003)
	g.DisableLog = true
	return &CheckerMonitoring{graphite: g, name: name}
}

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

func (c *CheckerMonitoring) MetricsTimer(name string) func() {
	// A simple way to time function execution ala
	// defer c.timer("checkall")()
	start := time.Now()
	return func() {
		duration := time.Since(start)
		g := c.graphite
		glog.V(2).Infof("Method %s took %d milliseconds\n", name, duration)
		g.SendMetrics([]graphite.Metric{
			graphite.NewMetric(fmt.Sprintf("%s.%s.%s.%s.duration", PREFIX, c.name, c.env, name), fmt.Sprintf("%d", duration.Milliseconds()), time.Now().Unix()),
			graphite.NewMetric(fmt.Sprintf("%s.%s.%s.%s.invocation", PREFIX, c.name, c.env, name), "1", time.Now().Unix()),
		})
	}
}

func (c *CheckerMonitoring) MetricsReport(name, val string) {
	g := c.graphite
	g.SendMetrics([]graphite.Metric{
		graphite.NewMetric(fmt.Sprintf("%s.%s.%s.%s.%s", PREFIX, c.name, c.env, name, val), "1", time.Now().Unix()),
	})
}
