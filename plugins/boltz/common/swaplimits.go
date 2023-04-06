//go:build plugins
// +build plugins

package common

type SwapLimits struct {
	MaxFeePercentage float64
	AllowZeroConf    bool
	MinSwap          uint64
	MaxSwap          uint64
	DefaultSwap      uint64
	MaxAttempts      int
}

type JobID int32