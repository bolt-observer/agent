package boltz

import (
	"fmt"

	"github.com/BoltzExchange/boltz-lnd/boltz"
)

// Connect tests the API.
func Connect() {
	b := &boltz.Boltz{
		URL: "https://boltz.exchange/api",
	}
	fmt.Printf("%v", b)
}
