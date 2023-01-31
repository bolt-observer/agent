package lightning

import (
	"context"
	"io"
)

// ClnConnection struct
type ClnConnection struct {
}

// ClnConnectionAPI interface
type ClnConnectionAPI interface {
	// Call calls serviceMethod with args and fills reply with response
	Call(ctx context.Context, serviceMethod string, args any, reply any) error
	// StreamResponse is meant for streaming responses it calls serviceMethod with arggs and returns an io.Reader
	StreamResponse(ctx context.Context, serviceMethod string, args any) (io.Reader, error)
}
