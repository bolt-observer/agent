package lightning

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const BUFSIZE = 2048

func TestConvertChanId(t *testing.T) {
	clnID := "761764x816x0"
	lndID := uint64(837568375674634240)

	id, err := ToLndChanID(clnID)
	if err != nil {
		t.Fatalf("Error %v\n", err)
	}
	if id != lndID {
		t.Fatalf("Conversion failed %v vs. %v\n", id, lndID)
	}

	if FromLndChanID(lndID) != clnID {
		t.Fatal("Conversion failed")
	}

	_, err = ToLndChanID("foobar")
	if err == nil {
		t.Fatal("Should have failed")
	}
}

func TestConvertFeatures(t *testing.T) {
	features := ConvertFeatures("800000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000802000888252a1")

	if features == nil {
		t.Fatalf("Conversion failed")
	}

	if val, ok := features["2023"]; ok {
		if !val.IsKnown {
			t.Fatalf("IsKnown is false")
		}

		if val.IsRequired {
			t.Fatalf("Required is true")
		}

		return
	}

	t.Fatalf("Should not happen")
}

type Handler func(c net.Conn)
type CloseFunc func()

func ClnSocketServer(t *testing.T, handler Handler) (string, CloseFunc) {
	tempf, err := os.CreateTemp("", "tempsocket-")
	if err != nil {
		t.Fatalf("Could not create temporary file: %v", err)
	}

	// From here...
	if err := tempf.Close(); err != nil {
		t.Fatalf("Could not close temporary file: %v", err)
	}
	if err := os.RemoveAll(tempf.Name()); err != nil {
		t.Fatalf("Could not remove temporary file: %v", err)
	}
	name := tempf.Name()
	listener, err := net.Listen("unix", name)
	if err != nil {
		t.Fatalf("Could not listen on temporary file: %v", err)
	}
	// ... to here there is a possibility of race conditions

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				// Swallow the error, but this just means test will fail
				return
			}

			if handler != nil {
				go handler(conn)
			}
		}
	}()

	closer := func() {
		listener.Close()
		os.Remove(name)
	}

	return name, closer
}

func clnData(t *testing.T, name string) []byte {
	f, err := os.OpenFile(fmt.Sprintf("%s/%s.json.template", FixtureDir, name), os.O_RDONLY, 0644)
	if err != nil {
		t.Fatalf("Could not open file: %v", err)
	}
	defer f.Close()

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatalf("Could not read file: %v", err)
	}

	return contents
}

func clnCommon(t *testing.T, handler Handler) (*LndClnSocketLightningAPI, LightingAPICalls, CloseFunc) {
	name, closeFunc := ClnSocketServer(t, handler)

	api := NewClnSocketLightningAPIRaw("unix", name)
	if api == nil {
		t.Fatalf("API should not be nil")
	}

	d, ok := api.(*LndClnSocketLightningAPI)
	if !ok {
		t.Fatalf("Should be CLN_SOCKET")
	}

	return d, api, closeFunc
}

type RequestExtractor struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
}

func TestClnGetInfo(t *testing.T) {
	data := clnData(t, "cln_info")

	_, api, closer := clnCommon(t, func(c net.Conn) {
		req := RequestExtractor{}
		err := json.NewDecoder(c).Decode(&req)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		if strings.Contains(req.Method, "getinfo") {
			reply := fmt.Sprintf(string(data), req.ID)
			_, err = c.Write(([]byte)(reply))

			if err != nil {
				t.Fatalf("Could not write to socket: %v", err)
			}
		}

		err = c.Close()
		if err != nil {
			t.Fatalf("Could not close socket: %v", err)
		}
	})
	defer closer()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()

	resp, err := api.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo call failed: %v", err)
	}

	if resp.IdentityPubkey != "03d1c07e00297eae99263dcc01850ec7339bb4c87a1a3e841a195cbfdcdec7a219" || resp.Alias != "cln1" || resp.Chain != "mainnet" || resp.Network != "bitcoin" {
		t.Fatal("Wrong response")
	}

	if !strings.HasPrefix(resp.Version, "corelightning-") || !resp.IsSyncedToChain || !resp.IsSyncedToGraph {
		t.Fatalf("GetInfo got wrong response: %v", resp)
		return
	}
}

