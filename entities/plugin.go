package entities

import "context"

type Plugin interface {
	Execute(ctx context.Context, data []byte, msg func(PluginMessage) error) error
}

type PluginMessage struct {
	JobID      int32
	Message    string
	Data       []byte
	IsError    bool
	IsFinished bool
}
