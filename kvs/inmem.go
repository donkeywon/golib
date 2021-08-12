package kvs

import (
	"strconv"
	"sync"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/conv"
	"github.com/donkeywon/golib/util/jsonu"
)

func init() {
	plugin.RegisterWithCfg(TypeInMem, func() interface{} { return NewInMemKVS() }, func() interface{} { return NewInMemKVSCfg() })
}

const TypeInMem Type = "inmem"

type InMemKVSCfg struct{}

func NewInMemKVSCfg() *InMemKVSCfg {
	return &InMemKVSCfg{}
}

type InMemKVS struct {
	Cfg *InMemKVSCfg
	m   sync.Map
}

func NewInMemKVS() *InMemKVS {
	return &InMemKVS{}
}

func (b *InMemKVS) Store(k string, v any) {
	b.m.Store(k, v)
}

func (b *InMemKVS) StoreAsString(k string, v any) {
	b.m.Store(k, conv.AnyToString(v))
}

func (b *InMemKVS) Stores(m map[string]any) {
	if m == nil {
		return
	}

	for k, v := range m {
		b.Store(k, v)
	}
}

func (b *InMemKVS) Load(k string) (any, bool) {
	return b.m.Load(k)
}

func (b *InMemKVS) LoadOrStore(k string, v any) (any, bool) {
	return b.m.LoadOrStore(k, v)
}

func (b *InMemKVS) LoadAndDelete(k string) (any, bool) {
	return b.m.LoadAndDelete(k)
}

func (b *InMemKVS) Del(k string) {
	b.m.Delete(k)
}

func (b *InMemKVS) LoadAsBool(k string) bool {
	v, exists := b.Load(k)
	if !exists {
		return false
	}
	switch vt := v.(type) {
	case string:
		return vt == "true"
	case *string:
		return *vt == "true"
	case bool:
		return vt
	case *bool:
		return *vt
	default:
		panic("unexpected value type")
	}
}

func (b *InMemKVS) LoadAsString(k string) string {
	v, exists := b.Load(k)
	if !exists {
		return ""
	}
	return conv.AnyToString(v)
}

func (b *InMemKVS) LoadAsStringOr(k string, d string) string {
	v := b.LoadAsString(k)
	if v == "" {
		return d
	}
	return v
}

func (b *InMemKVS) LoadAsInt(k string) int {
	return b.LoadAsIntOr(k, 0)
}

func (b *InMemKVS) LoadAsIntOr(k string, d int) int {
	v, exists := b.Load(k)
	if !exists {
		return d
	}
	if v == nil {
		return d
	}
	switch vt := v.(type) {
	case string:
		i, err := strconv.Atoi(vt)
		if err != nil {
			panic(err)
		}
		return i
	case *string:
		i, err := strconv.Atoi(*vt)
		if err != nil {
			panic(err)
		}
		return i
	case int8:
		return int(vt)
	case *int8:
		return int(*vt)
	case int16:
		return int(vt)
	case *int16:
		return int(*vt)
	case int32:
		return int(vt)
	case *int32:
		return int(*vt)
	case int64:
		return int(vt)
	case *int64:
		return int(*vt)
	case int:
		return vt
	case *int:
		return *vt
	default:
		panic("unexpected value type")
	}
}

func (b *InMemKVS) LoadAsUint(k string) uint {
	return b.LoadAsUintOr(k, 0)
}

func (b *InMemKVS) LoadAsUintOr(k string, d uint) uint {
	v, exists := b.Load(k)
	if !exists {
		return d
	}
	if v == nil {
		return d
	}
	switch vt := v.(type) {
	case string:
		i, err := strconv.ParseUint(vt, 10, 0)
		if err != nil {
			panic(err)
		}
		return uint(i)
	case *string:
		i, err := strconv.ParseUint(*vt, 10, 0)
		if err != nil {
			panic(err)
		}
		return uint(i)
	case uint8:
		return uint(vt)
	case *uint8:
		return uint(*vt)
	case uint16:
		return uint(vt)
	case *uint16:
		return uint(*vt)
	case uint32:
		return uint(vt)
	case *uint32:
		return uint(*vt)
	case uint64:
		return uint(vt)
	case *uint64:
		return uint(*vt)
	case uint:
		return vt
	case *uint:
		return *vt
	default:
		panic("unexpected value type")
	}
}

func (b *InMemKVS) LoadAsFloat(k string) float64 {
	return b.LoadAsFloatOr(k, 0.0)
}

func (b *InMemKVS) LoadAsFloatOr(k string, d float64) float64 {
	v, exists := b.Load(k)
	if !exists {
		return d
	}
	if v == nil {
		return d
	}
	switch vt := v.(type) {
	case string:
		f, err := strconv.ParseFloat(vt, 64)
		if err != nil {
			panic(err)
		}
		return f
	case *string:
		f, err := strconv.ParseFloat(*vt, 64)
		if err != nil {
			panic(err)
		}
		return f
	case float32:
		return float64(vt)
	case *float32:
		return float64(*vt)
	case float64:
		return vt
	case *float64:
		return *vt
	default:
		panic("unexpected value type")
	}
}

func (b *InMemKVS) LoadTo(k string, to any) error {
	v := b.LoadAsString(k)
	if v == "" {
		return nil
	}

	return jsonu.UnmarshalString(v, to)
}

func (b *InMemKVS) Collect() map[string]any {
	c := make(map[string]any)
	b.Range(func(k string, v any) bool {
		c[k] = v
		return true
	})
	return c
}

func (b *InMemKVS) Range(f func(k string, v any) bool) {
	b.m.Range(func(key, value any) bool {
		return f(key.(string), value)
	})
}

func (b *InMemKVS) CollectAsString() map[string]string {
	result := make(map[string]string)
	b.Range(func(k string, v any) bool {
		result[k] = conv.AnyToString(v)
		return true
	})
	return result
}

func (b *InMemKVS) Close() error {
	return nil
}
