package remoteclient

import (
	"fmt"
	"sync"

	"github.com/cvhariharan/flowctl/sdk/executor"
)

// NewRemoteClientFunc defines the signature for creating a new RemoteClient.
type NewRemoteClientFunc func(node executor.Node) (RemoteClient, error)

var (
	registry = make(map[string]NewRemoteClientFunc)
	mu       sync.RWMutex
)

// Register is called by remote client modules to make themselves available.
func Register(protocolName string, factory NewRemoteClientFunc) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[protocolName]; exists {
		panic(fmt.Sprintf("remote client for protocol '%s' is already registered", protocolName))
	}
	registry[protocolName] = factory
}

// GetClient is called by executors to get a client for a specific protocol.
func GetClient(protocolName string, node executor.Node) (RemoteClient, error) {
	mu.RLock()
	defer mu.RUnlock()
	factory, ok := registry[protocolName]
	if !ok {
		return nil, fmt.Errorf("remote client for protocol '%s' is not registered", protocolName)
	}
	return factory(node)
}
