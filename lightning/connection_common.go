package lightning

import "time"

// ClnConnection struct
type ClnConnection struct {
	Timeout time.Duration
}

// ClnConnectionAPI interface
type ClnConnectionAPI interface {
	CallWithTimeout(serviceMethod string, args any, reply any) error
}
