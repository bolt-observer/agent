package lightning

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type FooBar struct {
	Foo string `json:"foo"`
	Bar int    `json:"bar"`
}

func TestDoRequest(t *testing.T) {
	var (
		data  FooBar
		data2 FooBar
	)
	contents := `{"foo":"foostring","bar":1}`
	h := &HTTPAPI{}

	h.DoFunc = func(req *http.Request) (*http.Response, error) {
		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 200,
			Body:       r,
		}, nil
	}
	h.doRequest(nil, &data)

	assert.Equal(t, "foostring", data.Foo)
	assert.Equal(t, 1, data.Bar)

	h.DoFunc = func(req *http.Request) (*http.Response, error) {
		r := ioutil.NopCloser(bytes.NewReader([]byte(contents)))

		return &http.Response{
			StatusCode: 400,
			Body:       r,
		}, nil
	}
	h.doRequest(nil, &data2)
	assert.Equal(t, FooBar{}, data2)
}
