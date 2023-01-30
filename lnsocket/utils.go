package lnsocket

import (
	"crypto/rand"

	lnwire "github.com/lightningnetwork/lnd/lnwire"
)

// ParseMsgType parses the type of message
func ParseMsgType(bytes []byte) uint16 {
	return uint16(bytes[0])<<8 | uint16(bytes[1])
}

// GetMandatoryFeatures gets mandatory features
func GetMandatoryFeatures(init *lnwire.Init) map[lnwire.FeatureBit]struct{} {
	result := make(map[lnwire.FeatureBit]struct{})

	vector := lnwire.NewFeatureVector(init.Features, nil)
	for one := range vector.Features() {
		bit := uint16(one)

		if bit&1 == 0 {
			result[one] = struct{}{}
		}
	}

	global := lnwire.NewFeatureVector(init.GlobalFeatures, nil)
	for one := range global.Features() {
		bit := uint16(one)

		if bit&1 == 0 {
			result[one] = struct{}{}
		}
	}

	return result
}

// ToRawFeatureVector gets a raw feature vector
func ToRawFeatureVector(features map[lnwire.FeatureBit]struct{}) *lnwire.RawFeatureVector {
	all := make([]lnwire.FeatureBit, 0)

	for feature := range features {
		all = append(all, feature)
	}

	return lnwire.NewRawFeatureVector(all...)
}

// GenRandomBytes gets random bytes
func GenRandomBytes(size int) (blk []byte, err error) {
	blk = make([]byte, size)
	_, err = rand.Read(blk)
	return
}
