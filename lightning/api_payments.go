package lightningapi

import (
	"context"
	"time"
)

// GetFirstDay get first day of the month monthsAgo (0 is current month)
// firstLastMonth := GetFirstDay(1)
// beginOfThisMonth := GetFirstDay(0)
// endOfThisMonth := GetFirstDay(-1).Add(-1 * time.Millisecond)
func GetFirstDay(monthsAgo int) time.Time {
	now := time.Now()

	x := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return x.AddDate(0, -1*monthsAgo, 0)
}

// IterateForwardingHistory - get an interator to obtain ForwardingEvent's, second parameter is stop channel
func IterateForwardingHistory(api LightingAPICalls, from, to time.Time, batchSize uint) (<-chan ForwardingEvent, chan<- bool) {
	lastOffset := uint64(0)

	stopChan := make(chan bool, 1)
	outChan := make(chan ForwardingEvent)

	go func() {
		for {
			select {
			case <-stopChan:
				return
			default:
				resp, err := api.GetForwardingHistory(context.Background(), Pagination{
					Num:    uint64(batchSize),
					From:   &from,
					To:     &to,
					Offset: lastOffset,
				})
				if err != nil || resp == nil {
					return
				}

				if resp.ForwardingEvents != nil {
					for _, one := range resp.ForwardingEvents {
						outChan <- one
					}
				}

				lastOffset = resp.LastOffsetIndex
				if len(resp.ForwardingEvents) == 0 {
					time.Sleep(5 * time.Second)
				}
			}
		}
	}()

	return outChan, stopChan
}
