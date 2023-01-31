package lightning

import (
	"fmt"
	"time"
)

// ClnCommandoAPI struct
type ClnCommandoAPI struct {
	ClnSocketRawAPI
}

// Compile time check for the interface
var _ LightingAPICalls = &ClnCommandoAPI{}

// NewClnCommandoLightningAPIRaw gets a new API
func NewClnCommandoLightningAPIRaw(addr, rune string) LightingAPICalls {

	api := &ClnCommandoAPI{}

	api.connection = MakeCommandoConnection(addr, rune, 30*time.Second)
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
