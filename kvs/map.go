package kvs

import (
	"sync"
)

type MapKVS struct {
	m  map[any]any
	mu sync.RWMutex
}

func (m *MapKVS) init() {
	if m.m != nil {
		return
	}
	m.mu.Lock()
	m.m = make(map[any]any)
	m.mu.Unlock()
}

func (m *MapKVS) Store(k any, v any) {
	m.init()
	m.mu.Lock()
	m.m[k] = v
	m.mu.Unlock()
}

func (m *MapKVS) Load(k any) (any, bool) {
	m.init()
	m.mu.RLock()
	v, exists := m.m[k]
	m.mu.RUnlock()
	return v, exists
}

func (m *MapKVS) LoadOrStore(k any, v any) (any, bool) {
	m.init()
	m.mu.RLock()
	vv, exists := m.m[k]
	m.mu.RUnlock()
	if exists {
		return vv, true
	}

	m.mu.Lock()
	vv, exists = m.m[k]
	if exists {
		m.mu.Unlock()
		return vv, true
	}
	m.m[k] = v

	m.mu.Unlock()
	return v, false
}

func (m *MapKVS) LoadAndDelete(k any) (any, bool) {
	m.init()
	m.mu.Lock()
	v, exists := m.m[k]
	if !exists {
		m.mu.Unlock()
		return nil, false
	}

	delete(m.m, k)

	m.mu.Unlock()
	return v, true
}

func (m *MapKVS) Delete(k any) {
	m.init()
	m.mu.Lock()
	delete(m.m, k)
	m.mu.Unlock()
}

func (m *MapKVS) Range(f func(k any, v any) bool) {
	m.init()
	m.mu.RLock()
	for k, v := range m.m {
		if !f(k, v) {
			break
		}
	}
	m.mu.RUnlock()
}
