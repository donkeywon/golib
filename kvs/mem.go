package kvs

import (
	"sync"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/conv"
)

func init() {
	plugin.RegisterWithCfg(TypeMem, func() interface{} { return NewMemKVS() }, func() interface{} { return NewMemKVSCfg() })
}

const TypeMem Type = "mem"

type MemKVSCfg struct{}

func NewMemKVSCfg() *MemKVSCfg {
	return &MemKVSCfg{}
}

type MemKVS struct {
	*MemKVSCfg
	m sync.Map
}

func NewMemKVS() *MemKVS {
	return &MemKVS{}
}

func (i *MemKVS) Open() error {
	return nil
}

func (i *MemKVS) Close() error {
	return nil
}

func (i *MemKVS) Store(k string, v any) error {
	i.m.Store(k, v)
	return nil
}

func (i *MemKVS) StoreAsString(k string, v any) error {
	s, err := conv.AnyToString(v)
	if err != nil {
		return errs.Wrap(err, "convert value to string fail")
	}
	i.m.Store(k, s)
	return nil
}

func (i *MemKVS) Load(k string) (any, bool, error) {
	v, exists := i.m.Load(k)
	return v, exists, nil
}

func (i *MemKVS) LoadOrStore(k string, v any) (any, bool, error) {
	v, loaded := i.m.LoadOrStore(k, v)
	return v, loaded, nil
}

func (i *MemKVS) LoadAndDelete(k string) (any, bool, error) {
	v, loaded := i.m.LoadAndDelete(k)
	return v, loaded, nil
}

func (i *MemKVS) Del(k string) error {
	i.m.Delete(k)
	return nil
}

func (i *MemKVS) LoadAsBool(k string) (bool, error) {
	return LoadAsBool(i, k)
}

func (i *MemKVS) LoadAsString(k string) (string, error) {
	return LoadAsString(i, k)
}

func (i *MemKVS) LoadAsStringOr(k string, d string) (string, error) {
	return LoadAsStringOr(i, k, d)
}

func (i *MemKVS) LoadAsInt(k string) (int, error) {
	return LoadAsInt(i, k)
}

func (i *MemKVS) LoadAsIntOr(k string, d int) (int, error) {
	return LoadAsIntOr(i, k, d)
}

func (i *MemKVS) LoadAsUint(k string) (uint, error) {
	return LoadAsUint(i, k)
}

func (i *MemKVS) LoadAsUintOr(k string, d uint) (uint, error) {
	return LoadAsUintOr(i, k, d)
}

func (i *MemKVS) LoadAsFloat(k string) (float64, error) {
	return LoadAsFloat(i, k)
}

func (i *MemKVS) LoadAsFloatOr(k string, d float64) (float64, error) {
	return LoadAsFloatOr(i, k, d)
}

func (i *MemKVS) Collect() (map[string]any, error) {
	c := make(map[string]any)
	i.Range(func(k string, v any) bool {
		c[k] = v
		return true
	})
	return c, nil
}

func (i *MemKVS) Range(f func(k string, v any) bool) error {
	i.m.Range(func(key, value any) bool {
		return f(key.(string), value)
	})
	return nil
}

func (i *MemKVS) CollectAsString() (map[string]string, error) {
	var err error
	result := make(map[string]string)
	i.Range(func(k string, v any) bool {
		result[k], err = conv.AnyToString(v)
		if err != nil {
			err = errs.Wrap(err, "convert value to string fail")
			return false
		}
		return true
	})
	return result, err
}

func (i *MemKVS) Type() interface{} {
	return TypeMem
}

func (i *MemKVS) GetCfg() interface{} {
	return i.MemKVSCfg
}
