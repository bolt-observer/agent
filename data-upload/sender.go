package raw

import (
	"context"
	"fmt"
	"strings"
	"time"

	agent "github.com/bolt-observer/agent/agent"
	"github.com/bolt-observer/agent/entities"
	api "github.com/bolt-observer/agent/lightning"
	backoff "github.com/cenkalti/backoff/v4"
	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ReportBatch is how often a log is printed to show progress
const ReportBatch = 10

// Sender struct
type Sender struct {
	AuthToken    string
	LightningAPI entities.NewAPICall
	AgentAPI     agent.AgentAPIClient
	PubKey       string
	ClientType   int
}

func toClientType(t api.APIType) int {
	// So far this mapping is 1:1 to the API
	return int(t)
}

// MakeSender creates a new Sender
func MakeSender(ctx context.Context, authToken string, endpoint string, l entities.NewAPICall) (*Sender, error) {
	if l == nil {
		return nil, backoff.Permanent(fmt.Errorf("lightning API not specified"))
	}
	f := &Sender{
		AuthToken:    authToken,
		LightningAPI: l,
	}

	api := l()
	if api == nil {
		return nil, backoff.Permanent(fmt.Errorf("lightning API not obtained"))
	}
	defer api.Cleanup()

	info, err := api.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	f.ClientType = toClientType(api.GetAPIType())

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

type senderGetTime func(ctx context.Context, empty *agent.Empty, opts ...grpc.CallOption) (*agent.TimestampResponse, error)
type senderGetChan func(ctx context.Context, lightning entities.NewAPICall, from time.Time) <-chan api.RawMessage
type senderPushData func(ctx context.Context, data *agent.DataRequest, opts ...grpc.CallOption) (*agent.Empty, error)

type senderMethod struct {
	methodName string
	entityName string
	getTime    senderGetTime
	getChan    senderGetChan
	pushData   senderPushData
}

func (f *Sender) send(
	ctx context.Context,
	method senderMethod,
	shouldUpdateTimeToLatest bool,
	from time.Time) {

	// TODO: fix this
	ctx = metadata.AppendToOutgoingContext(ctx, "pubkey", f.PubKey, "clientType", fmt.Sprintf("%d", f.ClientType), "key", f.AuthToken)

	defer glog.Warningf("%s stopped running", method.methodName)

	if shouldUpdateTimeToLatest {
		ts, err := method.getTime(ctx, &agent.Empty{})

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
				_, err := method.pushData(ctx, data)
				return makePermanent(err)
			}, b, func(e error, d time.Duration) {
				st := status.Convert(e)
				switch st.Code() {
				case codes.ResourceExhausted:
					glog.V(4).Infof("Got resource exhausted error %v", e)
				default:
					glog.Warningf("Could not send data to GRPC endpoint: %v", e)
				}
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

// SendInvoices will send invoices
func (f *Sender) SendInvoices(ctx context.Context, shouldUpdateTimeToLatest bool, from time.Time) {
	method := senderMethod{
		methodName: "SendInvoices",
		entityName: "invoices",
		getTime:    senderGetTime(f.AgentAPI.LatestInvoiceTimestamp),
		getChan:    senderGetChan(GetInvoicesChannel),
		pushData:   senderPushData(f.AgentAPI.Invoices),
	}

	f.send(ctx, method, shouldUpdateTimeToLatest, from)
}

// SendForwards will send forwards
func (f *Sender) SendForwards(ctx context.Context, shouldUpdateTimeToLatest bool, from time.Time) {
	method := senderMethod{
		methodName: "SendForwards",
		entityName: "forwards",
		getTime:    senderGetTime(f.AgentAPI.LatestForwardTimestamp),
		getChan:    senderGetChan(GetForwardsChannel),
		pushData:   senderPushData(f.AgentAPI.Forwards),
	}

	f.send(ctx, method, shouldUpdateTimeToLatest, from)
}

// SendPayments will send payments
func (f *Sender) SendPayments(ctx context.Context, shouldUpdateTimeToLatest bool, from time.Time) {
	method := senderMethod{
		methodName: "SendPayments",
		entityName: "payments",
		getTime:    senderGetTime(f.AgentAPI.LatestPaymentTimestamp),
		getChan:    senderGetChan(GetPaymentsChannel),
		pushData:   senderPushData(f.AgentAPI.Payments),
	}

	f.send(ctx, method, shouldUpdateTimeToLatest, from)
}
