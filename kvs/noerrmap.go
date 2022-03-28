package kvs

import (
	"sync"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/conv"
)

func init() {
	plugin.RegWithCfg(TypeMap, NewMapKVS, func() any { return NewMapKVSCfg() })
}

type MapKVSCfg struct{}

func NewMapKVSCfg() *MapKVSCfg {
	return &MapKVSCfg{}
}

const TypeMap Type = "map"

type MapKVS struct {
	*MapKVSCfg
	m sync.Map
}

func NewMapKVS() *MapKVS {
	return &MapKVS{}
}

func (m *MapKVS) Type() Type {
	return TypeMap
}

func (m *MapKVS) Store(k string, v any) {
	m.m.Store(k, v)
}

func (m *MapKVS) StoreAsString(k string, v any) {
	s, err := conv.ToString(v)
	if err != nil {
		panic(err)
	}
	m.m.Store(k, s)
}

func (m *MapKVS) Load(k string) (any, bool) {
	return m.m.Load(k)
}

func (m *MapKVS) LoadOrStore(k string, v any) (any, bool) {
	return m.m.LoadOrStore(k, v)
}

func (m *MapKVS) LoadAndDelete(k string) (any, bool) {
	return m.m.LoadAndDelete(k)
}

func (m *MapKVS) Del(k string) {
	m.m.Delete(k)
}

func (m *MapKVS) LoadAsBool(k string) bool {
	return PLoadAsBool(m, k)
}

func (m *MapKVS) LoadAsString(k string) string {
	return PLoadAsString(m, k)
}

func (m *MapKVS) LoadAsStringOr(k string, d string) string {
	return PLoadAsStringOr(m, k, d)
}

func (m *MapKVS) LoadAsInt(k string) int {
	return PLoadAsInt(m, k)
}

func (m *MapKVS) LoadAsIntOr(k string, d int) int {
	return PLoadAsIntOr(m, k, d)
}

func (m *MapKVS) LoadAsUint(k string) uint {
	return PLoadAsUint(m, k)
}

func (m *MapKVS) LoadAsUintOr(k string, d uint) uint {
	return PLoadAsUintOr(m, k, d)
}

func (m *MapKVS) LoadAsFloat(k string) float64 {
	return PLoadAsFloat(m, k)
}

func (m *MapKVS) LoadAsFloatOr(k string, d float64) float64 {
	return PLoadAsFloatOr(m, k, d)
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
	m.m.Range(func(key, value any) bool {
		return f(key.(string), value)
	})
}
