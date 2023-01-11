package raw

import (
	"context"
	"time"

	agent "github.com/bolt-observer/agent/agent"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/golang/glog"
)

// Fetcher struct
type Fetcher struct {
	AuthToken    string
	LightningAPI api.LightingAPICalls
	AgentAPI     agent.AgentAPIClient
	PubKey       string
}

// MakeFetcher creates a new fetcher
func MakeFetcher(authToken string, endpoint string, l api.LightingAPICalls, a agent.AgentAPIClient) (*Fetcher, error) {
	f := &Fetcher{
		AuthToken:    authToken,
		LightningAPI: l,
	}

	info, err := l.GetInfo(context.Background())
	if err != nil {
		return nil, err
	}

	f.PubKey = info.IdentityPubkey
	agent := GetAgentAPI(endpoint, f.PubKey, f.AuthToken)
	if agent == nil {
		return nil, err
	}
	f.AgentAPI = agent

	return f, nil
}

// FetchInvoices will fetch and report invoices
func (f *Fetcher) FetchInvoices(ctx context.Context, from time.Time) {

	ts, err := f.AgentAPI.LatestInvoice(ctx, &agent.Empty{})
	if err != nil {
		glog.Warningf("Coud not get latest invoice timestamps: %v", err)
	}

	t := time.Unix(ts.Timestamp, 0)
	if t.After(from) {
		from = t
	}

	invoices := GetInvoices(ctx, f.LightningAPI, from)

	for {
		select {
		case <-ctx.Done():
			return
		case invoice := <-invoices:
			_, err := f.AgentAPI.Invoices(ctx, &agent.DataRequest{
				Timestamp: invoice.Timestamp.Unix(),
				Data:      string(invoice.Message),
			})
			if err != nil {
				glog.Warningf("Could not send data to agent: %v", err)
			}
		}
	}
}
