package runner

import (
	"strconv"
	"sync"

	"github.com/donkeywon/golib/util/conv"
	"github.com/donkeywon/golib/util/jsonu"
)

type kvs interface {
	Store(k string, v any)
	StoreAsString(k string, v any)
	Load(k string) (any, bool)
	LoadOrStore(k string, v any) (any, bool)
	LoadAndDelete(k string) (any, bool)
	DelKey(k string)
	LoadAsBool(k string) bool
	LoadAsString(k string) string
	LoadAsStringOr(k string, d string) string
	LoadAsInt(k string) int
	LoadAsIntOr(k string, d int) int
	LoadAsUint(k string) uint
	LoadAsUintOr(k string, d uint) uint
	LoadAsFloat(k string) float64
	LoadAsFloatOr(k string, d float64) float64
	LoadTo(k string, to any) error
	Collect() map[string]any
	CollectAsString() map[string]string
	Range(func(k string, v any) bool)
	StoreValues(map[string]any)
}

type simpleInMemKvs struct {
	m sync.Map
}

func newSimpleInMemKvs() kvs {
	return &simpleInMemKvs{
		m: sync.Map{},
	}
}

func (b *simpleInMemKvs) Store(k string, v any) {
	b.m.Store(k, v)
}

func (b *simpleInMemKvs) StoreAsString(k string, v any) {
	b.m.Store(k, conv.AnyToString(v))
}

func (b *simpleInMemKvs) Load(k string) (any, bool) {
	return b.m.Load(k)
}

func (b *simpleInMemKvs) LoadOrStore(k string, v any) (any, bool) {
	return b.m.LoadOrStore(k, v)
}

func (b *simpleInMemKvs) LoadAndDelete(k string) (any, bool) {
	return b.m.LoadAndDelete(k)
}

func (b *simpleInMemKvs) DelKey(k string) {
	b.m.Delete(k)
}

func (b *simpleInMemKvs) LoadAsBool(k string) bool {
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

func (b *simpleInMemKvs) LoadAsString(k string) string {
	v, exists := b.Load(k)
	if !exists {
		return ""
	}
	return conv.AnyToString(v)
}

func (b *simpleInMemKvs) LoadAsStringOr(k string, d string) string {
	v := b.LoadAsString(k)
	if v == "" {
		return d
	}
	return v
}

func (b *simpleInMemKvs) LoadAsInt(k string) int {
	return b.LoadAsIntOr(k, 0)
}

func (b *simpleInMemKvs) LoadAsIntOr(k string, d int) int {
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

func (b *simpleInMemKvs) LoadAsUint(k string) uint {
	return b.LoadAsUintOr(k, 0)
}

func (b *simpleInMemKvs) LoadAsUintOr(k string, d uint) uint {
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

func (b *simpleInMemKvs) LoadAsFloat(k string) float64 {
	return b.LoadAsFloatOr(k, 0.0)
}

func (b *simpleInMemKvs) LoadAsFloatOr(k string, d float64) float64 {
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

func (b *simpleInMemKvs) LoadTo(k string, to any) error {
	v := b.LoadAsString(k)
	if v == "" {
		return nil
	}

	return jsonu.UnmarshalString(v, to)
}

func (b *simpleInMemKvs) Collect() map[string]any {
	c := make(map[string]any)
	b.Range(func(k string, v any) bool {
		c[k] = v
		return true
	})
	return c
}

func (b *simpleInMemKvs) Range(f func(k string, v any) bool) {
	b.m.Range(func(key, value any) bool {
		return f(key.(string), value)
	})
}

func (b *simpleInMemKvs) CollectAsString() map[string]string {
	result := make(map[string]string)
	b.Range(func(k string, v any) bool {
		result[k] = conv.AnyToString(v)
		return true
	})
	return result
}

func (b *simpleInMemKvs) StoreValues(m map[string]any) {
	if m == nil {
		return
	}

	for k, v := range m {
		b.Store(k, v)
	}
}
