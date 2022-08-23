package lightning_api

import (
	"encoding/hex"
	"testing"

	"gopkg.in/macaroon.v2"
)

func TestMacaroonValid(t *testing.T) {

	macBytes, err := hex.DecodeString("0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec")
	if err != nil {
		t.Fatalf("Could not decode macaroon")
		return
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macBytes); err != nil {
		t.Fatalf("Could not unmarshal macaroon")
		return
	}

	valid, duration := IsMacaroonValid(mac)

	if valid || duration > 0 {
		t.Fatalf("Macaroon should not be valid")
		return
	}
}
