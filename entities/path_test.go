package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanAndExpandPath(t *testing.T) {
	result := CleanAndExpandPath("~//burek")
	assert.NotContains(t, result, "~")
	assert.NotContains(t, result, "//")
	assert.Contains(t, result, "burek")
}
