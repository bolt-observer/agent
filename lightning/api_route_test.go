package lightning

import (
	"context"
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
	return

	/*
		ctx := context.Background()
		srcNode := GetLocalLndByName(t, "A")
		src, err := srcNode()
		assert.NoError(t, err)

		a := ExcludedEdge{ChannelId: 128642860515328}
		route, err := src.GetRoute(ctx, "", getPubkey(t, ctx, "C"), []Exclusion{a}, Reliability, 1000)
		assert.NoError(t, err)
		fmt.Printf("%s\n", route.PrettyRoute(getPubkey(t, ctx, "C"), true))

		fmt.Printf("--------------------------------\n")
		iterator, err := src.GetRoutes(ctx, "", getPubkey(t, ctx, "C"), nil, Reliability, 1000)
		assert.NoError(t, err)

		i := 0
		for route := range iterator {
			fmt.Printf("|%s|\n", route.PrettyRoute(getPubkey(t, ctx, "C"), true))
			i++
			if i > 50 {
				break
			}
		}

		t.Fail()
	*/
}

func TestGetRouteCln(t *testing.T) {
	return

	/*
		ctx := context.Background()
		c := GetLocalCln(t, "B")
		cln, err := c()
		assert.NoError(t, err)

		a := ExcludedEdge{ChannelId: 128642860515328}

		route, err := cln.GetRoute(ctx, getPubkey(t, ctx, "A"), getPubkey(t, ctx, "C"), []Exclusion{a}, Reliability, 1000)
		//route, err := cln.GetRoute(ctx, getPubkey(t, ctx, "A"), getPubkey(t, ctx, "C"), nil, 1000)
		assert.NoError(t, err)
		fmt.Printf("%+v\n", route)
		t.Fail()
	*/
}

func TestRealNode(t *testing.T) {
	return
	/*
		api := getAPI(t, "fixture-grpc.secret", LndGrpc)
		if api == nil {
			return
		}

		eb := NewEmptyExclusionBuilder()
		//eb.AddNode("02a75f33c6fa0e5287d70ca69ca64902fed8beef561b990f33dd4a5a77b9ab43b3")

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		r, err := api.GetRoute(ctx, "", "0327f763c849bfd218910e41eef74f5a737989358ab3565f185e1a61bb7df445b8", eb.Build(), Reliability, 10000)
		assert.NoError(t, err)

		fmt.Printf("%s\n", r.PrettyRoute("0327f763c849bfd218910e41eef74f5a737989358ab3565f185e1a61bb7df445b8", true))

		ch, err := api.GetRoutes(ctx, "", "0327f763c849bfd218910e41eef74f5a737989358ab3565f185e1a61bb7df445b8", eb.Build(), Reliability, 10000)
		assert.NoError(t, err)
		i := 0
		for route := range ch {
			fmt.Printf("|%s|\n", route.PrettyRoute("0327f763c849bfd218910e41eef74f5a737989358ab3565f185e1a61bb7df445b8", true))
			i++
			if i > 50 {
				break
			}
		}

		t.Fail()
	*/
}
