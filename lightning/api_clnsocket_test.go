package lightning

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const Deadline = 15 * time.Second

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

	contents, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("Could not read file: %v", err)
	}

	return contents
}

func clnCommon(t *testing.T, handler Handler) (*ClnSocketLightningAPI, LightingAPICalls, CloseFunc) {
	name, closeFunc := ClnSocketServer(t, handler)

	api := NewClnSocketLightningAPIRaw("unix", name)
	if api == nil {
		t.Fatalf("API should not be nil")
	}

	d, ok := api.(*ClnSocketLightningAPI)
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(Deadline))
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
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(Deadline))
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(Deadline))
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(Deadline))
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

func TestClnConnectPeer(t *testing.T) {
	data := clnData(t, "cln_connect")

	pubkey := "0288037d3f0bdcfb240402b43b80cdc32e41528b3e2ebe05884aff507d71fca71a"
	host := "161.97.184.185:9735"

	_, api, closer := clnCommon(t, func(c net.Conn) {
		req := RequestExtractor{}
		err := json.NewDecoder(c).Decode(&req)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		if strings.Contains(req.Method, "connect") {
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(Deadline))
	defer cancel()

	err := api.ConnectPeer(ctx, fmt.Sprintf("%s@%s", pubkey, host))
	assert.NoError(t, err)
}

func TestClnGetOnChainAddress(t *testing.T) {
	data := clnData(t, "cln_newaddr")

	_, api, closer := clnCommon(t, func(c net.Conn) {
		req := RequestExtractor{}
		err := json.NewDecoder(c).Decode(&req)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		if strings.Contains(req.Method, "newaddr") {
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(Deadline))
	defer cancel()

	resp, err := api.GetOnChainAddress(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "bcrt1qxkt36rvxtjlkkxqv6msna7srlg85gx8fa5dd2f", resp)

	assert.NotEqual(t, 0, len(resp))
}

func TestClnGetOnChainFunds(t *testing.T) {
	data := clnData(t, "cln_funds")

	_, api, closer := clnCommon(t, func(c net.Conn) {
		req := RequestExtractor{}
		err := json.NewDecoder(c).Decode(&req)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		if strings.Contains(req.Method, "listfunds") {
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(Deadline))
	defer cancel()

	resp, err := api.GetOnChainFunds(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(85996736), resp.TotalBalance)
	assert.Equal(t, int64(85996736), resp.ConfirmedBalance)
	assert.Equal(t, int64(0), resp.LockedBalance)
}

func TestClnSendToOnChainAddress(t *testing.T) {
	data := clnData(t, "cln_withdraw")

	_, api, closer := clnCommon(t, func(c net.Conn) {
		req := RequestExtractor{}
		err := json.NewDecoder(c).Decode(&req)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		if strings.Contains(req.Method, "withdraw") {
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(Deadline))
	defer cancel()

	txid, err := api.SendToOnChainAddress(ctx, "bcrt1qu00529sy2n6ke3qe5q48t5e47u6wjdyc39s56g", 1337, false, Urgent)
	assert.NoError(t, err)
	assert.Equal(t, "69ae714ef8286742b902524eb9817cbf25cac72e7e5ab29db788a5d02ff923b9", txid)
}

func TestClnPayInvoice(t *testing.T) {
	data := clnData(t, "cln_pay")

	_, api, closer := clnCommon(t, func(c net.Conn) {
		req := RequestExtractor{}
		err := json.NewDecoder(c).Decode(&req)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		if strings.Contains(req.Method, "pay") {
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(Deadline))
	defer cancel()

	payreq := "lnbcrt1p3lvffzpp56jm4cq97e7ea2zucu2t287857kg27gynlh0cwrqp45hpnj8p8x2qdqqcqzpgxqyz5vqsp5ffzl54an7sq8crhft3e5l8h0ph2ye3qwewkes4n4xy2en8p6ctss9qyyssquq66ryhudp2vh032eyggur0wauasr6g86ezapwwwylk6ed5e5rpn92j5k0w674zu72nv3nstc39yv6j7e7ejmr8thzyd8ejly87zz8spzzex77"
	_, err := api.PayInvoice(ctx, payreq, 1337, nil)
	assert.NoError(t, err)
}

func TestClnGetPaymentStatus(t *testing.T) {
	data := clnData(t, "cln_listpay")

	_, api, closer := clnCommon(t, func(c net.Conn) {
		req := RequestExtractor{}
		err := json.NewDecoder(c).Decode(&req)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		if strings.Contains(req.Method, "listsendpay") {
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(Deadline))
	defer cancel()

	resp, err := api.GetPaymentStatus(ctx, "d4b75c00becfb3d50b98e296a3f8f4f590af2093fddf870c01ad2e19c8e13994")
	assert.NoError(t, err)
	assert.NotEqual(t, 0, resp.Preimage)

	_, err = api.GetPaymentStatus(ctx, "d4b75c00becfb3d50b98e296a3f8f4f590af2093fddf870c01ad2e19c8e13995")
	assert.Error(t, err)
}

func TestClnCreateInvoice(t *testing.T) {
	data := clnData(t, "cln_invoice")

	_, api, closer := clnCommon(t, func(c net.Conn) {
		req := RequestExtractor{}
		err := json.NewDecoder(c).Decode(&req)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		if strings.Contains(req.Method, "invoice") {
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(Deadline))
	defer cancel()

	_, err := api.CreateInvoice(ctx, 10000, "d4b75c00becfb3d50b98e296a3f8f4f590af2093fddf870c01ad2e19c8e13995", "test1", 5*time.Hour)
	assert.NoError(t, err)
}

func TestClnIsInvoicePaidPending(t *testing.T) {
	ClnIsInvoicePaid(t, "cln_invoicepending", false)
}

func TestClnIsInvoicePaidComplete(t *testing.T) {
	ClnIsInvoicePaid(t, "cln_invoicepaid", true)
}

func ClnIsInvoicePaid(t *testing.T, name string, paid bool) {
	data := clnData(t, name)

	_, api, closer := clnCommon(t, func(c net.Conn) {
		req := RequestExtractor{}
		err := json.NewDecoder(c).Decode(&req)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		if strings.Contains(req.Method, "listinvoices") {
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

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(Deadline))
	defer cancel()

	resp, err := api.IsInvoicePaid(ctx, "c246e952658ef14d1c71516e8ba66c7f2d16203acac0d8a6795dd97728e85dab")
	assert.NoError(t, err)
	assert.Equal(t, paid, resp)
}

func TestCall(t *testing.T) {
	// socat -t100 -x -v UNIX-LISTEN:/tmp/cln-socket,mode=777,reuseaddr,fork UNIX-CONNECT:/tmp/lnregtest-data/dev_network/lnnodes/B/regtest/lightning-rpc
	// bitcoin-cli -datadir=/tmp/lnregtest-data/dev_network/bitcoin -generate 10

	fileName := "/tmp/cln-socket"
	if _, err := os.Stat(fileName); err != nil {
		return
	}

	api := NewClnSocketLightningAPIRaw("unix", fileName)
	if api == nil {
		t.Fatalf("API should not be nil")
	}
	defer api.Cleanup()

	resp, err := api.GetOnChainFunds(context.Background())
	assert.NoError(t, err)
	fmt.Printf("%+v\n", resp)

	t.Fail()
}
