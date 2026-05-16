package kvs

import (
	"sync"
)

// SyncMapKVS is a type-safe generic key-value store backed by sync.Map.
// K must be comparable (required for map keys); V may be any type.
//
// Type assertions inside the methods are safe because Store is the only
// write path and it always stores values of type V.
type SyncMapKVS[K comparable, V any] struct {
	m sync.Map
}

func (s *SyncMapKVS[K, V]) Store(k K, v V) {
	s.m.Store(k, v)
}

func (s *SyncMapKVS[K, V]) Load(k K) (V, bool) {
	val, ok := s.m.Load(k)
	if !ok {
		var zero V
		return zero, false
	}
	return val.(V), true
}

func (s *SyncMapKVS[K, V]) LoadOrStore(k K, v V) (V, bool) {
	actual, loaded := s.m.LoadOrStore(k, v)
	return actual.(V), loaded
}

func (s *SyncMapKVS[K, V]) LoadAndDelete(k K) (V, bool) {
	val, loaded := s.m.LoadAndDelete(k)
	if !loaded {
		var zero V
		return zero, false
	}
	return val.(V), loaded
}

func (s *SyncMapKVS[K, V]) Delete(k K) {
	s.m.Delete(k)
}

func (s *SyncMapKVS[K, V]) Range(f func(k K, v V) bool) {
	s.m.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}
