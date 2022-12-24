package entities

import (
	"context"

	api "github.com/bolt-observer/agent/lightning"
	entities "github.com/bolt-observer/go_common/entities"
)

// InfoCallback represents the info callback
type InfoCallback func(ctx context.Context, report *InfoReport) bool

// InfoReport struct
type InfoReport struct {
	UniqueID  string            `json:"uniqueId,omitempty"` // optional unique identifier
	Timestamp entities.JsonTime `json:"timestamp"`

	api.NodeInfoAPIExtended
}
