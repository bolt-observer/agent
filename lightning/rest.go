package lightning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	utils "github.com/bolt-observer/go_common/utils"
)

// GetDoFunc = signature for Do function
type GetDoFunc func(req *http.Request) (*http.Response, error)

// HTTPAPI struct
type HTTPAPI struct {
	DoFunc GetDoFunc
	client *http.Client
}

// Do - invokes HTTP request
func (h *HTTPAPI) Do(req *http.Request) (*http.Response, error) {
	if h.DoFunc != nil {
		return h.DoFunc(req)
	} else if h.client != nil {
		return h.client.Do(req)
	}

	return nil, fmt.Errorf("no way to fulfill request")
}

// NewHTTPAPI returns a new HTTPAPI
func NewHTTPAPI() *HTTPAPI {
	return &HTTPAPI{client: nil, DoFunc: nil}
}

// SetTransport - sets HTTP transport
func (h *HTTPAPI) SetTransport(transport *http.Transport) {
	h.client = &http.Client{Transport: transport}
}

// HTTPGetInfo - invokes GetInfo method
func (h *HTTPAPI) HTTPGetInfo(ctx context.Context, req *http.Request) (*GetInfoResponseOverride, error) {
	var info GetInfoResponseOverride

	req = req.WithContext(ctx)

	u, err := url.Parse(fmt.Sprintf("%s/v1/getinfo", req.URL))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u
	req.Method = http.MethodGet

	err = h.doRequest(req, &info)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

// HTTPGetChannels - invokes GetChannels method
func (h *HTTPAPI) HTTPGetChannels(ctx context.Context, req *http.Request) (*Channels, error) {
	var channels Channels

	req = req.WithContext(ctx)

	u, err := url.Parse(fmt.Sprintf("%s/v1/channels", req.URL))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u
	req.Method = http.MethodGet

	err = h.doRequest(req, &channels)
	if err != nil {
		return nil, err
	}

	return &channels, nil
}

// HTTPGetGraph - invokes GetGraph method
func (h *HTTPAPI) HTTPGetGraph(ctx context.Context, req *http.Request, unannounced bool) (*Graph, error) {
	var graph Graph

	req = req.WithContext(ctx)

	u, err := url.Parse(fmt.Sprintf("%s/v1/graph?include_unannounced=%s", req.URL, strconv.FormatBool(unannounced)))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u
	req.Method = http.MethodGet

	err = h.doRequest(req, &graph)
	if err != nil {
		return nil, err
	}

	return &graph, nil
}

// HTTPGetNodeInfo - invokes GetNodeInfo method
func (h *HTTPAPI) HTTPGetNodeInfo(ctx context.Context, req *http.Request, pubKey string, channels bool) (*GetNodeInfoOverride, error) {
	var nodeinfo GetNodeInfoOverride

	req = req.WithContext(ctx)

	u, err := url.Parse(fmt.Sprintf("%s/v1/graph/node/%s?include_channels=%s", req.URL, pubKey, strconv.FormatBool(channels)))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u
	req.Method = http.MethodGet

	err = h.doRequest(req, &nodeinfo)
	if err != nil {
		return nil, err
	}

	return &nodeinfo, nil
}

// HTTPGetChanInfo - invokes GetChanInfo method
func (h *HTTPAPI) HTTPGetChanInfo(ctx context.Context, req *http.Request, chanID uint64) (*GraphEdgeOverride, error) {
	var chaninfo GraphEdgeOverride

	req = req.WithContext(ctx)

	u, err := url.Parse(fmt.Sprintf("%s/v1/graph/edge/%d", req.URL, chanID))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u
	req.Method = http.MethodGet

	err = h.doRequest(req, &chaninfo)
	if err != nil {
		return nil, err
	}

	return &chaninfo, nil
}

// HTTPForwardEvents - invokes ForwardEvents method
func (h *HTTPAPI) HTTPForwardEvents(ctx context.Context, req *http.Request, input *ForwardingHistoryRequestOverride) (*ForwardingHistoryResponseOverride, error) {
	var data ForwardingHistoryResponseOverride

	req = req.WithContext(ctx)
	req.Method = http.MethodPost

	u, err := url.Parse(fmt.Sprintf("%s/v1/switch", req.URL))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	s, _ := json.Marshal(input)
	b := bytes.NewBuffer(s)

	req.URL = u
	req.Body = io.NopCloser(b)

	resp, err := h.Do(req)

	if err != nil {
		return nil, fmt.Errorf("http request failed %s", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http got error %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)

	err = decoder.Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("got error %v", err)
	}

	return &data, nil
}

// HTTPSubscribeHtlcEvents - invokes SubscribeHtlcEvents method
func (h *HTTPAPI) HTTPSubscribeHtlcEvents(ctx context.Context, req *http.Request) (<-chan *HtlcEventOverride, error) {

	req = req.WithContext(ctx)

	u, err := url.Parse(fmt.Sprintf("%s/v2/router/htlcevents", req.URL))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u
	req.Method = http.MethodGet

	resp, err := h.Do(req)

	if err != nil {
		return nil, fmt.Errorf("http request failed %s", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http got error %d", resp.StatusCode)
	}

	outchan := make(chan *HtlcEventOverride)

	go func() {
		var data HtlcEventOverride
		defer resp.Body.Close()
		decoder := json.NewDecoder(resp.Body)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Do nothing
			}

			err := decoder.Decode(&data)

			if err != nil {
				return
			}

			outchan <- &data
		}
	}()

	return outchan, nil
}

