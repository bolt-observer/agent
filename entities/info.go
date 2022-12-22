package entities

import (
	"context"

	api "github.com/bolt-observer/agent/lightningapi"
	entities "github.com/bolt-observer/go_common/entities"
)

type InfoCallback func(ctx context.Context, report *InfoReport) bool

type InfoReport struct {
	UniqueId  string            `json:"uniqueId,omitempty"` // optional unique identifier
	Timestamp entities.JsonTime `json:"timestamp"`

	api.NodeInfoApiExtended
}
