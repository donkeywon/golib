package kvs

import (
	"github.com/donkeywon/golib/util/conv"
)

// NoErrKVS ignore KVS method returned error.
type NoErrKVS interface {
	Store(k string, v any)
	StoreAsString(k string, v any)
	Load(k string) (any, bool)
	LoadOrStore(k string, v any) (any, bool)
	LoadAndDelete(k string) (any, bool)
	Del(k string)
	LoadAsBool(k string) bool
	LoadAsString(k string) string
	LoadAsStringOr(k string, d string) string
	LoadAsInt(k string) int
	LoadAsIntOr(k string, d int) int
	LoadAsUint(k string) uint
	LoadAsUintOr(k string, d uint) uint
	LoadAsFloat(k string) float64
	LoadAsFloatOr(k string, d float64) float64
	LoadAll() map[string]any
	LoadAllAsString() map[string]string
	Range(func(k string, v any) bool)
}

func MustLoadAsBool(kvs NoErrKVS, k string) bool {
	v, exists := kvs.Load(k)
	if !exists {
		return false
	}
	vv, err := conv.ToBool(v)
	if err != nil {
		panic(err)
	}
	return vv
}

func MustLoadAsString(kvs NoErrKVS, k string) string {
	v, exists := kvs.Load(k)
	if !exists || v == nil {
		return ""
	}
	vv, err := conv.ToString(v)
	if err != nil {
		panic(err)
	}
	return vv
}

func MustLoadAsStringOr(kvs NoErrKVS, k string, d string) string {
	v := MustLoadAsString(kvs, k)
	if v == "" {
		return d
	}
	return v
}

func MustLoadAsInt(kvs NoErrKVS, k string) int {
	return MustLoadAsIntOr(kvs, k, 0)
}

func MustLoadAsIntOr(kvs NoErrKVS, k string, d int) int {
	v, exists := kvs.Load(k)
	if !exists || v == nil {
		return d
	}
	vv, err := conv.ToInt(v)
	if err != nil {
		panic(err)
	}
	return vv
}

func MustLoadAsUint(kvs NoErrKVS, k string) uint {
	return MustLoadAsUintOr(kvs, k, 0)
}

func MustLoadAsUintOr(kvs NoErrKVS, k string, d uint) uint {
	v, exists := kvs.Load(k)
	if !exists || v == nil {
		return d
	}
	vv, err := conv.ToUint(v)
	if err != nil {
		panic(err)
	}
	return vv
}

func MustLoadAsFloat(kvs NoErrKVS, k string) float64 {
	return MustLoadAsFloatOr(kvs, k, 0)
}

func MustLoadAsFloatOr(kvs NoErrKVS, k string, d float64) float64 {
	v, exists := kvs.Load(k)
	if !exists || v == nil {
		return d
	}
	vv, err := conv.ToFloat(v)
	if err != nil {
		panic(err)
	}
	return vv
}
