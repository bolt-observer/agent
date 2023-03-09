package boltz

import (
	"context"
	"testing"
	"time"
)

type DummyStruct struct {
}

func (d DummyStruct) GetSwapData() *SwapData {
	return nil
}

func TestInterval(t *testing.T) {
	NewRedeemer[DummyStruct](context.Background(), (RedeemForward | RedeemReverse), nil, nil, nil, 1*time.Second, nil, nil)

	//time.Sleep(5 * time.Second)
}
