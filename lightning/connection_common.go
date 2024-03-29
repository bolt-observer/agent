package lightning

import (
	"context"
	"io"
	"time"
)

// ClnConnection struct.
type ClnConnection struct{}

// ClnConnectionAPI interface.
type ClnConnectionAPI interface {
	// Call calls serviceMethod with args and fills reply with response
	Call(ctx context.Context, serviceMethod string, args any, reply any, timeout time.Duration) error
	// StreamResponse is meant for streaming responses it calls serviceMethod with args and returns an io.Reader
	StreamResponse(ctx context.Context, serviceMethod string, args any) (io.Reader, error)
	// Cleanup does the cleanup
	Cleanup()
}
