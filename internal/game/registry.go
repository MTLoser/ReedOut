package game

import "sync"

var (
	mu       sync.RWMutex
	adapters = map[string]GameAdapter{}
)

func Register(adapter GameAdapter) {
	mu.Lock()
	defer mu.Unlock()
	adapters[adapter.Game()] = adapter
}

func Get(game string) GameAdapter {
	mu.RLock()
	defer mu.RUnlock()
	return adapters[game]
}

func All() map[string]GameAdapter {
	mu.RLock()
	defer mu.RUnlock()
	result := make(map[string]GameAdapter, len(adapters))
	for k, v := range adapters {
		result[k] = v
	}
	return result
}
