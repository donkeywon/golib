package kvs

import (
	"sync"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/conv"
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
	*InMemKVSCfg
	m sync.Map
}

func NewInMemKVS() *InMemKVS {
	return &InMemKVS{}
}

func (i *InMemKVS) Open() error {
	return nil
}

func (i *InMemKVS) Close() error {
	return nil
}

func (i *InMemKVS) Store(k string, v any) error {
	i.m.Store(k, v)
	return nil
}

func (i *InMemKVS) StoreAsString(k string, v any) error {
	s, err := conv.AnyToString(v)
	if err != nil {
		return errs.Wrap(err, "convert value to string fail")
	}
	i.m.Store(k, s)
	return nil
}

func (i *InMemKVS) Load(k string) (any, bool, error) {
	v, exists := i.m.Load(k)
	return v, exists, nil
}

func (i *InMemKVS) LoadOrStore(k string, v any) (any, bool, error) {
	v, loaded := i.m.LoadOrStore(k, v)
	return v, loaded, nil
}

func (i *InMemKVS) LoadAndDelete(k string) (any, bool, error) {
	v, loaded := i.m.LoadAndDelete(k)
	return v, loaded, nil
}

func (i *InMemKVS) Del(k string) error {
	i.m.Delete(k)
	return nil
}

func (i *InMemKVS) LoadAsBool(k string) (bool, error) {
	return LoadAsBool(i, k)
}

func (i *InMemKVS) LoadAsString(k string) (string, error) {
	return LoadAsString(i, k)
}

func (i *InMemKVS) LoadAsStringOr(k string, d string) (string, error) {
	return LoadAsStringOr(i, k, d)
}

func (i *InMemKVS) LoadAsInt(k string) (int, error) {
	return LoadAsInt(i, k)
}

func (i *InMemKVS) LoadAsIntOr(k string, d int) (int, error) {
	return LoadAsIntOr(i, k, d)
}

func (i *InMemKVS) LoadAsUint(k string) (uint, error) {
	return LoadAsUint(i, k)
}

func (i *InMemKVS) LoadAsUintOr(k string, d uint) (uint, error) {
	return LoadAsUintOr(i, k, d)
}

func (i *InMemKVS) LoadAsFloat(k string) (float64, error) {
	return LoadAsFloat(i, k)
}

func (i *InMemKVS) LoadAsFloatOr(k string, d float64) (float64, error) {
	return LoadAsFloatOr(i, k, d)
}

func (i *InMemKVS) LoadTo(k string, to any) error {
	return LoadTo(i, k, to)
}

func (i *InMemKVS) Collect() (map[string]any, error) {
	c := make(map[string]any)
	i.Range(func(k string, v any) bool {
		c[k] = v
		return true
	})
	return c, nil
}

func (i *InMemKVS) Range(f func(k string, v any) bool) error {
	i.m.Range(func(key, value any) bool {
		return f(key.(string), value)
	})
	return nil
}

func (i *InMemKVS) CollectAsString() (map[string]string, error) {
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

func (i *InMemKVS) Type() interface{} {
	return TypeInMem
}

func (i *InMemKVS) GetCfg() interface{} {
	return i.InMemKVSCfg
}
