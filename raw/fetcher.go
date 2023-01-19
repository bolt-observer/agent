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

// ReportBatch is how often a log is printed to show progress
const ReportBatch = 10

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
	if err == nil {
		return nil
	}

	fmt.Printf("Error: %+v\n", err)

	st := status.Convert(err)
	if st.Code() == codes.Unknown {
		if strings.Contains(st.Message(), "ConditionalCheckFailedException") {
			return backoff.Permanent(err)
		}
	}

	return err
}

// FetcherGetTime a function signature to get time
type FetcherGetTime func(ctx context.Context) (*agent.TimestampResponse, error)

// FetcherGetChan a function signature to get chan
type FetcherGetChan func(ctx context.Context, itf api.LightingAPICalls, from time.Time) <-chan api.RawMessage

// FetcherPushData a function signature to push data
type FetcherPushData func(ctx context.Context, data *agent.DataRequest) error

func (f *Fetcher) fetch(
	ctx context.Context,
	methodName string,
	entityName string,
	getTime FetcherGetTime,
	getChan FetcherGetChan,
	pushData FetcherPushData,
	shouldUpdateTimeToLatest bool,
	from time.Time) {

	// TODO: fix this
	ctx = metadata.AppendToOutgoingContext(ctx, "pubkey", f.PubKey, "clientType", fmt.Sprintf("%d", f.ClientType), "key", f.AuthToken)

	defer glog.Warningf("%s stopped running", methodName)

	if shouldUpdateTimeToLatest {
		ts, err := getTime(ctx)

		if err != nil {
			glog.Warningf("Coud not get latest %s timestamps: %v", entityName, err)
		}

		if ts != nil {
			t := time.Unix(0, ts.Timestamp)
			glog.V(2).Infof("Latest %s timestamp: %v", entityName, t)
			if t.After(from) {
				from = t
			}
		}
	}

	outchan := getChan(ctx, f.LightningAPI, from)

	cnt := 0

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 0

	for {
		select {
		case <-ctx.Done():
			return
		case entity := <-outchan:
			data := &agent.DataRequest{
				Timestamp: entity.Timestamp.UnixNano(),
				Data:      string(entity.Message),
			}

			err := backoff.RetryNotify(func() error {
				err := pushData(ctx, data)
				return makePermanent(err)
			}, b, func(e error, d time.Duration) {
				glog.Warningf("Could not send data to GRPC endpoint: %v %v", data, e)
			})

			if err != nil {
				glog.Warningf("Fatal error in %s: %v", methodName, err)
				return
			}

			cnt++
			if cnt%ReportBatch == 0 {
				glog.V(2).Infof("Reported %d %s (last timestamp %v)", cnt, entityName, entity.Timestamp)
			}
		}
	}
}

// FetchInvoices will fetch and report invoices
func (f *Fetcher) FetchInvoices(ctx context.Context, shouldUpdateTimeToLatest bool, from time.Time) {
	getTime := func(ctx context.Context) (*agent.TimestampResponse, error) {
		return f.AgentAPI.LatestInvoiceTimestamp(ctx, &agent.Empty{})
	}

	getChan := func(ctx context.Context, itf api.LightingAPICalls, from time.Time) <-chan api.RawMessage {
		return GetInvoicesChannel(ctx, itf, from)
	}

	pushData := func(ctx context.Context, data *agent.DataRequest) error {
		_, err := f.AgentAPI.Invoices(ctx, data)
		return err
	}

	f.fetch(ctx, "FetchInvoices", "invoices", getTime, getChan, pushData, shouldUpdateTimeToLatest, from)
}

// FetchForwards will fetch and report forwards
func (f *Fetcher) FetchForwards(ctx context.Context, shouldUpdateTimeToLatest bool, from time.Time) {

	getTime := func(ctx context.Context) (*agent.TimestampResponse, error) {
		return f.AgentAPI.LatestForwardTimestamp(ctx, &agent.Empty{})
	}

	getChan := func(ctx context.Context, itf api.LightingAPICalls, from time.Time) <-chan api.RawMessage {
		return GetForwardsChannel(ctx, itf, from)
	}

	pushData := func(ctx context.Context, data *agent.DataRequest) error {
		_, err := f.AgentAPI.Forwards(ctx, data)
		return err
	}

	f.fetch(ctx, "FetchForwards", "forwards", getTime, getChan, pushData, shouldUpdateTimeToLatest, from)
}

// FetchPayments will fetch and report payments
func (f *Fetcher) FetchPayments(ctx context.Context, shouldUpdateTimeToLatest bool, from time.Time) {
	getTime := func(ctx context.Context) (*agent.TimestampResponse, error) {
		return f.AgentAPI.LatestPaymentTimestamp(ctx, &agent.Empty{})
	}

	getChan := func(ctx context.Context, itf api.LightingAPICalls, from time.Time) <-chan api.RawMessage {
		return GetPaymentsChannel(ctx, itf, from)
	}

	pushData := func(ctx context.Context, data *agent.DataRequest) error {
		_, err := f.AgentAPI.Payments(ctx, data)
		return err
	}

	f.fetch(ctx, "FetchPayments", "payments", getTime, getChan, pushData, shouldUpdateTimeToLatest, from)
}
