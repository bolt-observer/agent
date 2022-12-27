package lightning

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

// ToLndChanID - converts from CLN to LND channel id
func ToLndChanID(id string) (uint64, error) {
	split := strings.Split(strings.ToLower(id), "x")
	if len(split) != 3 {
		return 0, fmt.Errorf("wrong channel id: %v", id)
	}

	blockID, err := strconv.ParseUint(split[0], 10, 64)
	if err != nil {
		return 0, err
	}

	txIdx, err := strconv.ParseUint(split[1], 10, 64)
	if err != nil {
		return 0, err
	}

	outputIdx, err := strconv.ParseUint(split[2], 10, 64)
	if err != nil {
		return 0, err
	}

	result := (blockID&0xffffff)<<40 | (txIdx&0xffffff)<<16 | (outputIdx & 0xffff)

	return result, nil
}

// FromLndChanID - converts from LND to CLN channel id
func FromLndChanID(chanID uint64) string {
	blockID := int64((chanID & 0xffffff0000000000) >> 40)
	txIdx := int((chanID & 0x000000ffffff0000) >> 16)
	outputIdx := int(chanID & 0x000000000000ffff)

	return fmt.Sprintf("%dx%dx%d", blockID, txIdx, outputIdx)
}

// ConvertAmount - converts string amount to satoshis
func ConvertAmount(s string) uint64 {
	x := strings.ReplaceAll(s, "msat", "")
	ret, err := strconv.ParseUint(x, 10, 64)
	if err != nil {
		glog.Warningf("Could not convert: %v %v", s, err)
		return 0
	}

	return ret
}

// ConvertFeatures - convert bitmask to LND like feature map
func ConvertFeatures(features string) map[string]NodeFeatureAPI {
	n := new(big.Int)

	n, ok := n.SetString(features, 16)
	if !ok {
		return nil
	}

	result := make(map[string]NodeFeatureAPI)

	m := big.NewInt(0)
	zero := big.NewInt(0)
	two := big.NewInt(2)

	bit := 0
	for n.Cmp(zero) == 1 {
		n.DivMod(n, two, m)

		if m.Cmp(zero) != 1 {
			// Bit is not set
			bit++
			continue
		}

		result[fmt.Sprintf("%d", bit)] = NodeFeatureAPI{
			Name:       "",
			IsKnown:    true,
			IsRequired: bit%2 == 0,
		}

		bit++
	}

	return result
}

// SumCapacitySimple - get sum of channel capacity
func SumCapacitySimple(channels []NodeChannelAPI) uint64 {
	sum := uint64(0)
	for _, channel := range channels {
		sum += channel.Capacity
	}

	return sum
}

// SumCapacityExtended - get sum of channel capacity
func SumCapacityExtended(channels []NodeChannelAPIExtended) uint64 {
	sum := uint64(0)
	for _, channel := range channels {
		sum += channel.Capacity
	}

	return sum
}
