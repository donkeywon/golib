package kvs

import (
	"sync"
)

type SyncMap[K comparable, V any] struct {
	m sync.Map
}

func (s *SyncMap[K, V]) Store(k K, v V) {
	s.m.Store(k, v)
}

func (s *SyncMap[K, V]) Load(k K) (V, bool) {
	val, ok := s.m.Load(k)
	if !ok {
		var zero V
		return zero, false
	}
	return val.(V), true
}

func (s *SyncMap[K, V]) LoadOrStore(k K, v V) (V, bool) {
	actual, loaded := s.m.LoadOrStore(k, v)
	return actual.(V), loaded
}

func (s *SyncMap[K, V]) LoadAndDelete(k K) (V, bool) {
	val, loaded := s.m.LoadAndDelete(k)
	if !loaded {
		var zero V
		return zero, false
	}
	return val.(V), loaded
}

func (s *SyncMap[K, V]) Delete(k K) {
	s.m.Delete(k)
}

func (s *SyncMap[K, V]) Range(f func(k K, v V) bool) {
	s.m.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}
