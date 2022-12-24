package filter

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFile(t *testing.T) {
	f, err := os.CreateTemp("", "tmpfile-")
	if err != nil {
		t.Fatalf("Got error %v\n", err)
	}

	defer f.Close()
	defer os.Remove(f.Name())

	data := []byte(`
	###
	# Demo file
	###
	1
	2
	3 # Comment
	# 4
	`)
	if _, err := f.Write(data); err != nil {
		t.Fatalf("Got error %v\n", err)
	}

	fil, err := NewFilterFromFile(context.TODO(), f.Name(), None)
	if err != nil {
		t.Fatalf("Got error %v\n", err)
	}

	for _, chanid := range []uint64{1, 2, 3} {
		assert.Equal(t, true, fil.AllowChanID(chanid), "Should be allowed")
	}

	for _, chanid := range []uint64{4, 1337} {
		assert.Equal(t, false, fil.AllowChanID(chanid), "Should not be allowed")
	}
}

func TestOptions(t *testing.T) {
	f, err := os.CreateTemp("", "tmpfile-")
	if err != nil {
		t.Fatalf("Got error %v\n", err)
	}

	defer f.Close()
	defer os.Remove(f.Name())

	data := []byte(`
	###
	# Demo file
	###
	public
	`)
	if _, err := f.Write(data); err != nil {
		t.Fatalf("Got error %v\n", err)
	}

	fil, err := NewFilterFromFile(context.TODO(), f.Name(), AllowAllPrivate)
	if err != nil {
		t.Fatalf("Got error %v\n", err)
	}

	assert.Equal(t, true, fil.AllowSpecial(true), "Private channels should be allowed")
	assert.Equal(t, true, fil.AllowSpecial(false), "Publlic channels should be allowed")
}
