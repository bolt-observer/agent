package raw

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bolt-observer/agent/lightning"
)

const FailNoCredsFetcher = false

func TestFetcher(t *testing.T) {
	const (
		TokenFile = "token.secret"
	)
	itf := getAPI(t, "fixture.secret", lightning.LndGrpc)
	if itf == nil {
		return
	}

	if _, err := os.Stat(TokenFile); errors.Is(err, os.ErrNotExist) {
		if FailNoCredsFetcher {
			t.Fatal("No credentials")
		}
		return
	}

	tokenData, err := ioutil.ReadFile(TokenFile)
	if err != nil {
		t.Fatalf("Error when opening file: %v", err)
		return
	}

	token := strings.Trim(string(tokenData), "\t\r\n ")

	f, err := MakeFetcher(context.Background(), token, "agent-api-staging-new.bolt.observer:443", itf)

	if err != nil {
		t.Fatalf("failed to make fetcher: %v", err)
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	f.FetchInvoices(ctx, true, time.Time{})

	//t.Fatal("fail")
}