func TestClnGetChanInfo(t *testing.T) {
	data := clnData(t, "cln_listchans")

	_, api, closer := clnCommon(t, func(c net.Conn) {
		req := RequestExtractor{}
		err := json.NewDecoder(c).Decode(&req)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		if strings.Contains(req.Method, "listchannels") {
			reply := fmt.Sprintf(string(data), req.ID)
			_, err = c.Write(([]byte)(reply))

			if err != nil {
				t.Fatalf("Could not write to socket: %v", err)
			}
		}

		err = c.Close()
		if err != nil {
			t.Fatalf("Could not close socket: %v", err)
		}
	})
	defer closer()

	id, err := ToLndChanID("763291x473x0")
	if err != nil {
		t.Fatalf("Could not convert id %d", err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()

	resp, err := api.GetChanInfo(ctx, id)
	if err != nil {
		t.Fatalf("GetInfo call failed: %v", err)
	}

	if resp.ChannelID != 839247329907769344 || resp.Node1Pub != "020f63ca0fd5cbb11012727c035b7c087c2d014a26ed8ed5ed2115c783945a3fc7" || resp.Node2Pub != "03d1c07e00297eae99263dcc01850ec7339bb4c87a1a3e841a195cbfdcdec7a219" {
		t.Fatal("Wrong response")
	}
}

func TestClnGetNodeInfoFull(t *testing.T) {
	info := clnData(t, "cln_nodeinfo_info")
	funds := clnData(t, "cln_nodeinfo_funds")
	channels := clnData(t, "cln_nodeinfo_channels")

	_, api, closer := clnCommon(t, func(c net.Conn) {
		for {
			req := RequestExtractor{}
			err := json.NewDecoder(c).Decode(&req)
			if err != nil {
				t.Fatalf("Decode error: %v", err)
			}

			reply := ""

			if strings.Contains(req.Method, "getinfo") {
				reply = fmt.Sprintf(string(info), req.ID)
			} else if strings.Contains(req.Method, "listfunds") {
				reply = fmt.Sprintf(string(funds), req.ID)
			} else if strings.Contains(req.Method, "listchannels") {
				reply = fmt.Sprintf(string(channels), req.ID)
			}

			if reply == "" {
				t.Fatalf("Called unexpected method %s", req.Method)
				return
			}

			_, err = c.Write(([]byte)(reply))
			assert.NoError(t, err)
		}
	})
	defer closer()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()

	resp, err := api.GetNodeInfoFull(ctx, true, true)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resp.Channels))
	assert.Equal(t, "031f786dcbac09a9174522a17a1bd6dfa6d01638d1fe250c6d0927ca0fdce36d:0", resp.Channels[0].ChanPoint)
}

type RawMethodCall func(ctx context.Context, api LightingAPICalls) ([]RawMessage, error)

func rawCommon(t *testing.T, file string, method string, call RawMethodCall) {

	data := clnData(t, file)

	_, api, closer := clnCommon(t, func(c net.Conn) {
		req := RequestExtractor{}
		err := json.NewDecoder(c).Decode(&req)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		if strings.Contains(req.Method, method) {
			reply := fmt.Sprintf(string(data), req.ID)
			_, err = c.Write(([]byte)(reply))

			if err != nil {
				t.Fatalf("Could not write to socket: %v", err)
			}
		}

		err = c.Close()
		if err != nil {
			t.Fatalf("Could not close socket: %v", err)
		}
	})
	defer closer()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()
	resp, err := call(ctx, api)
	assert.NoError(t, err)
	assert.Equal(t, 10, len(resp))
	assert.Equal(t, 2023, resp[0].Timestamp.Year())
}

func TestClnGetForwardsRaw(t *testing.T) {
	p := RawPagination{}
	p.BatchSize = 50
	rawCommon(t, "cln_listforwards", "listforwards",
		RawMethodCall(func(ctx context.Context, api LightingAPICalls) ([]RawMessage, error) {
			resp, _, err := api.GetForwardsRaw(ctx, p)
			return resp, err
		}),
	)
}

func TestClnGetInvoicesRaw(t *testing.T) {
	p := RawPagination{}
	p.BatchSize = 50
	rawCommon(t, "cln_listinvoices", "listinvoices",
		RawMethodCall(func(ctx context.Context, api LightingAPICalls) ([]RawMessage, error) {
			resp, _, err := api.GetInvoicesRaw(ctx, false, p)
			return resp, err
		}),
	)
}

func TestClnGetPaymentsRaw(t *testing.T) {
	p := RawPagination{}
	p.BatchSize = 50
	rawCommon(t, "cln_listsendpays", "listsendpays",
		RawMethodCall(func(ctx context.Context, api LightingAPICalls) ([]RawMessage, error) {
			resp, _, err := api.GetPaymentsRaw(ctx, false, p)
			return resp, err
		}),
	)
}