// HTTPListInvoices - invokes ListInvoices method
func (h *HTTPAPI) HTTPListInvoices(ctx context.Context, req *http.Request, input *ListInvoiceRequestOverride) (*ListInvoiceResponseOverride, error) {
	var data ListInvoiceResponseOverride

	req = req.WithContext(ctx)

	u, err := url.Parse(fmt.Sprintf("%s/v1/invoices?pending_only=%v&index_offset=%s&num_max_invoices=%s&reversed=%v", req.URL, input.PendingOnly, input.IndexOffset,
		input.NumMaxInvoices, input.Reversed))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u
	req.Method = http.MethodGet

	err = h.doRequest(req, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

// HTTPListPayments - invokes ListPayments method
func (h *HTTPAPI) HTTPListPayments(ctx context.Context, req *http.Request, input *ListPaymentsRequestOverride) (*ListPaymentsResponseOverride, error) {
	var data ListPaymentsResponseOverride

	req = req.WithContext(ctx)

	u, err := url.Parse(fmt.Sprintf("%s/v1/payments?include_incomplete=%v&index_offset=%s&max_payments=%s&reversed=%v", req.URL, input.IncludeIncomplete, input.IndexOffset,
		input.MaxPayments, input.Reversed))
	if err != nil {
		return nil, fmt.Errorf("invalid url %s", err)
	}

	req.URL = u
	req.Method = http.MethodGet

	err = h.doRequest(req, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

// GetHTTPRequest - generic method for doing HTTP requests
func (h *HTTPAPI) GetHTTPRequest(getData GetDataCall) (*http.Request, *http.Transport, error) {

	if getData == nil {
		return nil, nil, fmt.Errorf("getData is nil")
	}

	data, err := getData()

	if err != nil {
		return nil, nil, err
	}

	myurl := data.Endpoint
	if !strings.HasPrefix(data.Endpoint, "http") {
		myurl = fmt.Sprintf("https://%s", data.Endpoint)
	}

	req, err := http.NewRequest(http.MethodGet, myurl, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("new request %s", err.Error())
	}
	req.Header.Set("Grpc-Metadata-macaroon", data.MacaroonHex)

	certBytes, err := utils.SafeBase64Decode(data.CertificateBase64)
	if err != nil {
		return nil, nil, fmt.Errorf("base64 decode error %s", err)
	}

	verification := PublicCAorCert
	if data.CertVerificationType != nil {
		verification = CertificateVerification(*data.CertVerificationType)
	}

	tls, err := getTLSConfig(certBytes, data.Endpoint, verification)
	if err != nil {
		return nil, nil, fmt.Errorf("getTlsConfig failed %v", err)
	}

	trans := &http.Transport{
		TLSClientConfig: tls,
	}

	return req, trans, nil
}

func (h *HTTPAPI) doRequest(req *http.Request, data any) error {
	resp, err := h.Do(req)

	if err != nil {
		return fmt.Errorf("http request failed %s", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http got error %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)

	err = decoder.Decode(&data)
	if err != nil {
		return fmt.Errorf("got error %v", err)
	}

	return nil
}
