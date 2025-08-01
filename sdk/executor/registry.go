package executor

import (
	"fmt"
	"regexp"
	"sync"
)

// NewExecutorFunc defines the signature for a function that can create an executor.
type NewExecutorFunc func(name string, node Node) (Executor, error)

var (
	registry       = make(map[string]NewExecutorFunc)
	schemaRegistry = make(map[string]interface{})
	mu             sync.RWMutex
	smu            sync.RWMutex
)

var validNameRegex = regexp.MustCompile(`^[a-zA-Z_]+$`)

func isValidName(name string) bool {
	return validNameRegex.MatchString(name)
}

// RegisterExecutor should be called by executor modules their init() function.
func RegisterExecutor(name string, factory NewExecutorFunc) {
	mu.Lock()
	defer mu.Unlock()

	if !isValidName(name) {
		panic("executor name can only include alphabets and underscore")
	}

	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("executor with name '%s' is already registered", name))
	}
	registry[name] = factory
}

// RegisterSchema should be called by executor modules to register their config schema
func RegisterSchema(name string, schema interface{}) {
	smu.Lock()
	defer smu.Unlock()

	if !isValidName(name) {
		panic("executor name can only include alphabets and underscore")
	}

	mu.RLock()
	if _, exists := registry[name]; !exists {
		panic(fmt.Sprintf("executor '%s' is not registered, cannot register schema", name))
	}
	mu.RUnlock()

	if _, exists := schemaRegistry[name]; exists {
		panic(fmt.Sprintf("schema with name '%s' is already registered", name))
	}
	schemaRegistry[name] = schema
}

// GetSchema returns the config schema of the executor
func GetSchema(name string) (interface{}, error) {
	smu.RLock()
	defer smu.RUnlock()

	schema, ok := schemaRegistry[name]
	if !ok {
		return nil, fmt.Errorf("schema for executor '%s' is not registered", name)
	}
	return schema, nil
}

// GetNewExecutorFunc is used to retrieve the NewExecutorFunc for an executor
func GetNewExecutorFunc(name string) (NewExecutorFunc, error) {
	mu.RLock()
	defer mu.RUnlock()

	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("executor '%s' is not registered or not included in the build", name)
	}
	return f, nil
}
