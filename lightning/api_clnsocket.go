package lightning

import (
	"time"
)

// ClnSocketAPI struct
type ClnSocketAPI struct {
	ClnSocketRawAPI
}

// Compile time check for the interface
var _ LightingAPICalls = &ClnSocketAPI{}

// NewClnSocketLightningAPIRaw gets a new API - usage "unix", "/home/ubuntu/.lightning/bitcoin/lightning-rpc"
func NewClnSocketLightningAPIRaw(socketType string, address string) LightingAPICalls {

	api := &ClnSocketAPI{}

	api.connection = MakeUnixConnection(socketType, address, 30*time.Second)
	api.Name = "clnsocket"

	return api
}

// NewClnSocketLightningAPI return a new lightning API
func NewClnSocketLightningAPI(getData GetDataCall) LightingAPICalls {
	if getData == nil {
		return nil
	}
	data, err := getData()
	if data == nil || err != nil {
		return nil
	}

	return NewClnSocketLightningAPIRaw("unix", data.Endpoint)
}
