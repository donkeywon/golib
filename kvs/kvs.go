package kvs

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/conv"
	"github.com/donkeywon/golib/util/jsonu"
)

type DBModel struct {
	RowID     int64  `db:"rowid"`
	K         string `db:"k"`
	V         string `db:"v"`
	UpdatedAt int64  `db:"updated_at"`
}

type Type string

type Cfg struct {
	Type Type        `yaml:"type" json:"type"`
	Cfg  interface{} `yaml:"cfg"  json:"cfg"`
}

type KVS interface {
	plugin.Plugin

	Open() error
	Close() error

	Store(k string, v any) error
	StoreAsString(k string, v any) error
	Load(k string) (any, bool, error)
	LoadOrStore(k string, v any) (any, bool, error)
	LoadAndDelete(k string) (any, bool, error)
	Del(k string) error
	LoadAsBool(k string) (bool, error)
	LoadAsString(k string) (string, error)
	LoadAsStringOr(k string, d string) (string, error)
	LoadAsInt(k string) (int, error)
	LoadAsIntOr(k string, d int) (int, error)
	LoadAsUint(k string) (uint, error)
	LoadAsUintOr(k string, d uint) (uint, error)
	LoadAsFloat(k string) (float64, error)
	LoadAsFloatOr(k string, d float64) (float64, error)
	LoadTo(k string, to any) error
	Collect() (map[string]any, error)
	CollectAsString() (map[string]string, error)
	Range(func(k string, v any) bool) error
}

func LoadAsBool(kvs KVS, k string) (bool, error) {
	v, exists, err := kvs.Load(k)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	vv, err := conv.ToBool(v)
	if err != nil {
		return false, errs.Wrap(err, "convert value to bool fail")
	}
	return vv, nil
}

func LoadAsString(kvs KVS, k string) (string, error) {
	v, exists, err := kvs.Load(k)
	if err != nil {
		return "", err
	}
	if !exists || v == nil {
		return "", nil
	}
	vv, err := conv.AnyToString(v)
	if err != nil {
		return "", errs.Wrap(err, "convert value to string fail")
	}
	return vv, nil
}

func LoadAsStringOr(kvs KVS, k string, d string) (string, error) {
	v, err := LoadAsString(kvs, k)
	if err != nil {
		return "", err
	}
	if v == "" {
		return d, nil
	}
	return v, nil
}

func LoadAsInt(kvs KVS, k string) (int, error) {
	return LoadAsIntOr(kvs, k, 0)
}

func LoadAsIntOr(kvs KVS, k string, d int) (int, error) {
	v, exists, err := kvs.Load(k)
	if err != nil {
		return 0, err
	}
	if !exists || v == nil {
		return d, nil
	}
	vv, err := conv.ToInt(v)
	if err != nil {
		return 0, errs.Wrap(err, "convert value to int fail")
	}
	return vv, nil
}

func LoadAsUint(kvs KVS, k string) (uint, error) {
	return LoadAsUintOr(kvs, k, 0)
}

func LoadAsUintOr(kvs KVS, k string, d uint) (uint, error) {
	v, exists, err := kvs.Load(k)
	if err != nil {
		return 0, err
	}
	if !exists || v == nil {
		return d, nil
	}
	vv, err := conv.ToUint(v)
	if err != nil {
		return 0, errs.Wrap(err, "convert value to uint fail")
	}
	return vv, nil
}

func LoadAsFloat(kvs KVS, k string) (float64, error) {
	return LoadAsFloatOr(kvs, k, 0)
}

func LoadAsFloatOr(kvs KVS, k string, d float64) (float64, error) {
	v, exists, err := kvs.Load(k)
	if err != nil {
		return 0, err
	}
	if !exists || v == nil {
		return d, nil
	}
	vv, err := conv.ToFloat(v)
	if err != nil {
		return 0, errs.Wrap(err, "convert value to float64 fail")
	}
	return vv, nil
}

func LoadTo(kvs KVS, k string, to any) error {
	v, err := LoadAsString(kvs, k)
	if err != nil {
		return err
	}
	if v == "" {
		return nil
	}

	return jsonu.UnmarshalString(v, to)
}
