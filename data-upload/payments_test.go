package raw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	common_entities "github.com/bolt-observer/agent/entities"

	"github.com/bolt-observer/go_common/entities"
	"github.com/stretchr/testify/assert"

	"github.com/procodr/monkey"

	api "github.com/bolt-observer/agent/lightning"
)

const FailNoCredsPayments = false

func getAPI(t *testing.T, name string, typ api.APIType) api.LightingAPICalls {
	var data entities.Data

	if _, err := os.Stat(name); errors.Is(err, os.ErrNotExist) {
		if FailNoCredsPayments {
			t.Fatalf("No credentials")
		}
		return nil
	}

	content, err := os.ReadFile(name)
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

	api, _ := api.NewAPI(typ, func() (*entities.Data, error) {
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

		channel := GetInvoicesChannel(ctx, func() (api.LightingAPICalls, error) {
			return itf, nil
		}, from)
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

		channel := GetForwardsChannel(ctx, func() (api.LightingAPICalls, error) {
			return itf, nil
		}, from)
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

		channel := GetPaymentsChannel(ctx, func() (api.LightingAPICalls, error) {
			return itf, nil
		}, from)
		num := 0
		for i := 0; i < Limit; i++ {
			data := <-channel
			if data.Timestamp.Before(from) {
				t.Fatalf("Data has too old timestamp %v", data.Timestamp)
			}
			num++
		}

		assert.Equal(t, Limit, num)
	}

	//t.Fatalf("fail")
}

func TestPaginatorSimple(t *testing.T) {

	min := int64(1674047551)
	outchan := make(chan api.RawMessage)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()

	f := func(ctx context.Context, lightning common_entities.NewAPICall, pagination api.RawPagination) ([]api.RawMessage, *api.ResponseRawPagination, error) {
		return []api.RawMessage{
				{Timestamp: time.Unix(min-1, 0), Implementation: "A"},
				{Timestamp: time.Unix(min-1, 0), Implementation: "B"},
				{Timestamp: time.Unix(min, 0), Implementation: "C"},
				{Timestamp: time.Unix(min, 0), Implementation: "D"},
				{Timestamp: time.Unix(min+1, 0), Implementation: "E"},
				{Timestamp: time.Unix(min+1, 0), Implementation: "F"},
			},
			&api.ResponseRawPagination{FirstTime: time.Unix(min-1, 0), LastTime: time.Unix(min+1, 0)}, nil
	}

	go paginator(ctx, nil, f, time.Unix(min, 0), -1, outchan)

	data := <-outchan
	assert.Equal(t, data.Implementation, "C")
	data = <-outchan
	assert.Equal(t, data.Implementation, "D")
	data = <-outchan
	assert.Equal(t, data.Implementation, "E")
	data = <-outchan
	assert.Equal(t, data.Implementation, "F")

	cancel()
}

func TestPaginatorSplit(t *testing.T) {

	min := int64(1674047551)
	outchan := make(chan api.RawMessage)

	step := 0

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()

	type ctxKey struct{}

	ctx = context.WithValue(ctx, ctxKey{}, &step)

	f := func(ctx context.Context, lightning common_entities.NewAPICall, pagination api.RawPagination) ([]api.RawMessage, *api.ResponseRawPagination, error) {

		ptr, ok := ctx.Value(ctxKey{}).(*int)
		if !ok {
			return nil, nil, fmt.Errorf("error in step")
		}

		val := *ptr
		*ptr = val + 1

		switch val {
		case 0:
			return []api.RawMessage{
					{Timestamp: time.Unix(min-1, 0), Implementation: "A"},
					{Timestamp: time.Unix(min-1, 0), Implementation: "B"},
				},
				&api.ResponseRawPagination{FirstTime: time.Unix(min-1, 0), LastTime: time.Unix(min-1, 0)}, nil
		case 1:
			return []api.RawMessage{
					{Timestamp: time.Unix(min, 0), Implementation: "C"},
					{Timestamp: time.Unix(min, 0), Implementation: "D"},
				},
				&api.ResponseRawPagination{FirstTime: time.Unix(min, 0), LastTime: time.Unix(min, 0)}, nil
		case 2:
			return []api.RawMessage{
					{Timestamp: time.Unix(min+1, 0), Implementation: "E"},
					{Timestamp: time.Unix(min+1, 0), Implementation: "F"},
				},
				&api.ResponseRawPagination{FirstTime: time.Unix(min+1, 0), LastTime: time.Unix(min+1, 0)}, nil
		}

		return nil, nil, fmt.Errorf("unreached")
	}

	go paginator(ctx, nil, f, time.Unix(min, 0), -1, outchan)

	data := <-outchan
	assert.Equal(t, data.Implementation, "C")
	data = <-outchan
	assert.Equal(t, data.Implementation, "D")
	data = <-outchan
	assert.Equal(t, data.Implementation, "E")
	data = <-outchan
	assert.Equal(t, data.Implementation, "F")

	cancel()
}
