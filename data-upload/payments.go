package raw

import (
	"context"
	"fmt"
	"time"

	"github.com/bolt-observer/agent/entities"
	api "github.com/bolt-observer/agent/lightning"
	"github.com/golang/glog"
)

// GetRawData - signature for the function to get raw data
type GetRawData func(ctx context.Context, lightning entities.NewAPICall, pagination api.RawPagination) ([]api.RawMessage, *api.ResponseRawPagination, error)

// DefaultBatchSize is the default batch size
const DefaultBatchSize = 50

// GetPaymentsChannel returns a channel with raw payments
func GetPaymentsChannel(ctx context.Context, lightning entities.NewAPICall, from time.Time) <-chan api.RawMessage {
	outchan := make(chan api.RawMessage)

	go paginator(ctx, lightning, GetRawData(
		func(ctx context.Context, lightning entities.NewAPICall, pagination api.RawPagination) ([]api.RawMessage, *api.ResponseRawPagination, error) {
			itf := lightning()
			if itf == nil {
				return nil, nil, fmt.Errorf("could not get lightning API")
			}
			return itf.GetPaymentsRaw(ctx, false, pagination)
		}),
		from,
		3, // using a small batch size here because more did not work fine with test lnd node (due to big payload size)
		outchan)

	return outchan
}

// GetInvoicesChannel returns a channel with raw invoices
func GetInvoicesChannel(ctx context.Context, lightning entities.NewAPICall, from time.Time) <-chan api.RawMessage {
	outchan := make(chan api.RawMessage)

	go paginator(ctx, lightning, GetRawData(
		func(ctx context.Context, lightning entities.NewAPICall, pagination api.RawPagination) ([]api.RawMessage, *api.ResponseRawPagination, error) {
			itf := lightning()
			if itf == nil {
				return nil, nil, fmt.Errorf("could not get lightning API")
			}
			return itf.GetInvoicesRaw(ctx, false, pagination)
		}),
		from,
		DefaultBatchSize,
		outchan)

	return outchan
}

// GetForwardsChannel returns a channel with raw forwards
func GetForwardsChannel(ctx context.Context, lightning entities.NewAPICall, from time.Time) <-chan api.RawMessage {
	outchan := make(chan api.RawMessage)

	itf := lightning()

	if itf != nil {
		// A hack to get failed forwards too
		if itf.GetAPIType() == api.LndGrpc {
			lndGrpc, ok := itf.(*api.LndGrpcLightningAPI)
			if ok {
				lndGrpc.SubscribeFailedForwards(ctx, outchan)
			}
		} else if itf.GetAPIType() == api.LndRest {
			lndRest, ok := itf.(*api.LndRestLightningAPI)
			if ok {
				lndRest.SubscribeFailedForwards(ctx, outchan)
			}
		}
	}

	go paginator(ctx, lightning, GetRawData(
		func(ctx context.Context, lightning entities.NewAPICall, pagination api.RawPagination) ([]api.RawMessage, *api.ResponseRawPagination, error) {
			itf := lightning()
			if itf == nil {
				return nil, nil, fmt.Errorf("could not get lightning API")
			}
			return itf.GetForwardsRaw(ctx, pagination)
		}),
		from,
		DefaultBatchSize,
		outchan)

	return outchan
}

// GetFirstDay get first day of the month monthsAgo (0 is current month)
// firstLastMonth := GetFirstDay(1)
// beginOfThisMonth := GetFirstDay(0)
// endOfThisMonth := GetFirstDay(-1).Add(-1 * time.Millisecond)
func GetFirstDay(monthsAgo int) time.Time {
	now := time.Now()

	x := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return x.AddDate(0, -1*monthsAgo, 0)
}

func paginator(ctx context.Context, lightning entities.NewAPICall, getData GetRawData, from time.Time, pageSize int, outchan chan api.RawMessage) error {
	req := api.RawPagination{}
	req.BatchSize = uint64(pageSize)
	req.Pagination.BatchSize = uint64(pageSize)
	isLast := false

	// Try our luck - Raw function should not complain
	req.From = &from

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// Do nothing
		}

		if isLast {
			time.Sleep(10 * time.Second)
		}

		data, page, err := getData(ctx, lightning, req)
		if err != nil {
			// Pretend this is the last page (so we sleep)
			glog.Warningf("paginator error: %v\n", err)
			isLast = true
			continue
		}

		if page.LastTime.After(from) || page.LastTime.Equal(from) {
			for _, one := range data {
				if one.Timestamp.Before(from) {
					continue
				}

				outchan <- one
			}
		}

		if !page.UseTimestamp {
			req.Offset = page.LastOffsetIndex
			if len(data) == 0 {
				isLast = true
			}
		} else {
			isLast = true
		}
	}
}
