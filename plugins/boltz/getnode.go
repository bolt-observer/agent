package boltz

import (
  "github.com/BoltzExchange/boltz-lnd/boltz"
  "fmt"
)


// Connect tests the API
func Connect() {
	b := &boltz.Boltz{
		URL:    "https://boltz.exchange/api",
	}
        fmt.Printf("%v", b)
}
