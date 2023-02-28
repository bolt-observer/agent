package entities

// Plugin is temporary interface for plugins. This will probably be replaced by gRPC interface
type Plugin interface {
	// Execute can be called multiple times for the same job
	// It's plugin responsibility to check if the job is already running and ignore such requests
	Execute(jobID int32, data []byte, msgCallback MessageCallback) error
}

// MessageCallback is callback function which is called by plugin to send message to connector
type MessageCallback func(PluginMessage) error

// PluginMessage is structure of message sent from plugin to connector
type PluginMessage struct {
	JobID      int32
	Message    string
	Data       []byte
	IsError    bool
	IsFinished bool
}
