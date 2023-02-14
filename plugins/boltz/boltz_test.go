// //go:build plugins

package boltz

import "testing"

func TestBurek(t *testing.T) {
	Burek()
	t.Fail()
}
