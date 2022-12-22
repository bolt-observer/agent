package channelchecker

import (
	"testing"

	"github.com/bolt-observer/agent/entities"
	api "github.com/bolt-observer/agent/lightningapi"
	"github.com/bolt-observer/go_common/utils"
)

func getAPI() api.LightingApiCalls {
	return nil
}

func TestDeleteInTheMiddle(t *testing.T) {

	settings := NewGlobalSettings()

	settings.Set("burek", Settings{identifier: entities.NodeIdentifier{Identifier: "1337", UniqueId: "1337"}, getAPI: getAPI})

	if !utils.Contains(settings.GetKeys(), "burek") {
		t.Fatalf("Element not present")
	}

	s := settings.Get("burek")
	if s.identifier.UniqueId != "1337" {
		t.Fatalf("Wrong stuff returned")
	}

	s.getAPI()

	settings.Delete("burek")
	if s.identifier.UniqueId != "1337" {
		t.Fatalf("Wrong stuff returned")
	}
}
