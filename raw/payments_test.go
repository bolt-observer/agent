package raw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bolt-observer/go_common/entities"
	"github.com/stretchr/testify/assert"

	"github.com/procodr/monkey"

	api "github.com/bolt-observer/agent/lightning"
)

func getAPI(t *testing.T, name string, typ api.APIType) api.LightingAPICalls {
	var data entities.Data

	if _, err := os.Stat(name); errors.Is(err, os.ErrNotExist) {
		if FailNoCreds {
			t.Fatalf("No credentials")
		}
		return nil
	}

	content, err := ioutil.ReadFile(name)
	if err != nil {
		t.Fatalf("Error when opening file: %v", err)
		return nil
	}

	err = json.Unmarshal(content, &data)
	if err != nil {
		t.Fatalf("Error during Unmarshal(): %v", err)
		return nil
	}

	s := strings.Split(data.Endpoint, ":")
	if len(s) == 2 {
		if typ == api.LndGrpc {
			data.Endpoint = fmt.Sprintf("%s:%s", s[0], "10009")
		} else if typ == api.LndRest {
			data.Endpoint = fmt.Sprintf("%s:%s", s[0], "8080")
		}
	}

	api := api.NewAPI(typ, func() (*entities.Data, error) {
		return &data, nil
	})

	return api
}

func TestGetFirstDay(t *testing.T) {

	monkey.Patch(time.Now, func() time.Time {
		return time.Date(2023, 01, 15, 20, 34, 58, 651387237, time.UTC)
	})

	res := GetFirstDay(0)
	assert.Equal(t, time.January, res.Month())
	assert.Equal(t, 1, res.Day())
	assert.Equal(t, 2023, res.Year())

	res = GetFirstDay(1)
	assert.Equal(t, time.December, res.Month())
	assert.Equal(t, 1, res.Day())
	assert.Equal(t, 2022, res.Year())

	res = GetFirstDay(-1).Add(-1 * time.Millisecond)
	assert.Equal(t, time.January, res.Month())
	assert.Equal(t, 31, res.Day())
	assert.Equal(t, 2023, res.Year())
}

func TestGetInvoices(t *testing.T) {
	const Limit = 3

	from := time.Date(2021, 01, 01, 0, 0, 0, 0, time.UTC)

	for _, lightningImpl := range []api.APIType{api.LndGrpc, api.LndRest} {
		itf := getAPI(t, "fixture.secret", lightningImpl)
		if itf == nil {
			return
		}

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
		defer cancel()

		channel := GetInvoices(ctx, itf, from)
		num := 0
		for i := 0; i < Limit; i++ {
			data := <-channel
			//fmt.Printf("Received %v %s\n", data.Timestamp, string(data.Message))
			if data.Timestamp.Before(from) {
				t.Fatalf("Data has too old timestamp %v", data.Timestamp)
			}
			num++
		}

		assert.Equal(t, Limit, num)
	}

	t.Fatalf("fail")
}

func TestGetForwards(t *testing.T) {
	const Limit = 3

	from := time.Date(2021, 01, 01, 0, 0, 0, 0, time.UTC)

	for _, lightningImpl := range []api.APIType{api.LndGrpc, api.LndRest} {
		itf := getAPI(t, "fixture.secret", lightningImpl)
		if itf == nil {
			return
		}

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
		defer cancel()

		channel := GetForwards(ctx, itf, from)
		num := 0
		for i := 0; i < Limit; i++ {
			data := <-channel
			//fmt.Printf("Received %v %s\n", data.Timestamp, string(data.Message))
			if data.Timestamp.Before(from) {
				t.Fatalf("Data has too old timestamp %v", data.Timestamp)
			}
			num++
		}

		assert.Equal(t, Limit, num)
	}

	//t.Fatalf("fail")
}

func TestGetPayments(t *testing.T) {
	const Limit = 3

	from := time.Date(2021, 01, 01, 0, 0, 0, 0, time.UTC)

	for _, lightningImpl := range []api.APIType{api.LndGrpc, api.LndRest} {
		itf := getAPI(t, "fixture.secret", lightningImpl)
		if itf == nil {
			return
		}

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Minute))
		defer cancel()

		channel := GetPayments(ctx, itf, from)
		num := 0
		for i := 0; i < Limit; i++ {
			data := <-channel
			//fmt.Printf("Received %v %s\n", data.Timestamp, string(data.Message))
			if data.Timestamp.Before(from) {
				t.Fatalf("Data has too old timestamp %v", data.Timestamp)
			}
			num++
		}

		assert.Equal(t, Limit, num)
	}

	//t.Fatalf("fail")
}
