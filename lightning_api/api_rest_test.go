package lightning_api

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	utils "github.com/bolt-observer/go_common/utils"
	entities "github.com/bolt-observer/go_common/entities"
)

func TestGetInfo(t *testing.T) {
	pubKey := "030f7b46defcec976ed516de5e7841bdcb7a19bb388b679ec9dba4bb526e93efb0"
	cert := utils.ObtainCert("bolt.observer:443")
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"

	data := entities.Data{
		PubKey:            pubKey,
		MacaroonHex:       dummyMac,
		CertificateBase64: cert,
		Endpoint:          "bolt.observer:443",
	}

	api := NewApi(LND_REST, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	d, ok := api.(*LndRestLightningApi)
	if !ok {
		t.Fatalf("Should be LND_REST")
	}

	// Mock
	d.HttpApi.DoFunc = func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "v1/getinfo") {
			t.Fatalf("URL should contain v1/getinfo")
		}

		response := `
		{
			"identity_pubkey": "030f7b46defcec976ed516de5e7841bdcb7a19bb388b679ec9dba4bb526e93efb0",
			"alias": "alias",
			"chains": [ {"chain": "bitcoin", "network": "mainnet"}]
		}
		`

		r := ioutil.NopCloser(bytes.NewReader([]byte(response)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}

	result, err := api.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("GetInfo failed: %v", err)
	}

	if result.IdentityPubkey != pubKey || result.Alias != "alias" || result.Chain != "bitcoin" || result.Network != "mainnet" {
		t.Fatalf("GetInfo got wrong response: %v", result)
	}
}
