package lightning

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func getPubkey(t *testing.T, ctx context.Context, key string) string {
	node := GetLocalLndByName(t, key)
	dst, err := node()
	assert.NoError(t, err)
	dstData, err := dst.GetInfo(ctx)
	assert.NoError(t, err)

	return dstData.IdentityPubkey
}

// getHops returns lnd hops for BuildRoute equivalent
func getHops(route DeterminedRoute, dst string) []string {
	ret := make([]string, 0)

	for i := 1; i < len(route); i++ {
		ret = append(ret, route[i].PubKey)
	}

	ret = append(ret, dst)

	return ret
}

func TestIsValidPubKey(t *testing.T) {
	assert.True(t, IsValidPubKey("0327f763c849bfd218910e41eef74f5a737989358ab3565f185e1a61bb7df445b8"))
	assert.False(t, IsValidPubKey("0327f763c849bfd218910e41eef74f5a737989358ab3565f185e1a61bb7df445b8a"))
	assert.False(t, IsValidPubKey("0127f763c849bfd218910e41eef74f5a737989358ab3565f185e1a61bb7df445b8"))
	assert.False(t, IsValidPubKey("0327f763c849bfd218910e41eef74f5a737989358ab3565f185e1a61bb7df445b"))
	assert.True(t, IsValidPubKey("0227f763c849bfd218910e41eef74f5a737989358ab3565f185e1a61bb7df445F8"))
	assert.False(t, IsValidPubKey(""))
	assert.False(t, IsValidPubKey("03"))
	assert.False(t, IsValidPubKey("03ff"))
	assert.False(t, IsValidPubKey("ffff"))
}

func TestGetRouteLnd(t *testing.T) {

	ctx := context.Background()
	srcNode := GetLocalLndByName(t, "A")
	src, err := srcNode()
	assert.NoError(t, err)
	srcData, err := src.GetInfo(ctx)
	assert.NoError(t, err)

	a := ExcludedEdge{ChannelId: 128642860515328}

	route, err := src.GetRoute(ctx, srcData.IdentityPubkey, getPubkey(t, ctx, "C"), []Exclusion{a}, 1000)
	//route, err := src.GetRoute(ctx, srcData.IdentityPubkey, getPubkey(t, ctx, "C"), nil, 1000)
	assert.NoError(t, err)

	fmt.Printf("from %s -> %s\n", srcData.IdentityPubkey, getPubkey(t, ctx, "C"))
	fmt.Printf("%v %+v\n", srcData, route)
	//t.Fail()
}

func TestGetRouteCln(t *testing.T) {
	ctx := context.Background()
	c := GetLocalCln(t, "B")
	cln, err := c()
	assert.NoError(t, err)

	a := ExcludedEdge{ChannelId: 128642860515328}

	route, err := cln.GetRoute(ctx, getPubkey(t, ctx, "A"), getPubkey(t, ctx, "C"), []Exclusion{a}, 1000)
	//route, err := cln.GetRoute(ctx, getPubkey(t, ctx, "A"), getPubkey(t, ctx, "C"), nil, 1000)
	assert.NoError(t, err)
	fmt.Printf("%+v\n", route)
	//t.Fail()
}
