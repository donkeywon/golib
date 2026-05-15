package kvs

import (
	"sync"

	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.Reg[KVS, any](TypeMap, func() KVS { return NewMapKVS() }, nil)
}

const TypeMap Type = "map"

type MapKVS struct {
	m  map[string]any
	mu sync.RWMutex
}

func NewMapKVS() *MapKVS {
	return &MapKVS{
		m: make(map[string]any),
	}
}

func (m *MapKVS) Store(k string, v any) error {
	m.mu.Lock()
	m.m[k] = v
	m.mu.Unlock()
	return nil
}

func (m *MapKVS) Load(k string) (any, bool, error) {
	m.mu.RLock()
	v, exists := m.m[k]
	m.mu.RUnlock()
	return v, exists, nil
}

func (m *MapKVS) LoadOrStore(k string, v any) (any, bool, error) {
	m.mu.RLock()
	vv, exists := m.m[k]
	m.mu.RUnlock()
	if exists {
		return vv, true, nil
	}

	m.mu.Lock()
	vv, exists = m.m[k]
	if exists {
		m.mu.Unlock()
		return vv, true, nil
	}
	m.m[k] = v

	m.mu.Unlock()
	return v, false, nil
}

func (m *MapKVS) LoadAndDelete(k string) (any, bool, error) {
	m.mu.Lock()

	v, exists := m.m[k]
	if !exists {
		m.mu.Unlock()
		return nil, false, nil
	}

	delete(m.m, k)

	m.mu.Unlock()
	return v, true, nil
}

func (m *MapKVS) Del(k string) error {
	m.mu.Lock()
	delete(m.m, k)
	m.mu.Unlock()
	return nil
}

func (m *MapKVS) LoadAll() map[string]any {
	m.mu.RLock()
	result := make(map[string]any, len(m.m))
	for k, v := range m.m {
		result[k] = v
	}
	m.mu.RUnlock()
	return result
}

func (m *MapKVS) Range(f func(k string, v any) bool) error {
	m.mu.RLock()
	for k, v := range m.m {
		if !f(k, v) {
			break
		}
	}
	m.mu.RUnlock()
	return nil
}
