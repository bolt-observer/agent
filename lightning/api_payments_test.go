package lightning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	entities "github.com/bolt-observer/go_common/entities"
)

func getAPI(t *testing.T, name string, typ APIType) LightingAPICalls {
	var data entities.Data

	if _, err := os.Stat(name); errors.Is(err, os.ErrNotExist) {
		// If file with credentials does not exist succeed
		return nil
	}

	content, err := ioutil.ReadFile(name)
	if err != nil {
		t.Fatalf("Error when opening file: %v", err)
		return nil
	}

	if _, err := os.Stat(FixtureDir); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(FixtureDir, os.ModePerm)
		if err != nil {
			t.Fatalf("Could not create directory: %v", err)
			return nil
		}
	}

	err = json.Unmarshal(content, &data)
	if err != nil {
		t.Fatalf("Error during Unmarshal(): %v", err)
		return nil
	}

	api := NewAPI(typ, func() (*entities.Data, error) {
		return &data, nil
	})

	return api
}

func TestGetInvoicesRest(t *testing.T) {

	api := getAPI(t, "fixture.secret", LndRest)
	if api == nil {
		return
	}

	resp, err := api.GetInvoices(context.Background(), false, Pagination{Reversed: true, Num: 10})
	if err != nil {
		t.Fatalf("Error received %v\n", err)
	}

	fmt.Printf("%+v\n", resp)

	//t.Fail()
}

func TestGetInvoicesGrpc(t *testing.T) {

	api := getAPI(t, "fixture-grpc.secret", LndGrpc)
	if api == nil {
		return
	}

	resp, err := api.GetInvoices(context.Background(), false, Pagination{Reversed: true, Num: 10})
	if err != nil {
		t.Fatalf("Error received %v\n", err)
	}

	fmt.Printf("%+v\n", resp)

	//t.Fail()
}