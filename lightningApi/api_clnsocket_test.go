package lightningapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"
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

func clnCommon(t *testing.T, handler Handler) (*ClnSocketLightningAPI, LightingApiCalls, CloseFunc) {
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

type IDExtractor struct {
	ID int `json:"id"`
}

func TestClnGetInfo(t *testing.T) {
	data := clnData(t, "cln_info")

	_, api, closer := clnCommon(t, func(c net.Conn) {
		buf := make([]byte, BUFSIZE)
		n, err := c.Read(buf)
		if err != nil {
			t.Fatalf("Could not read request body: %v", err)
		}

		// Reslice else the thing contains zero bytes
		buf = buf[:n]
		s := string(buf)

		id := IDExtractor{}
		err = json.Unmarshal(buf, &id)
		if err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		if strings.Contains(s, "getinfo") {
			reply := fmt.Sprintf(string(data), id.ID)
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

	resp, err := api.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("GetInfo call failed: %v", err)
	}

	if resp.IdentityPubkey != "03d1c07e00297eae99263dcc01850ec7339bb4c87a1a3e841a195cbfdcdec7a219" || resp.Alias != "cln1" || resp.Chain != "mainnet" || resp.Network != "bitcoin" {
		t.Fatal("Wrong response")
	}
}

func TestClnGetChanInfo(t *testing.T) {
	data := clnData(t, "cln_listchans")

	_, api, closer := clnCommon(t, func(c net.Conn) {
		buf := make([]byte, BUFSIZE)
		n, err := c.Read(buf)
		if err != nil {
			t.Fatalf("Could not read request body: %v", err)
		}

		// Reslice else the thing contains zero bytes
		buf = buf[:n]
		s := string(buf)

		id := IDExtractor{}
		err = json.Unmarshal(buf, &id)
		if err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		if strings.Contains(s, "listchannels") {
			reply := fmt.Sprintf(string(data), id.ID)
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
	resp, err := api.GetChanInfo(context.Background(), id)
	if err != nil {
		t.Fatalf("GetInfo call failed: %v", err)
	}

	if resp.ChannelId != 839247329907769344 || resp.Node1Pub != "020f63ca0fd5cbb11012727c035b7c087c2d014a26ed8ed5ed2115c783945a3fc7" || resp.Node2Pub != "03d1c07e00297eae99263dcc01850ec7339bb4c87a1a3e841a195cbfdcdec7a219" {
		t.Fatal("Wrong response")
	}
}
