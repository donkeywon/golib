package kvs

import (
	"sync"
)

type MapKVS[K comparable, V any] struct {
	m    map[K]V
	mu   sync.RWMutex
	once sync.Once
}

func NewMapKVS[K comparable, V any]() *MapKVS[K, V] {
	return &MapKVS[K, V]{
		m: make(map[K]V),
	}
}

func (m *MapKVS[K, V]) Store(k K, v V) {
	m.mu.Lock()
	m.m[k] = v
	m.mu.Unlock()
}

func (m *MapKVS[K, V]) Load(k K) (V, bool) {
	m.mu.RLock()
	v, exists := m.m[k]
	m.mu.RUnlock()
	return v, exists
}

func (m *MapKVS[K, V]) LoadOrStore(k K, v V) (V, bool) {
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

func (m *MapKVS[K, V]) LoadAndDelete(k K) (V, bool) {
	m.mu.Lock()
	v, exists := m.m[k]
	if !exists {
		m.mu.Unlock()
		var zero V
		return zero, false
	}
	delete(m.m, k)

	m.mu.Unlock()
	return v, true
}

func (m *MapKVS[K, V]) Delete(k K) {
	m.mu.Lock()
	delete(m.m, k)
	m.mu.Unlock()
}

func (m *MapKVS[K, V]) Range(f func(k K, v V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.m {
		if !f(k, v) {
			break
		}
	}
}
