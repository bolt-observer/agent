package boltz

import (
	"time"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/bolt-observer/agent/entities"
	agent_entities "github.com/bolt-observer/agent/entities"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/golang/glog"
)

const (
	DefaultBoltzUrl = "https://boltz.exchange/api"
)

// Plugin can save its data here
type Plugin struct {
	BoltzAPI    *boltz.Boltz
	ChainParams *chaincfg.Params
	LnAPI       entities.NewAPICall

	agent_entities.Plugin
}

// NewBoltzPlugin creates new instance
func NewBoltzPlugin(lnAPI entities.NewAPICall, network string, boltzUrl string) *Plugin {
	params := chaincfg.MainNetParams
	switch network {
	case "mainnet":
		params = chaincfg.MainNetParams
	case "testnet":
		params = chaincfg.TestNet3Params
	case "regtest":
		params = chaincfg.RegressionNetParams
	case "simnet":
		params = chaincfg.SimNetParams
	}

	if boltzUrl == "" {
		boltzUrl = params.DefaultPort
	}

	resp := &Plugin{
		ChainParams: &params,
		BoltzAPI: &boltz.Boltz{
			URL: boltzUrl,
		},
	}
	if lnAPI == nil {
		return nil
	}

	resp.LnAPI = lnAPI
	return resp
}

// Execute is currently just mocked
func (b *Plugin) Execute(jobID int32, data []byte, msgCallback func(agent_entities.PluginMessage) error, isDryRun bool) error {
	go func() {
		// LOG
		<-time.After(2 * time.Second)
		glog.Infof("Swap executed, sending to api")
		msgCallback(agent_entities.PluginMessage{
			JobID:      jobID,
			Message:    "Swap executed, waiting for confirmation",
			Data:       []byte(`{"fee": "123124"}`),
			IsError:    false,
			IsFinished: false,
		})

		// FINISHED
		<-time.After(4 * time.Second)
		glog.Infof("Swap finished, sending to api")
		msgCallback(agent_entities.PluginMessage{
			JobID:      jobID,
			Message:    "Swap finished successfully",
			Data:       []byte(`{"swap_id": "123124"}`),
			IsError:    false,
			IsFinished: true,
		})
	}()

	// TODO
	// check in memory if job is already running
	// check DB if job is already running -> load to memory
	// create new job
	return nil
}
