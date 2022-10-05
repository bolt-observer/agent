package lightning_api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type GetDoFunc func(req *http.Request) (*http.Response, error)

type HttpApi struct {
	DoFunc GetDoFunc
	client *http.Client
}

func (h *HttpApi) Do(req *http.Request) (*http.Response, error) {
	if h.DoFunc != nil {
		return h.DoFunc(req)
	} else if h.client != nil {
		return h.client.Do(req)
	}

	return nil, fmt.Errorf("no way to fulfill request")
}

func NewHttpApi() *HttpApi {
	return &HttpApi{client: nil, DoFunc: nil}
}

func (h *HttpApi) SetTransport(transport *http.Transport) {
	h.client = &http.Client{Transport: transport}
}

func (h *HttpApi) HttpGetInfo(ctx context.Context, req *http.Request, trans *http.Transport) (*GetInfoResponseOverride, error) {
	var info GetInfoResponseOverride

	req = req.WithContext(ctx)

	req.Method = http.MethodGet

	u, err := url.Parse(fmt.Sprintf("%s/v1/getinfo", req.URL))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u

	resp, err := h.Do(req)

	if err != nil {
		return nil, fmt.Errorf("http request failed %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http got error %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	err = decoder.Decode(&info)
	if err != nil {
		return nil, fmt.Errorf("decode error %v", err)
	}

	return &info, nil
}

func (h *HttpApi) HttpGetChannels(ctx context.Context, req *http.Request, trans *http.Transport) (*Channels, error) {
	var info Channels

	req = req.WithContext(ctx)

	req.Method = http.MethodGet

	u, err := url.Parse(fmt.Sprintf("%s/v1/channels", req.URL))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u

	resp, err := h.Do(req)

	if err != nil {
		return nil, fmt.Errorf("http request failed %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http got error %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	err = decoder.Decode(&info)
	if err != nil {
		return nil, fmt.Errorf("got error %v", err)
	}

	return &info, nil
}

func (h *HttpApi) HttpGetGraph(ctx context.Context, req *http.Request, trans *http.Transport, unannounced bool) (*Graph, error) {
	var graph Graph

	req = req.WithContext(ctx)

	req.Method = http.MethodGet

	u, err := url.Parse(fmt.Sprintf("%s/v1/graph?include_unannounced=%s", req.URL, strconv.FormatBool(unannounced)))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u

	resp, err := h.Do(req)

	if err != nil {
		return nil, fmt.Errorf("http request failed %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http got error %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	err = decoder.Decode(&graph)
	if err != nil {
		return nil, fmt.Errorf("got error %v", err)
	}

	return &graph, nil
}

func (h *HttpApi) HttpGetNodeInfo(ctx context.Context, req *http.Request, trans *http.Transport, pubKey string, channels bool) (*GetNodeInfoOverride, error) {
	var graph GetNodeInfoOverride

	req = req.WithContext(ctx)

	req.Method = http.MethodGet

	u, err := url.Parse(fmt.Sprintf("%s/v1/graph/node/%s?include_channels=%s", req.URL, pubKey, strconv.FormatBool(channels)))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u

	resp, err := h.Do(req)

	if err != nil {
		return nil, fmt.Errorf("http request failed %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http got error %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	err = decoder.Decode(&graph)
	if err != nil {
		return nil, fmt.Errorf("got error %v", err)
	}

	return &graph, nil
}

func (h *HttpApi) HttpGetChanInfo(ctx context.Context, req *http.Request, trans *http.Transport, chanId uint64) (*GraphEdgeOverride, error) {
	var graph GraphEdgeOverride

	req = req.WithContext(ctx)

	req.Method = http.MethodGet

	u, err := url.Parse(fmt.Sprintf("%s/v1/graph/edge/%d", req.URL, chanId))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u

	resp, err := h.Do(req)

	if err != nil {
		return nil, fmt.Errorf("http request failed %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http got error %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	err = decoder.Decode(&graph)
	if err != nil {
		return nil, fmt.Errorf("got error %v", err)
	}

	return &graph, nil
}

func (h *HttpApi) GetHttpRequest(getData GetDataCall) (*http.Request, *http.Transport, error) {

	if getData == nil {
		return nil, nil, fmt.Errorf("getData is nil")
	}

	data, err := getData()

	if err != nil {
		return nil, nil, err
	}

	myurl := data.Endpoint
	if !strings.HasPrefix(data.Endpoint, "https") {
		myurl = fmt.Sprintf("https://%s", data.Endpoint)
	}

	req, err := http.NewRequest(http.MethodGet, myurl, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("new request %s", err.Error())
	}
	req.Header.Set("Grpc-Metadata-macaroon", data.MacaroonHex)

	certBytes, err := base64.StdEncoding.DecodeString(data.CertificateBase64)
	if err != nil {
		return nil, nil, fmt.Errorf("base64 decode error %s", err)
	}

	// TODO: verification mode will come from data
	tls, err := getTlsConfig(certBytes, data.Endpoint, ALLOW_WHEN_PUBKEY_SAME)
	if err != nil {
		return nil, nil, fmt.Errorf("getTlsConfig failed %v", err)
	}

	trans := &http.Transport{
		TLSClientConfig: tls,
	}

	return req, trans, nil
}
