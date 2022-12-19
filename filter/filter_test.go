package filter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFile(t *testing.T) {
	f, err := os.CreateTemp("", "tmpfile-") // in Go version older than 1.17 you can use ioutil.TempFile
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

	fil, err := NewFilterFromFile(context.TODO(), f.Name(), 0*time.Second)
	if err != nil {
		t.Fatalf("Got error %v\n", err)
	}

	for _, chanid := range []uint64{1, 2, 3} {
		assert.Equal(t, true, fil.AllowChanId(chanid), "Should be allowed")
	}

	for _, chanid := range []uint64{4, 1337} {
		assert.Equal(t, false, fil.AllowChanId(chanid), "Should not be allowed")
	}
}
