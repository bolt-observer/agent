package lightning

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertArgs(t *testing.T) {
	assert.Equal(t, `["1337","1338"]`, convertArgs([]string{"1337", "1338"}))
	assert.Equal(t, `[1337,1338]`, convertArgs([]int{1337, 1338}))
	assert.Equal(t, `[]`, convertArgs([]int{}))
	assert.Equal(t, `[]`, convertArgs(nil))
}

func TestParseResp(t *testing.T) {
	msg := ""
	_, err := parseResp(strings.NewReader(msg))
	assert.Error(t, err)

	msg = `{"error":{"code":19536,"message":"Params must be object or array"}}`
	_, err = parseResp(strings.NewReader(msg))
	assert.Error(t, err)
	assert.Equal(t, "invalid response: Params must be object or array", err.Error())

	msg = `{"error":{"code":19537,"message":"Not authorized: Invalid rune"}}`
	_, err = parseResp(strings.NewReader(msg))
	assert.Error(t, err)
	assert.Equal(t, "invalid response: Not authorized: Invalid rune", err.Error())

	msg = `{"error":{"code":19537,"message":"Not authorized: Not derived from master"}}`
	_, err = parseResp(strings.NewReader(msg))
	assert.Error(t, err)
	assert.Equal(t, "invalid response: Not authorized: Not derived from master", err.Error())

	msg = `{"jsonrpc":"2.0","id":11,"result":{"outputs":[],"channels":[]}}`
	resp, err := parseResp(strings.NewReader(msg))
	assert.NoError(t, err)
	assert.Equal(t, `{"outputs":[],"channels":[]}`, string(resp.Result))

	msg = `{"jsonrpc":"2.0","id":7,"result":{"id":"123","alias":"alias","color":"ffffff","num_peers":1,"num_pending_channels":0,"num_active_channels":0,"num_inactive_channels":0,"address":[{"type":"ipv6","address":"3ffe::1337","port":9735}],"binding":[{"type":"ipv6","address":"::","port":9735},{"type":"ipv4","address":"0.0.0.0","port":9735}],"version":"v0.12.0","blockheight":774495,"network":"bitcoin","msatoshi_fees_collected":0,"fees_collected_msat":"0msat","lightning-dir":"/tmp/fun","our_features":{"init":"08a080282269a2","node":"88a080282269a2","channel":"","invoice":"02000020024100"}}}`
	resp, err = parseResp(strings.NewReader(msg))
	assert.NoError(t, err)
	var objmap map[string]json.RawMessage
	err = json.Unmarshal(resp.Result, &objmap)
	assert.NoError(t, err)
	alias := string(objmap["alias"])
	assert.Equal(t, `"alias"`, alias)
}
