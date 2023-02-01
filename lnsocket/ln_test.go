package lnsocket

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseUrl(t *testing.T) {
	ln := NewLN(1 * time.Second)
	assert.NotNil(t, ln)

	_, err := ln.parseURL("bolt.observer:9735")
	assert.Error(t, err)

	pubkey := "0288037d3f0bdcfb240402b43b80cdc32e41528b3e2ebe05884aff507d71fca71a"
	url, err := ln.parseURL(fmt.Sprintf("%s@bolt.observer:9735", pubkey))
	assert.NoError(t, err)
	assert.Equal(t, "tcp", url.addr.Network())

	_, err = ln.parseURL(fmt.Sprintf("%s@tkdqnx3q4eotawbh.onion:9735", pubkey))
	assert.Error(t, err)
	_, err = ln.parseURL(fmt.Sprintf("%s@tkdqnx3q4eotawbh.onion", pubkey))
	assert.Error(t, err)
}
