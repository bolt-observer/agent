package lightning

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertArgs(t *testing.T) {
	assert.Equal(t, `["1337","1338"]`, convertArgs([]string{"1337", "1338"}))
	assert.Equal(t, `[1337,1338]`, convertArgs([]int{1337, 1338}))
	assert.Equal(t, `[]`, convertArgs([]int{}))
	assert.Equal(t, `[]`, convertArgs(nil))
}
