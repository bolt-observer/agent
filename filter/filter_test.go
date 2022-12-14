package filter

import (
	"context"
	"os"
	"testing"
	"time"
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

	if !fil.AllowChanId(1) {
		t.Fatalf("Should be allowed")
	}

	if !fil.AllowChanId(2) {
		t.Fatalf("Should be allowed")
	}

	if !fil.AllowChanId(3) {
		t.Fatalf("Should be allowed")
	}

	if fil.AllowChanId(4) {
		t.Fatalf("Should not be allowed")
	}

	if fil.AllowChanId(1337) {
		t.Fatalf("Should not be allowed")
	}
}
