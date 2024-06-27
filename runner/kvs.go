package runner

import (
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/donkeywon/golib/util"
)

type kvs interface {
	Store(k string, v any)
	StoreAsString(k string, v any)
	GetValue(k string) any
	DelKey(k string)
	HasKey(k string) bool
	GetBoolValue(k string) bool
	GetStringValue(k string) string
	GetStringValueOr(k string, d string) string
	GetIntValue(k string) int
	GetIntValueOr(k string, d int) int
	GetUintValue(k string) uint
	GetUintValueOr(k string, d uint) uint
	GetFloatValue(k string) float64
	GetFloatValueOr(k string, d float64) float64
	GetValueTo(k string, to any) error
	Collect() map[string]any
	CollectBy(func(map[string]any))
	CollectAsString() map[string]string
	StoreValues(map[string]any)
}

type simpleInMemKvs struct {
	m  map[string]any
	mu sync.Mutex
}

func newSimpleInMemKvs() kvs {
	return &simpleInMemKvs{
		m: make(map[string]any),
	}
}

func (b *simpleInMemKvs) Store(k string, v any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.store(k, v)
}

func (b *simpleInMemKvs) store(k string, v any) {
	b.m[k] = v
}

func (b *simpleInMemKvs) StoreAsString(k string, v any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.storeAsString(k, v)
}

func (b *simpleInMemKvs) storeAsString(k string, v any) {
	b.store(k, convertToString(v))
}

func (b *simpleInMemKvs) GetValue(k string) any {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.m[k]
}

func (b *simpleInMemKvs) HasKey(k string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, exists := b.m[k]
	return exists
}

func (b *simpleInMemKvs) DelKey(k string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.m, k)
}

func (b *simpleInMemKvs) GetStringValue(k string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.m[k].(string)
}

func (b *simpleInMemKvs) GetStringValueOr(k string, d string) string {
	v := b.GetStringValue(k)
	if v == "" {
		return d
	}
	return v
}

func (b *simpleInMemKvs) GetBoolValue(k string) bool {
	v := b.GetValue(k)
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

func (b *simpleInMemKvs) GetIntValue(k string) int {
	return b.GetIntValueOr(k, 0)
}

func (b *simpleInMemKvs) GetIntValueOr(k string, d int) int {
	v := b.GetValue(k)
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

func (b *simpleInMemKvs) GetUintValue(k string) uint {
	return b.GetUintValueOr(k, 0)
}

func (b *simpleInMemKvs) GetUintValueOr(k string, d uint) uint {
	v := b.GetValue(k)
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

func (b *simpleInMemKvs) GetFloatValue(k string) float64 {
	return b.GetFloatValueOr(k, 0.0)
}

func (b *simpleInMemKvs) GetFloatValueOr(k string, d float64) float64 {
	v := b.GetValue(k)
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

func (b *simpleInMemKvs) GetValueTo(k string, to any) error {
	v := b.GetStringValue(k)
	if v == "" {
		return nil
	}

	return sonic.Unmarshal(util.String2Bytes(v), to)
}

func (b *simpleInMemKvs) Collect() map[string]any {
	c := make(map[string]any)
	b.CollectBy(func(m map[string]any) {
		for k, v := range m {
			c[k] = v
		}
	})
	return c
}

func (b *simpleInMemKvs) CollectBy(f func(map[string]any)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	f(b.m)
}

func (b *simpleInMemKvs) CollectAsString() map[string]string {
	result := make(map[string]string)
	b.CollectBy(func(m map[string]any) {
		for k, v := range m {
			result[k] = convertToString(v)
		}
	})
	return result
}

func (b *simpleInMemKvs) StoreValues(m map[string]any) {
	if m == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for k, v := range m {
		b.store(k, v)
	}
}

func convertToString(v any) string {
	var vs string

	switch vv := v.(type) {
	case string:
		vs = vv
	case *string:
		vs = *vv
	case []byte:
		vs = string(vv)
	case bool:
		vs = strconv.FormatBool(vv)
	case *bool:
		vs = strconv.FormatBool(*vv)
	case complex128:
		vs = strconv.FormatComplex(vv, 'f', 10, 128)
	case *complex128:
		vs = strconv.FormatComplex(*vv, 'f', 10, 128)
	case complex64:
		vs = strconv.FormatComplex(complex128(vv), 'f', 10, 128)
	case *complex64:
		vs = strconv.FormatComplex(complex128(*vv), 'f', 10, 128)
	case float64:
		vs = strconv.FormatFloat(vv, 'f', 10, 64)
	case *float64:
		vs = strconv.FormatFloat(*vv, 'f', 10, 64)
	case float32:
		vs = strconv.FormatFloat(float64(vv), 'f', 10, 64)
	case *float32:
		vs = strconv.FormatFloat(float64(*vv), 'f', 10, 64)
	case int:
		vs = strconv.FormatInt(int64(vv), 10)
	case *int:
		vs = strconv.FormatInt(int64(*vv), 10)
	case int64:
		vs = strconv.FormatInt(vv, 10)
	case *int64:
		vs = strconv.FormatInt(*vv, 10)
	case int32:
		vs = strconv.FormatInt(int64(vv), 10)
	case *int32:
		vs = strconv.FormatInt(int64(*vv), 10)
	case int16:
		vs = strconv.FormatInt(int64(vv), 10)
	case *int16:
		vs = strconv.FormatInt(int64(*vv), 10)
	case int8:
		vs = strconv.FormatInt(int64(vv), 10)
	case *int8:
		vs = strconv.FormatInt(int64(*vv), 10)
	case uint:
		vs = strconv.FormatUint(uint64(vv), 10)
	case *uint:
		vs = strconv.FormatUint(uint64(*vv), 10)
	case uint64:
		vs = strconv.FormatUint(vv, 10)
	case *uint64:
		vs = strconv.FormatUint(*vv, 10)
	case uint32:
		vs = strconv.FormatUint(uint64(vv), 10)
	case *uint32:
		vs = strconv.FormatUint(uint64(*vv), 10)
	case uint16:
		vs = strconv.FormatUint(uint64(vv), 10)
	case *uint16:
		vs = strconv.FormatUint(uint64(*vv), 10)
	case uint8:
		vs = strconv.FormatUint(uint64(vv), 10)
	case *uint8:
		vs = strconv.FormatUint(uint64(*vv), 10)
	case uintptr:
		vs = strconv.FormatInt(int64(vv), 16)
	case *uintptr:
		vs = strconv.FormatInt(int64(*vv), 16)
	case time.Time:
		vs = vv.String()
	case *time.Time:
		vs = vv.String()
	case time.Duration:
		vs = vv.String()
	case *time.Duration:
		vs = vv.String()
	default:
		vs, _ = sonic.MarshalString(vv)
	}

	return vs
}
