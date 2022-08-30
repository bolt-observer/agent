package entities

import (
	"context"

	api "github.com/bolt-observer/agent/lightning_api"
)

type InfoCallback func(ctx context.Context, report *InfoReport) bool

type InfoReport struct {
	UniqueId  string   `json:"uniqueId,omitempty"` // optional unique identifier
	Timestamp JsonTime `json:"timestamp"`

	api.NodeInfoApiExtended
}
