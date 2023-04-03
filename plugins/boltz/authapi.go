//go:build plugins
// +build plugins

package boltz

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const Timeout = 5 * time.Second

type Credentials struct {
	Key    string
	Secret string
}

func calcHmac(key string, message string) string {
	sig := hmac.New(sha256.New, []byte(key))
	sig.Write([]byte(message))

	return strings.ToUpper(hex.EncodeToString(sig.Sum(nil)))
}

type BoltzPrivateAPI struct {
	URL   string
	Creds *Credentials
}

func NewBoltzPrivateAPIFake() *BoltzPrivateAPI {
	return &BoltzPrivateAPI{URL: ""}
}

func NewBoltzPrivateAPI(url string, creds *Credentials) *BoltzPrivateAPI {
	return &BoltzPrivateAPI{URL: url, Creds: creds}
}

func (b *BoltzPrivateAPI) SendGetRequest(endpoint string, response interface{}) error {
	if b.URL == "" {
		return fmt.Errorf("Disabled Boltz API endpoint")
	}
	req, err := http.NewRequest(http.MethodGet, b.URL+endpoint, nil)
	if err != nil {
		return err
	}
	unixTime := strconv.Itoa(int(time.Now().Unix()))

	req.Header.Set("TS", unixTime)
	req.Header.Set("API-KEY", b.Creds.Key)

	signature := calcHmac(b.Creds.Secret, unixTime+"GET"+endpoint)

	req.Header.Set("API-HMAC", signature)

	client := &http.Client{Timeout: Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	return unmarshalJson(resp.Body, &response)
}

func (b *BoltzPrivateAPI) SendPostRequest(endpoint string, requestBody interface{}, response interface{}) error {
	if b.URL == "" {
		return fmt.Errorf("Disabled Boltz API endpoint")
	}

	rawBody, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	unixTime := strconv.Itoa(int(time.Now().Unix()))

	req, err := http.NewRequest(http.MethodPost, b.URL+endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return err
	}
	req.Header.Set("TS", unixTime)
	req.Header.Set("API-KEY", b.Creds.Key)

	signature := calcHmac(b.Creds.Secret, unixTime+"GET"+endpoint+string(rawBody))

	req.Header.Set("API-HMAC", signature)

	client := &http.Client{Timeout: Timeout}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	return unmarshalJson(resp.Body, &response)
}

func unmarshalJson(body io.ReadCloser, response interface{}) error {
	rawBody, err := io.ReadAll(body)

	if err != nil {
		return err
	}

	return json.Unmarshal(rawBody, &response)
}
