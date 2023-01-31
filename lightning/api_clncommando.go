package lightning

import (
	"fmt"
	"time"
)

// LndClnCommandoLightningAPI struct
type LndClnCommandoLightningAPI struct {
	ClnSocketRawAPI
}

// Compile time check for the interface
var _ LightingAPICalls = &LndClnCommandoLightningAPI{}

// NewClnCommandoLightningAPIRaw gets a new API (note that addr is in pubkey@address format)
func NewClnCommandoLightningAPIRaw(addr, rune string) LightingAPICalls {
	api := &LndClnCommandoLightningAPI{}

	api.connection = NewCommandoConnection(addr, rune, 30*time.Second)
	api.Name = "clncommando"

	return api
}

// NewClnCommandoLightningAPI returns a new lightning API
func NewClnCommandoLightningAPI(getData GetDataCall) LightingAPICalls {
	if getData == nil {
		return nil
	}
	data, err := getData()
	if data == nil || err != nil {
		return nil
	}

	return NewClnCommandoLightningAPIRaw(fmt.Sprintf("%s@%s", data.PubKey, data.Endpoint), data.MacaroonHex)
}
