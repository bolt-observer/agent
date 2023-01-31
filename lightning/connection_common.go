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
	Call(ctx context.Context, serviceMethod string, args any, reply any) error
	StreamResponse(ctx context.Context, serviceMethod string, args any) (io.Reader, error)
}
