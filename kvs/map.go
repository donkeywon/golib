package kvs

import (
	"sync"
)

type Map[K comparable, V any] struct {
	m    map[K]V
	mu   sync.RWMutex
	once sync.Once
}

func (m *Map[K, V]) init() {
	m.once.Do(func() {
		m.m = make(map[K]V)
	})
}

func (m *Map[K, V]) Store(k K, v V) {
	m.init()

	m.mu.Lock()
	m.m[k] = v
	m.mu.Unlock()
}

func (m *Map[K, V]) Load(k K) (V, bool) {
	m.init()

	m.mu.RLock()
	v, exists := m.m[k]
	m.mu.RUnlock()
	return v, exists
}

func (m *Map[K, V]) LoadOrStore(k K, v V) (V, bool) {
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

func (m *Map[K, V]) LoadAndDelete(k K) (V, bool) {
	m.init()

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

func (m *Map[K, V]) Delete(k K) {
	m.init()

	m.mu.Lock()
	delete(m.m, k)
	m.mu.Unlock()
}

func (m *Map[K, V]) Range(f func(k K, v V) bool) {
	m.init()

	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.m {
		if !f(k, v) {
			break
		}
	}
}
