package lightningapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

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

func TestGetForwardsGrpc(t *testing.T) {

	api := getAPI(t, "fixture-grpc.secret", LndGrpc)
	if api == nil {
		return
	}

	firstLastMonth := GetFirstDay(1)
	//beginOfThisMonth := getFirstDay(0)
	endOfThisMonth := GetFirstDay(-1).Add(-1 * time.Millisecond)

	out, stop := IterateForwardingHistory(api, firstLastMonth, endOfThisMonth, 100)
	for i := 0; i < 25; i++ {
		x := <-out
		fmt.Printf("%v\n", x)
	}

	stop <- true
	//t.Fail()
}

func TestGetForwardsRest(t *testing.T) {

	api := getAPI(t, "fixture.secret", LndRest)
	if api == nil {
		return
	}

	firstLastMonth := GetFirstDay(1)
	//beginOfThisMonth := getFirstDay(0)
	endOfThisMonth := GetFirstDay(-1).Add(-1 * time.Millisecond)

	out, stop := IterateForwardingHistory(api, firstLastMonth, endOfThisMonth, 100)
	for i := 0; i < 25; i++ {
		x := <-out
		fmt.Printf("%v\n", x)
	}

	stop <- true
	//t.Fail()
}
