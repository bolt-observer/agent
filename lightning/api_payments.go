package lightning

import (
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
