package raw

import (
	"context"
	"fmt"
	"time"

	agent "github.com/bolt-observer/agent/agent"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/golang/glog"
	"google.golang.org/grpc/metadata"
)

// Fetcher struct
type Fetcher struct {
	AuthToken    string
	LightningAPI api.LightingAPICalls
	AgentAPI     agent.AgentAPIClient
	PubKey       string
	ClientType   int
}

// MakeFetcher creates a new fetcher
func MakeFetcher(authToken string, endpoint string, l api.LightingAPICalls) (*Fetcher, error) {
	f := &Fetcher{
		AuthToken:    authToken,
		LightningAPI: l,
	}

	info, err := l.GetInfo(context.Background())
	if err != nil {
		return nil, err
	}

	typ := 0
	switch l.GetAPIType() {
	case api.LndGrpc:
		typ = 0
	case api.LndRest:
		typ = 1
	case api.ClnSocket:
		typ = 2
	}
	f.ClientType = typ

	f.PubKey = info.IdentityPubkey
	agent := GetAgentAPI(endpoint, f.PubKey, f.AuthToken)
	if agent == nil {
		return nil, err
	}
	f.AgentAPI = agent

	return f, nil
}

// FetchInvoices will fetch and report invoices
func (f *Fetcher) FetchInvoices(ctx context.Context, updateTimeWithLast bool, from time.Time) {

	ctx = metadata.AppendToOutgoingContext(ctx, "pubkey", f.PubKey, "clientType", fmt.Sprintf("%d", f.ClientType), "key", f.AuthToken)

	if updateTimeWithLast {
		ts, err := f.AgentAPI.LatestInvoiceTimestamp(ctx, &agent.Empty{})

		if err != nil {
			glog.Warningf("Coud not get latest invoice timestamps: %v", err)
		}

		if ts != nil {
			t := time.Unix(0, ts.Timestamp)
			if t.After(from) {
				from = t
			}
		}
	}

	outchan := GetInvoicesChannel(ctx, f.LightningAPI, from)

	num := 0

	for {
		select {
		case <-ctx.Done():
			return
		case invoice := <-outchan:
			_, err := f.AgentAPI.Invoices(ctx, &agent.DataRequest{
				Timestamp: invoice.Timestamp.UnixNano(),
				Data:      string(invoice.Message),
			})
			if err != nil {
				glog.Warningf("Could not send data to GRPC endpoint: %v", err)
				continue
			}
			num++
			if num%10 == 0 {
				glog.V(2).Infof("Reported %d invoices (last timestamp %v)", num, invoice.Timestamp)
			}
		}
	}
}

// FetchForwards will fetch and report forwards
func (f *Fetcher) FetchForwards(ctx context.Context, updateTimeWithLast bool, from time.Time) {

	ctx = metadata.AppendToOutgoingContext(ctx, "pubkey", f.PubKey, "clientType", fmt.Sprintf("%d", f.ClientType), "key", f.AuthToken)

	if updateTimeWithLast {
		ts, err := f.AgentAPI.LatestForwardTimestamp(ctx, &agent.Empty{})

		if err != nil {
			glog.Warningf("Coud not get latest forward timestamps: %v", err)
		}

		if ts != nil {
			t := time.Unix(0, ts.Timestamp)
			if t.After(from) {
				from = t
			}
		}
	}

	outchan := GetForwardsChannel(ctx, f.LightningAPI, from)

	num := 0

	for {
		select {
		case <-ctx.Done():
			return
		case forward := <-outchan:
			_, err := f.AgentAPI.Forwards(ctx, &agent.DataRequest{
				Timestamp: forward.Timestamp.UnixNano(),
				Data:      string(forward.Message),
			})
			if err != nil {
				glog.Warningf("Could not send data to GRPC endpoint: %v", err)
				continue
			}
			num++
			if num%10 == 0 {
				glog.V(2).Infof("Reported %d forwards (last timestamp %v)", num, forward.Timestamp)
			}
		}
	}
}

// FetchPayments will fetch and report payments
func (f *Fetcher) FetchPayments(ctx context.Context, updateTimeWithLast bool, from time.Time) {

	ctx = metadata.AppendToOutgoingContext(ctx, "pubkey", f.PubKey, "clientType", fmt.Sprintf("%d", f.ClientType), "key", f.AuthToken)

	if updateTimeWithLast {
		ts, err := f.AgentAPI.LatestPaymentTimestamp(ctx, &agent.Empty{})

		if err != nil {
			glog.Warningf("Coud not get latest forward timestamps: %v", err)
		}

		if ts != nil {
			t := time.Unix(0, ts.Timestamp)
			if t.After(from) {
				from = t
			}
		}
	}

	outchan := GetPaymentsChannel(ctx, f.LightningAPI, from)

	num := 0

	for {
		select {
		case <-ctx.Done():
			return
		case payment := <-outchan:
			_, err := f.AgentAPI.Payments(ctx, &agent.DataRequest{
				Timestamp: payment.Timestamp.UnixNano(),
				Data:      string(payment.Message),
			})
			if err != nil {
				glog.Warningf("Could not send data to GRPC endpoint: %v", err)
				continue
			}
			num++
			if num%10 == 0 {
				glog.V(2).Infof("Reported %d payments (last timestamp %v)", num, payment.Timestamp)
			}
		}
	}
}
