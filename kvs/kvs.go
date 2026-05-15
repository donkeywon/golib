package kvs

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/conv"
)

type Type string

type Cfg struct {
	Type Type `yaml:"type" json:"type"`
	Cfg  any  `yaml:"cfg"  json:"cfg"`
}

type KVS interface {
	Store(k string, v any) error
	Load(k string) (any, bool, error)
	LoadOrStore(k string, v any) (any, bool, error)
	LoadAndDelete(k string) (any, bool, error)
	Del(k string) error
	Range(func(k string, v any) bool) error
}

func StoreAsString(kvs KVS, k string, v any) error {
	s, err := conv.ToString(v)
	if err != nil {
		return err
	}
	return kvs.Store(k, s)
}

func MustStoreAsString(kvs KVS, k string, v any) {
	if err := StoreAsString(kvs, k, v); err != nil {
		panic(err)
	}
}

func LoadAsBool(kvs KVS, k string) (bool, bool, error) {
	v, exists, err := kvs.Load(k)
	if err != nil {
		return false, false, err
	}
	if !exists {
		return false, false, nil
	}
	vv, err := conv.ToBool(v)
	if err != nil {
		return false, true, errs.Wrap(err, "convert value to bool failed")
	}
	return vv, true, nil
}

func LoadAsString(kvs KVS, k string) (string, bool, error) {
	return LoadAsStringOr(kvs, k, "")
}

func LoadAsStringOr(kvs KVS, k string, d string) (string, bool, error) {
	v, exists, err := kvs.Load(k)
	if err != nil {
		return "", false, err
	}
	if !exists {
		return d, false, nil
	}
	if v == nil {
		return "", true, nil
	}
	vv, err := conv.ToString(v)
	if err != nil {
		return "", true, errs.Wrap(err, "convert value to string failed")
	}
	return vv, true, nil
}

func LoadAsInt(kvs KVS, k string) (int, bool, error) {
	return LoadAsIntOr(kvs, k, 0)
}

func LoadAsIntOr(kvs KVS, k string, d int) (int, bool, error) {
	v, exists, err := kvs.Load(k)
	if err != nil {
		return 0, false, err
	}
	if !exists {
		return d, false, nil
	}
	if v == nil {
		return 0, true, nil
	}
	vv, err := conv.ToInt(v)
	if err != nil {
		return 0, true, errs.Wrap(err, "convert value to int failed")
	}
	return vv, true, nil
}

func LoadAsUint(kvs KVS, k string) (uint, bool, error) {
	return LoadAsUintOr(kvs, k, 0)
}

func LoadAsUintOr(kvs KVS, k string, d uint) (uint, bool, error) {
	v, exists, err := kvs.Load(k)
	if err != nil {
		return 0, false, err
	}
	if !exists {
		return d, false, nil
	}
	if v == nil {
		return 0, true, nil
	}
	vv, err := conv.ToUint(v)
	if err != nil {
		return 0, true, errs.Wrap(err, "convert value to uint failed")
	}
	return vv, true, nil
}

func LoadAsFloat(kvs KVS, k string) (float64, bool, error) {
	return LoadAsFloatOr(kvs, k, 0)
}

func LoadAsFloatOr(kvs KVS, k string, d float64) (float64, bool, error) {
	v, exists, err := kvs.Load(k)
	if err != nil {
		return 0, false, err
	}
	if !exists {
		return d, false, nil
	}
	if v == nil {
		return 0, true, nil
	}
	vv, err := conv.ToFloat(v)
	if err != nil {
		return 0, true, errs.Wrap(err, "convert value to float64 failed")
	}
	return vv, true, nil
}

func LoadAllAsString(kvs KVS) (map[string]string, error) {
	var err error
	result := make(map[string]string)
	kvs.Range(func(k string, v any) bool {
		result[k], err = conv.ToString(v)
		if err != nil {
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func MustLoadAsBool(kvs KVS, k string) (bool, bool) {
	v, exists, err := LoadAsBool(kvs, k)
	if err != nil {
		panic(err)
	}
	return v, exists
}

func MustLoadAsString(kvs KVS, k string) (string, bool) {
	return MustLoadAsStringOr(kvs, k, "")
}

func MustLoadAsStringOr(kvs KVS, k string, d string) (string, bool) {
	v, exists, err := LoadAsStringOr(kvs, k, d)
	if err != nil {
		panic(err)
	}
	return v, exists
}

func MustLoadAsInt(kvs KVS, k string) (int, bool) {
	return MustLoadAsIntOr(kvs, k, 0)
}

func MustLoadAsIntOr(kvs KVS, k string, d int) (int, bool) {
	v, exists, err := LoadAsIntOr(kvs, k, d)
	if err != nil {
		panic(err)
	}
	return v, exists
}

func MustLoadAsUint(kvs KVS, k string) (uint, bool) {
	return MustLoadAsUintOr(kvs, k, 0)
}

func MustLoadAsUintOr(kvs KVS, k string, d uint) (uint, bool) {
	v, exists, err := LoadAsUintOr(kvs, k, d)
	if err != nil {
		panic(err)
	}
	return v, exists
}

func MustLoadAsFloat(kvs KVS, k string) (float64, bool) {
	return MustLoadAsFloatOr(kvs, k, 0)
}

func MustLoadAsFloatOr(kvs KVS, k string, d float64) (float64, bool) {
	v, exists, err := LoadAsFloatOr(kvs, k, d)
	if err != nil {
		panic(err)
	}
	return v, exists
}

func MustLoadAllAsString(kvs KVS) map[string]string {
	result, err := LoadAllAsString(kvs)
	if err != nil {
		panic(err)
	}
	return result
}
