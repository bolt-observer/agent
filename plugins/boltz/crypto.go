package boltz

import (
	"crypto/hmac"
	"crypto/sha256"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

func DeterministicPrivateKey(id string, orig *secp256k1.PrivateKey) *secp256k1.PrivateKey {
	hash := sha256.Sum256([]byte(id))

	scalar := &secp256k1.ModNScalar{}
	scalar.SetBytes(&hash)

	return secp256k1.NewPrivateKey(orig.Key.Add(scalar))
}

func DeterministicPublicKey(id string, orig *secp256k1.PublicKey) *secp256k1.PublicKey {
	curve := secp256k1.S256()

	hash := sha256.Sum256([]byte(id))
	scalar := &secp256k1.ModNScalar{}
	scalar.SetBytes(&hash)
	bytes := scalar.Bytes()

	x, y := curve.ScalarBaseMult(bytes[:])
	x2, y2 := curve.Add(orig.X(), orig.Y(), x, y)

	var (
		fx secp256k1.FieldVal
		fy secp256k1.FieldVal
	)
	fx.SetByteSlice(x2.Bytes())
	fy.SetByteSlice(y2.Bytes())

	return secp256k1.NewPublicKey(&fx, &fy)
}

func DeterministicPreimage(id string, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(id))
	r := h.Sum(nil)
	return r
}
