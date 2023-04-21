//go:build plugins
// +build plugins

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bolt-observer/agent/plugins/boltz/common"
	crypto "github.com/bolt-observer/agent/plugins/boltz/crypto"
)

type Api struct {
	ApiKey    string `json:"apikey"`
	ApiSecret string `json:"apisecret"`
}

func TestQueryReferralsE2e(t *testing.T) {
	id := crypto.RandSeq(20)

	out, err := exec.Command("docker", "exec", "boltz-backend", "/opt/boltz-backend/bin/boltz-cli", "addreferral", id, "10").Output()
	if err != nil {
		// Assume e2e is not available
		t.Log("E2E is not available")
		return
	}

	var api Api
	err = json.Unmarshal(out, &api)
	require.NoError(t, err)
	itf := NewBoltzPrivateAPI("http://localhost:9001", &Credentials{
		Key:    api.ApiKey,
		Secret: api.ApiSecret,
	})

	_, err = itf.QueryReferrals()
	require.NoError(t, err)
}

func TestDeserialize(t *testing.T) {
	v := `{
		"2021": {
		  "9": {
			"cliTest": {
			  "BTC": 60
			}
		  }
		}
	  }`

	var resp ReferralsResponse

	err := json.Unmarshal([]byte(v), &resp)
	require.NoError(t, err)
	assert.Equal(t, 60, resp["2021"]["9"]["cliTest"]["BTC"])
}

func TestAuthHeadersSent(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const Delta = 1000

		key := r.Header["Api-Key"][0]
		assert.Equal(t, "key", key)
		ts := r.Header["Ts"][0]
		tsNum, err := strconv.ParseInt(ts, 10, 64)
		require.NoError(t, err)
		assert.Greater(t, time.Now().Unix()+Delta, tsNum)
		assert.Less(t, time.Now().Unix()-Delta, tsNum)

		assert.Equal(t, calcHmac("secret", ts+"GET/burek"), r.Header["Api-Hmac"][0])

		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))

	server.Start()
	defer server.Close()

	itf := NewBoltzPrivateAPI(server.URL, &Credentials{
		Key:    "key",
		Secret: "secret",
	})

	err := itf.sendGetRequestAuth("/burek", nil)
	require.NoError(t, err)
}

func TestNoCredentials(t *testing.T) {
	const NoCredentials = "no credentials"
	itf := NewBoltzPrivateAPI("http://test.not.existing", nil)
	err := itf.sendGetRequestAuth("/burek", nil)
	assert.Error(t, err)
	assert.Equal(t, NoCredentials, err.Error())

	err = itf.sendPostRequestAuth("/burek", nil, nil)
	assert.Error(t, err)
	assert.Equal(t, NoCredentials, err.Error())

	_, err = itf.QueryReferrals()
	assert.Error(t, err)
	assert.Equal(t, NoCredentials, err.Error())
}

func TestGetTransaction(t *testing.T) {
	return // do not do http requests to testnet from unit test
	id := "1a07ecae07ad96187017ad030fa7130519a6fbee2d8387173ec1737c830caef1"
	hex := "0100000000010114e6f93bec215f3ae47059e2ce5255fc41ad214469661ef659ddb8a3d39df9b50000000000ffffffff02cc96110000000000225120a8510fdaeafedcc7f29144590df9f953e102a410fc7c80c2d60fbe9aff34102a080801000000000017a914a9ada294de4419f1bc932f5b00ebd22f20d6fd498702483045022100ebbbfdfabac276b5f7244b93d6dbad4b788eb47105de7f1393e7ba877f34f30902200e2c127beedee620438a0ec413f71d0e99532b23a40c7f79d361457f05c45d8d01210207a677b40f09768d6db074283742ae050bee970e0fc72f62cd20a22b957871ad00000000"

	itf := NewBoltzPrivateAPI("https://testnet.boltz.exchange/api", nil)
	resp, err := itf.GetTransaction(GetTransactionRequest{
		Currency:      common.Btc,
		TransactionId: id,
	})

	require.NoError(t, err)
	assert.Equal(t, hex, resp.TransactionHex)
}
