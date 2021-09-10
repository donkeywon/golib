package kvs

import (
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/conv"
)

// NoErrKVS ignore KVS method returned error.
type NoErrKVS interface {
	plugin.Plugin

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
	Collect() map[string]any
	CollectAsString() map[string]string
	Range(func(k string, v any) bool)
}

func PLoadAsBool(kvs NoErrKVS, k string) bool {
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

func PLoadAsString(kvs NoErrKVS, k string) string {
	v, exists := kvs.Load(k)
	if !exists || v == nil {
		return ""
	}
	vv, err := conv.AnyToString(v)
	if err != nil {
		panic(err)
	}
	return vv
}

func PLoadAsStringOr(kvs NoErrKVS, k string, d string) string {
	v := PLoadAsString(kvs, k)
	if v == "" {
		return d
	}
	return v
}

func PLoadAsInt(kvs NoErrKVS, k string) int {
	return PLoadAsIntOr(kvs, k, 0)
}

func PLoadAsIntOr(kvs NoErrKVS, k string, d int) int {
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

func PLoadAsUint(kvs NoErrKVS, k string) uint {
	return PLoadAsUintOr(kvs, k, 0)
}

func PLoadAsUintOr(kvs NoErrKVS, k string, d uint) uint {
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

func PLoadAsFloat(kvs NoErrKVS, k string) float64 {
	return PLoadAsFloatOr(kvs, k, 0)
}

func PLoadAsFloatOr(kvs NoErrKVS, k string, d float64) float64 {
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
