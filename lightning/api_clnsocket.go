package lightning

// ClnSocketAPI struct
type ClnSocketAPI struct {
	ClnSocketRawAPI
}

// Compile time check for the interface
var _ LightingAPICalls = &ClnSocketAPI{}

// NewClnSocketLightningAPIRaw gets a new API - usage "unix", "/home/ubuntu/.lightning/bitcoin/lightning-rpc"
func NewClnSocketLightningAPIRaw(socketType string, address string) LightingAPICalls {
	api := &ClnSocketAPI{}

	api.connection = MakeUnixConnection(socketType, address)
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

	// "unix" corresponds to SOCK_STREAM
	// "unixgram" corresponds to SOCK_DGRAM
	// "unixpacket" corresponds to SOCK_SEQPACKET
	// We chose stream oriented socket since datagram oriented one has problems with size
	return NewClnSocketLightningAPIRaw("unix", data.Endpoint)
}
