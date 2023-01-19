package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConvertTimeSetting(t *testing.T) {
	s := convertTimeSetting(0)
	assert.Equal(t, false, s.enabled)

	s = convertTimeSetting(-1)
	assert.Equal(t, true, s.enabled)
	assert.Equal(t, false, s.useLatestTimeFromServer)
	assert.Equal(t, time.Unix(1, 0), s.time)

	s = convertTimeSetting(1)
	assert.Equal(t, true, s.enabled)
	assert.Equal(t, true, s.useLatestTimeFromServer)
	assert.Equal(t, time.Unix(1, 0), s.time)

	// Milliseconds should be 0
	now := time.Unix(time.Now().Unix(), 0)

	s = convertTimeSetting(now.Unix())
	assert.Equal(t, true, s.enabled)
	assert.Equal(t, true, s.useLatestTimeFromServer)
	assert.Equal(t, now, s.time)

	s = convertTimeSetting(-now.Unix())
	assert.Equal(t, true, s.enabled)
	assert.Equal(t, false, s.useLatestTimeFromServer)
	assert.Equal(t, now, s.time)
}
