package kvs

import (
	"sync"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/conv"
)

func init() {
	plugin.Reg(TypeMap, NewMapKVS, func() MapKVSCfg { return NewMapKVSCfg() })
}

type MapKVSCfg struct{}

func NewMapKVSCfg() MapKVSCfg {
	return MapKVSCfg{}
}

const TypeMap Type = "map"

type MapKVS struct {
	*MapKVSCfg
	m  map[string]any
	mu sync.RWMutex
}

func NewMapKVS() *MapKVS {
	return &MapKVS{
		m: make(map[string]any),
	}
}

func (m *MapKVS) Store(k string, v any) {
	m.mu.Lock()
	m.m[k] = v
	m.mu.Unlock()
}

func (m *MapKVS) StoreAsString(k string, v any) {
	s, err := conv.ToString(v)
	if err != nil {
		panic(err)
	}
	m.Store(k, s)
}

func (m *MapKVS) Load(k string) (any, bool) {
	m.mu.RLock()
	v, exists := m.m[k]
	m.mu.RUnlock()
	return v, exists
}

func (m *MapKVS) LoadOrStore(k string, v any) (any, bool) {
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

func (m *MapKVS) LoadAndDelete(k string) (any, bool) {
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

func (m *MapKVS) Del(k string) {
	m.mu.Lock()
	delete(m.m, k)
	m.mu.Unlock()
}

func (m *MapKVS) LoadAsBool(k string) bool {
	return MustLoadAsBool(m, k)
}

func (m *MapKVS) LoadAsString(k string) string {
	return MustLoadAsString(m, k)
}

func (m *MapKVS) LoadAsStringOr(k string, d string) string {
	return MustLoadAsStringOr(m, k, d)
}

func (m *MapKVS) LoadAsInt(k string) int {
	return MustLoadAsInt(m, k)
}

func (m *MapKVS) LoadAsIntOr(k string, d int) int {
	return MustLoadAsIntOr(m, k, d)
}

func (m *MapKVS) LoadAsUint(k string) uint {
	return MustLoadAsUint(m, k)
}

func (m *MapKVS) LoadAsUintOr(k string, d uint) uint {
	return MustLoadAsUintOr(m, k, d)
}

func (m *MapKVS) LoadAsFloat(k string) float64 {
	return MustLoadAsFloat(m, k)
}

func (m *MapKVS) LoadAsFloatOr(k string, d float64) float64 {
	return MustLoadAsFloatOr(m, k, d)
}

func (m *MapKVS) LoadAll() map[string]any {
	c := make(map[string]any)
	m.Range(func(k string, v any) bool {
		c[k] = v
		return true
	})
	return c
}

func (m *MapKVS) LoadAllAsString() map[string]string {
	var err error
	result := make(map[string]string)
	m.Range(func(k string, v any) bool {
		result[k], err = conv.ToString(v)
		if err != nil {
			panic(err)
		}
		return true
	})
	return result
}

func (m *MapKVS) Range(f func(k string, v any) bool) {
	m.mu.RLock()
	for k, v := range m.m {
		if !f(k, v) {
			break
		}
	}
	m.mu.RUnlock()
}
