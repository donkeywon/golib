package kvs

import (
	"sync"
)

type SyncMapKVS struct {
	m sync.Map
}

func (s *SyncMapKVS) Store(k, v any) {
	s.m.Store(k, v)
}

func (s *SyncMapKVS) Load(k any) (any, bool) {
	return s.m.Load(k)
}

func (s *SyncMapKVS) LoadOrStore(k, v any) (any, bool) {
	return s.m.LoadOrStore(k, v)
}

func (s *SyncMapKVS) LoadAndDelete(k any) (any, bool) {
	return s.m.LoadAndDelete(k)
}

func (s *SyncMapKVS) Delete(k any) {
	s.m.Delete(k)
}

func (s *SyncMapKVS) Range(f func(k, v any) bool) {
	s.m.Range(func(k, v any) bool {
		return f(k, v)
	})
}
