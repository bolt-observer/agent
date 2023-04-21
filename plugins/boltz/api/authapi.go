//go:build plugins
// +build plugins

package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
)

// This is a wrapper around boltz.Boltz API

const (
	Timeout = 5 * time.Second
)

type Credentials struct {
	Key    string
	Secret string
}

type BoltzPrivateAPI struct {
	Creds *Credentials
	boltz.Boltz
}

type CreateSwapRequestChannel struct {
	Auto             bool `json:"auto"`
	Private          bool `json:"private"`
	InboundLiquidity int  `json:"inboundLiquidity"` // 10 - 50 %
}

type CreateSwapRequestOverride struct {
	ReferralId string `json:"referralId,omitempty"`
	//Channel    CreateSwapRequestChannel `json:"channel"`
	boltz.CreateSwapRequest
}

type CreateReverseSwapRequestOverride struct {
	ReferralId string `json:"referralId,omitempty"`
	boltz.CreateReverseSwapRequest
}

type GetTransactionRequest struct {
	Currency      string `json:"currency"`
	TransactionId string `json:"transactionId"`
}

type GetTransactionResponse struct {
	TransactionHex string `json:"transactionHex"`
}

type Month map[string]Token
type Token map[string]Currency
type Currency map[string]int
type ReferralsResponse map[string]Month

func NewBoltzPrivateAPI(url string, creds *Credentials) *BoltzPrivateAPI {
	const Btc = "BTC"
	ret := &BoltzPrivateAPI{Creds: creds}

	ret.Boltz = boltz.Boltz{
		URL: url,
	}

	ret.Boltz.Init(Btc) // required

	return ret
}

func (b *BoltzPrivateAPI) CreateSwap(request CreateSwapRequestOverride) (*boltz.CreateSwapResponse, error) {
	var response boltz.CreateSwapResponse
	err := b.sendPostRequest("/createswap", request, &response)

	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return &response, err
}

func (b *BoltzPrivateAPI) CreateReverseSwap(request CreateReverseSwapRequestOverride) (*boltz.CreateReverseSwapResponse, error) {
	var response boltz.CreateReverseSwapResponse
	err := b.sendPostRequest("/createswap", request, &response)

	if response.Error != "" {
		return nil, errors.New(response.Error)
	}

	return &response, err
}

func (b *BoltzPrivateAPI) GetTransaction(request GetTransactionRequest) (*GetTransactionResponse, error) {
	var response GetTransactionResponse
	err := b.sendPostRequest("/gettransaction", request, &response)

	if err != nil {
		return nil, err
	}

	return &response, err
}

func (b *BoltzPrivateAPI) QueryReferrals() (*ReferralsResponse, error) {
	var response ReferralsResponse
	err := b.sendGetRequestAuth("/referrals/query", &response)

	if err != nil {
		return nil, err
	}

	return &response, err
}

func (b *BoltzPrivateAPI) sendGetRequestAuth(endpoint string, response interface{}) error {
	if b.Creds == nil {
		return fmt.Errorf("no credentials")
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

func (b *BoltzPrivateAPI) sendPostRequestAuth(endpoint string, requestBody interface{}, response interface{}) error {
	if b.Creds == nil {
		return fmt.Errorf("no credentials")
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

func (b *BoltzPrivateAPI) sendGetRequest(endpoint string, response interface{}) error {
	res, err := http.Get(b.URL + endpoint)

	if err != nil {
		return err
	}

	return unmarshalJson(res.Body, &response)
}

func (b *BoltzPrivateAPI) sendPostRequest(endpoint string, requestBody interface{}, response interface{}) error {
	rawBody, err := json.Marshal(requestBody)

	if err != nil {
		return err
	}

	res, err := http.Post(b.URL+endpoint, "application/json", bytes.NewBuffer(rawBody))

	if err != nil {
		return err
	}

	return unmarshalJson(res.Body, &response)
}

func unmarshalJson(body io.ReadCloser, response interface{}) error {
	rawBody, err := io.ReadAll(body)

	if err != nil {
		return err
	}

	return json.Unmarshal(rawBody, &response)
}

func calcHmac(key string, message string) string {
	sig := hmac.New(sha256.New, []byte(key))
	sig.Write([]byte(message))

	return hex.EncodeToString(sig.Sum(nil))
}
