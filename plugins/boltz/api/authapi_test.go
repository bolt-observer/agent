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
