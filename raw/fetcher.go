package raw

import (
	"context"
	"fmt"
	"strings"
	"time"

	agent "github.com/bolt-observer/agent/agent"
	api "github.com/bolt-observer/agent/lightning"
	backoff "github.com/cenkalti/backoff/v4"
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Fetcher struct
type Fetcher struct {
	AuthToken    string
	LightningAPI api.LightingAPICalls
	AgentAPI     agent.AgentAPIClient
	PubKey       string
	ClientType   int
}

func toClientType(t api.APIType) int {
	// So far this mapping is 1:1 to the API
	return int(t)
}

// MakeFetcher creates a new fetcher
func MakeFetcher(ctx context.Context, authToken string, endpoint string, l api.LightingAPICalls) (*Fetcher, error) {
	f := &Fetcher{
		AuthToken:    authToken,
		LightningAPI: l,
	}

	info, err := l.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	f.ClientType = toClientType(l.GetAPIType())

	f.PubKey = info.IdentityPubkey
	agent, err := getAgentAPI(endpoint, f.PubKey, f.AuthToken)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("could not obtain agent")
	}

	f.AgentAPI = agent

	return f, nil
}

func makePermanent(err error) error {
	st := status.Convert(err)
	if st.Code() == codes.Unknown {
		if strings.Contains(st.Message(), "ConditionalCheckFailedException") {
			return backoff.Permanent(err)
		}
	}

	return err
}

// FetchInvoices will fetch and report invoices
func (f *Fetcher) FetchInvoices(ctx context.Context, shouldUpdateTimeToLatest bool, from time.Time) {

	ctx = metadata.AppendToOutgoingContext(ctx, "pubkey", f.PubKey, "clientType", fmt.Sprintf("%d", f.ClientType), "key", f.AuthToken)

	if shouldUpdateTimeToLatest {
		ts, err := f.AgentAPI.LatestInvoiceTimestamp(ctx, &agent.Empty{})

		if err != nil {
			glog.Warningf("Coud not get latest invoice timestamps: %v", err)
		}

		if ts != nil {
			t := time.Unix(0, ts.Timestamp)
			glog.V(2).Infof("Latest invoice timestamp: %v", t)
			if t.After(from) {
				from = t
			}
		}
	}

	outchan := GetInvoicesChannel(ctx, f.LightningAPI, from)

	cnt := 0

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 0

	for {
		select {
		case <-ctx.Done():
			return
		case invoice := <-outchan:
			data := &agent.DataRequest{
				Timestamp: invoice.Timestamp.UnixNano(),
				Data:      string(invoice.Message),
			}

			err := backoff.RetryNotify(func() error {
				_, err := f.AgentAPI.Invoices(ctx, data)
				return makePermanent(err)
			}, b, func(e error, d time.Duration) {
				glog.Warningf("Could not send data to GRPC endpoint: %v %v", data, e)
			})

			if err != nil {
				glog.Warningf("Fatal error in FetchInvoices: %v", err)
				return
			}

			cnt++
			if cnt%10 == 0 {
				glog.V(2).Infof("Reported %d invoices (last timestamp %v)", cnt, invoice.Timestamp)
			}
		}
	}
}

// FetchForwards will fetch and report forwards
func (f *Fetcher) FetchForwards(ctx context.Context, shouldUpdateTimeToLatest bool, from time.Time) {

	ctx = metadata.AppendToOutgoingContext(ctx, "pubkey", f.PubKey, "clientType", fmt.Sprintf("%d", f.ClientType), "key", f.AuthToken)

	if shouldUpdateTimeToLatest {
		ts, err := f.AgentAPI.LatestForwardTimestamp(ctx, &agent.Empty{})

		if err != nil {
			glog.Warningf("Coud not get latest forward timestamps: %v", err)
		}

		if ts != nil {
			t := time.Unix(0, ts.Timestamp)
			glog.V(2).Infof("Latest forwards timestamp: %v", t)
			if t.After(from) {
				from = t
			}
		}
	}

	outchan := GetForwardsChannel(ctx, f.LightningAPI, from)

	cnt := 0

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 0

	for {
		select {
		case <-ctx.Done():
			return
		case forward := <-outchan:
			data := &agent.DataRequest{
				Timestamp: forward.Timestamp.UnixNano(),
				Data:      string(forward.Message),
			}

			err := backoff.RetryNotify(func() error {
				_, err := f.AgentAPI.Forwards(ctx, data)
				return makePermanent(err)
			}, b, func(e error, d time.Duration) {
				glog.Warningf("Could not send data to GRPC endpoint: %v %v", data, e)
			})

			if err != nil {
				glog.Warningf("Fatal error in FetchForwards: %v", err)
				return
			}

			cnt++
			if cnt%10 == 0 {
				glog.V(2).Infof("Reported %d forwards (last timestamp %v)", cnt, forward.Timestamp)
			}
		}
	}
}

// FetchPayments will fetch and report payments
func (f *Fetcher) FetchPayments(ctx context.Context, shouldUpdateTimeToLatest bool, from time.Time) {

	ctx = metadata.AppendToOutgoingContext(ctx, "pubkey", f.PubKey, "clientType", fmt.Sprintf("%d", f.ClientType), "key", f.AuthToken)

	if shouldUpdateTimeToLatest {
		ts, err := f.AgentAPI.LatestPaymentTimestamp(ctx, &agent.Empty{})

		if err != nil {
			glog.Warningf("Coud not get latest forward timestamps: %v", err)
		}

		if ts != nil {
			t := time.Unix(0, ts.Timestamp)
			glog.V(2).Infof("Latest payments timestamp: %v", t)
			if t.After(from) {
				from = t
			}
		}
	}

	outchan := GetPaymentsChannel(ctx, f.LightningAPI, from)

	cnt := 0

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 0

	for {
		select {
		case <-ctx.Done():
			return
		case payment := <-outchan:
			data := &agent.DataRequest{
				Timestamp: payment.Timestamp.UnixNano(),
				Data:      string(payment.Message),
			}

			err := backoff.RetryNotify(func() error {
				_, err := f.AgentAPI.Payments(ctx, data)
				return makePermanent(err)
			}, b, func(e error, d time.Duration) {
				glog.Warningf("Could not send data to GRPC endpoint: %v %v", data, e)
			})

			if err != nil {
				glog.Warningf("Fatal error in FetchPayments: %v", err)
				return
			}

			cnt++
			if cnt%10 == 0 {
				glog.V(2).Infof("Reported %d payments (last timestamp %v)", cnt, payment.Timestamp)
			}
		}
	}
}
