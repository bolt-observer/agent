package nodedata

import (
	"testing"
)

func TestBasicMonitoring(t *testing.T) {

	c := NewNodeDataMonitoring("a", "b", "127.0.0.1", "9000")

	c.MetricsTimer("a", nil)
	c.MetricsReport("b", "c", nil)

	c = NewNopNodeDataMonitoring("c")

	if !c.graphite.IsNop() {
		t.Fatalf("Should be nop checker")
		return
	}

	c = NewNodeDataMonitoring("a", "b", "", "9000")
	if !c.graphite.IsNop() {
		t.Fatalf("Should be nop checker")
		return
	}

	defer c.MetricsTimer("a", nil)()
	c.MetricsReport("b", "c", nil)
	c.MetricsReport("d", "e", map[string]string{"foo": "bar", "baz": "ok"})
}
