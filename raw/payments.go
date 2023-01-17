package raw

import (
	"context"
	"time"

	api "github.com/bolt-observer/agent/lightning"
	"github.com/golang/glog"
)

// GetRawData - signature for the function to get raw data
type GetRawData func(ctx context.Context, itf api.LightingAPICalls, pagination api.RawPagination) ([]api.RawMessage, *api.ResponseRawPagination, error)

// GetPaymentsChannel returns payments
func GetPaymentsChannel(ctx context.Context, itf api.LightingAPICalls, from time.Time) <-chan api.RawMessage {
	outchan := make(chan api.RawMessage)

	go paginator(ctx, itf,
		func(ctx context.Context, itf api.LightingAPICalls, pagination api.RawPagination) ([]api.RawMessage, *api.ResponseRawPagination, error) {
			return itf.GetPaymentsRaw(ctx, false, pagination)
		},
		from,
		3,
		outchan)

	return outchan
}

// GetInvoicesChannel returns invoices
func GetInvoicesChannel(ctx context.Context, itf api.LightingAPICalls, from time.Time) <-chan api.RawMessage {
	outchan := make(chan api.RawMessage)

	go paginator(ctx, itf,
		func(ctx context.Context, itf api.LightingAPICalls, pagination api.RawPagination) ([]api.RawMessage, *api.ResponseRawPagination, error) {
			return itf.GetInvoicesRaw(ctx, false, pagination)
		},
		from,
		50,
		outchan)

	return outchan
}

// GetForwardsChannel returns forwards
func GetForwardsChannel(ctx context.Context, itf api.LightingAPICalls, from time.Time) <-chan api.RawMessage {
	outchan := make(chan api.RawMessage)

	go paginator(ctx, itf,
		func(ctx context.Context, itf api.LightingAPICalls, pagination api.RawPagination) ([]api.RawMessage, *api.ResponseRawPagination, error) {
			return itf.GetForwardsRaw(ctx, pagination)
		},
		from,
		50,
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

func paginator(ctx context.Context, itf api.LightingAPICalls, getData GetRawData, from time.Time, pageSize int, outchan chan api.RawMessage) error {
	req := api.RawPagination{}
	req.Num = uint64(pageSize)
	req.Pagination.Num = uint64(pageSize)
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

		data, page, err := getData(ctx, itf, req)
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
