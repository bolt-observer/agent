package lightning

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"

	common_entities "github.com/bolt-observer/go_common/entities"
)

const (
	LnRegTestPathPrefix = "/tmp/lnregtest-data/dev_network/"
)

// NewAPICall is the signature of the function to get Lightning API
type NewAPICall func() (LightingAPICalls, error)

func GetLocalCln(t *testing.T, name string) NewAPICall {
	data := &common_entities.Data{}
	x := int(ClnSocket)
	data.Endpoint = fmt.Sprintf("%s/lnnodes/%s/regtest/lightning-rpc", LnRegTestPathPrefix, name)
	if _, err := os.Stat(data.Endpoint); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	data.ApiType = &x

	return func() (LightingAPICalls, error) {
		api, err := NewAPI(ClnSocket, func() (*common_entities.Data, error) {
			return data, nil
		})

		return api, err
	}
}

func GetLocalLndByName(t *testing.T, name string) NewAPICall {
	nodes := map[string]string{
		"A": "localhost:11009",
		"C": "localhost:11011",
		"D": "localhost:11012",
		"E": "localhost:11013",
		"F": "localhost:11014",
		"G": "localhost:11015",
	}

	return GetLocalLnd(t, name, nodes[name])
}

func GetLocalLnd(t *testing.T, name string, endpoint string) NewAPICall {
	data := &common_entities.Data{}
	x := int(LndGrpc)
	data.Endpoint = endpoint
	data.ApiType = &x

	content, err := os.ReadFile(fmt.Sprintf("%s/lnnodes/%s/tls.cert", LnRegTestPathPrefix, name))
	if err != nil {
		return nil
	}
	data.CertificateBase64 = base64.StdEncoding.EncodeToString(content)

	macBytes, err := os.ReadFile(fmt.Sprintf("%s/lnnodes/%s/data/chain/bitcoin/regtest/admin.macaroon", LnRegTestPathPrefix, name))
	if err != nil {
		return nil
	}
	data.MacaroonHex = hex.EncodeToString(macBytes)

	return func() (LightingAPICalls, error) {
		api, err := NewAPI(LndGrpc, func() (*common_entities.Data, error) {
			return data, nil
		})

		return api, err
	}
}

func GetTestnetLnd(t *testing.T) NewAPICall {
	name := "testnet.secret"
	data := &common_entities.Data{}

	if _, err := os.Stat(name); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("Error when opening file: %v", err)
		return nil
	}

	err = json.Unmarshal(content, &data)
	if err != nil {
		t.Fatalf("Error during Unmarshal(): %v", err)
		return nil
	}

	return func() (LightingAPICalls, error) {
		api, err := NewAPI(LndGrpc, func() (*common_entities.Data, error) {
			return data, nil
		})

		return api, err
	}
}

func RegtestMine(numBlocks int) error {
	_, err := exec.Command("bitcoin-cli", fmt.Sprintf("-datadir=%s/bitcoin", LnRegTestPathPrefix), "-generate", fmt.Sprintf("%d", numBlocks)).Output()
	return err
}
