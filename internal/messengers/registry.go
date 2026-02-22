package messengers

import (
	"fmt"
	"sync"
)

var (
	schemaRegistry = make(map[string]any)
	smu            sync.RWMutex
)

// RegisterSchema registers a messenger's config schema. Returns an error on duplicate.
func RegisterSchema(name string, schema any) error {
	smu.Lock()
	defer smu.Unlock()

	if _, exists := schemaRegistry[name]; exists {
		return fmt.Errorf("messenger schema '%s' is already registered", name)
	}
	schemaRegistry[name] = schema
	return nil
}

// GetAllSchemas returns a map of all registered messenger names to their config schemas.
func GetAllSchemas() map[string]any {
	smu.RLock()
	defer smu.RUnlock()

	result := make(map[string]any, len(schemaRegistry))
	for k, v := range schemaRegistry {
		result[k] = v
	}
	return result
}
