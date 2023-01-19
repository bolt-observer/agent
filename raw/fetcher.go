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

	st := status.Convert(err)
	switch st.Code() {
	case codes.Unknown:
		// TODO: remove obsolete check
		if strings.Contains(st.Message(), "ConditionalCheckFailedException") {
			return backoff.Permanent(err)
		}
	case codes.NotFound:
	case codes.PermissionDenied:
	case codes.Unimplemented:
	case codes.OutOfRange:
		return backoff.Permanent(err)
	}

	return err
}

type fetcherGetTime func(ctx context.Context) (*agent.TimestampResponse, error)
type fetcherGetChan func(ctx context.Context, itf api.LightingAPICalls, from time.Time) <-chan api.RawMessage
type fetcherPushData func(ctx context.Context, data *agent.DataRequest) error

type fetcherMethod struct {
	methodName string
	entityName string
	getTime    fetcherGetTime
	getChan    fetcherGetChan
	pushData   fetcherPushData
}

func (f *Fetcher) fetch(
	ctx context.Context,
	method fetcherMethod,
	shouldUpdateTimeToLatest bool,
	from time.Time) {

	// TODO: fix this
	ctx = metadata.AppendToOutgoingContext(ctx, "pubkey", f.PubKey, "clientType", fmt.Sprintf("%d", f.ClientType), "key", f.AuthToken)

	defer glog.Warningf("%s stopped running", method.methodName)

	if shouldUpdateTimeToLatest {
		ts, err := method.getTime(ctx)

		if err != nil {
			glog.Warningf("Coud not get latest %s timestamps: %v", method.entityName, err)
		}

		if ts != nil {
			t := time.Unix(0, ts.Timestamp)
			glog.V(2).Infof("Latest %s timestamp: %v", method.entityName, t)
			if t.After(from) {
				from = t
			}
		}
	}

	outchan := method.getChan(ctx, f.LightningAPI, from)

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
				err := method.pushData(ctx, data)
				return makePermanent(err)
			}, b, func(e error, d time.Duration) {
				glog.Warningf("Could not send data to GRPC endpoint: %v %v", data, e)
			})

			if err != nil {
				glog.Warningf("Fatal error in %s: %v", method.methodName, err)
				return
			}

			cnt++
			if cnt%ReportBatch == 0 {
				glog.V(2).Infof("Reported %d %s (last timestamp %v)", cnt, method.entityName, entity.Timestamp)
			}
		}
	}
}

// FetchInvoices will fetch and report invoices
func (f *Fetcher) FetchInvoices(ctx context.Context, shouldUpdateTimeToLatest bool, from time.Time) {
	method := fetcherMethod{
		methodName: "FetchInvoices",
		entityName: "invoices",
		getTime: fetcherGetTime(func(ctx context.Context) (*agent.TimestampResponse, error) {
			return f.AgentAPI.LatestInvoiceTimestamp(ctx, &agent.Empty{})
		}),
		getChan: fetcherGetChan(GetInvoicesChannel),
		pushData: fetcherPushData(func(ctx context.Context, data *agent.DataRequest) error {
			_, err := f.AgentAPI.Invoices(ctx, data)
			return err
		}),
	}

	f.fetch(ctx, method, shouldUpdateTimeToLatest, from)
}

// FetchForwards will fetch and report forwards
func (f *Fetcher) FetchForwards(ctx context.Context, shouldUpdateTimeToLatest bool, from time.Time) {
	method := fetcherMethod{
		methodName: "FetchForwards",
		entityName: "forwards",
		getTime: fetcherGetTime(func(ctx context.Context) (*agent.TimestampResponse, error) {
			return f.AgentAPI.LatestForwardTimestamp(ctx, &agent.Empty{})
		}),
		getChan: fetcherGetChan(GetForwardsChannel),
		pushData: fetcherPushData(func(ctx context.Context, data *agent.DataRequest) error {
			_, err := f.AgentAPI.Forwards(ctx, data)
			return err
		}),
	}

	f.fetch(ctx, method, shouldUpdateTimeToLatest, from)
}

// FetchPayments will fetch and report payments
func (f *Fetcher) FetchPayments(ctx context.Context, shouldUpdateTimeToLatest bool, from time.Time) {
	method := fetcherMethod{
		methodName: "FetchPayments",
		entityName: "payments",
		getTime: fetcherGetTime(func(ctx context.Context) (*agent.TimestampResponse, error) {
			return f.AgentAPI.LatestPaymentTimestamp(ctx, &agent.Empty{})
		}),
		getChan: fetcherGetChan(GetPaymentsChannel),
		pushData: fetcherPushData(func(ctx context.Context, data *agent.DataRequest) error {
			_, err := f.AgentAPI.Payments(ctx, data)
			return err
		}),
	}

	f.fetch(ctx, method, shouldUpdateTimeToLatest, from)
}
