package raw

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bolt-observer/agent/lightning"
)

const FailNoCredsSender = false

func TestSender(t *testing.T) {
	const (
		TokenFile = "token.secret"
	)
	itf := getAPI(t, "fixture.secret", lightning.LndGrpc)
	if itf == nil {
		return
	}

	if _, err := os.Stat(TokenFile); errors.Is(err, os.ErrNotExist) {
		if FailNoCredsSender {
			t.Fatal("No credentials")
		}
		return
	}

	tokenData, err := os.ReadFile(TokenFile)
	if err != nil {
		t.Fatalf("Error when opening file: %v", err)
		return
	}

	token := strings.Trim(string(tokenData), "\t\r\n ")

	f, err := MakeSender(context.Background(), token, "agent-api-staging-new.bolt.observer:443", itf)

	if err != nil {
		t.Fatalf("failed to make Sender: %v", err)
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	f.SendInvoices(ctx, true, time.Time{})

	//t.Fatal("fail")
}
