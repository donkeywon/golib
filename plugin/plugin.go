package plugin

import (
	"fmt"
	"reflect"

	"codeberg.org/donkeywon/golib/util/reflects"
)

type Creator[P Plugin] func() P
type CfgCreator[C any] func() C

type Type string

type CfgSetter[C any] interface {
	SetCfg(C)
}

type Plugin any

var (
	_pluginCreators    = make(map[any]func() any)
	_pluginCfgCreators = make(map[any]func() any)
	_pluginCfgSetters  = make(map[any]func(p any, cfg any))
)

func Reg[P Plugin, C any](typ any, creator Creator[P], cfgCreator CfgCreator[C]) {
	validate(typ, creator, cfgCreator)

	_pluginCreators[typ] = func() any { return creator() }
	if cfgCreator != nil {
		_pluginCfgCreators[typ] = func() any { return cfgCreator() }

		_pluginCfgSetters[typ] = func(p any, cfgAny any) {
			if cfgAny == nil {
				return
			}

			SetCfg(p, cfgAny.(C))
		}
	} else {
		delete(_pluginCfgCreators, typ)
		delete(_pluginCfgSetters, typ)
	}
}

func validate[P Plugin, C any](typ any, creator Creator[P], cfgCreator CfgCreator[C]) {
	if creator == nil {
		panic(fmt.Sprintf("nil plugin creator: %v", typ))
	}
	if typ == nil {
		panic("nil plugin type")
	}
	if cfgCreator != nil {
		if reflect.TypeFor[C]().Kind() != reflect.Struct {
			panic(fmt.Sprintf("plugin cfg creator return non-struct type: %v", typ))
		}
	}
	// allow duplicate reg for replacing or testing
	// _, exists := _pluginCreators[typ]
	// if exists {
	// 	panic("duplicate reg")
	// }
}

func CreateWithCfg[P Plugin, C any](typ any, cfg C) P {
	p := Create[P](typ)

	if cfgSetter, ok := _pluginCfgSetters[typ]; ok {
		cfgSetter(p, cfg)
	} else if !reflects.IsNil(cfg) {
		SetCfg(p, cfg)
	}

	return p
}

func CreateCfg[C any](typ any) C {
	var zero C
	f, exists := _pluginCfgCreators[typ]
	if !exists {
		return zero
	}

	c := f()
	cc, ok := c.(C)
	if !ok {
		panic(fmt.Sprintf("plugin cfg type mismatch: want %v, got %v (type: %+v)", reflect.TypeFor[C](), reflect.TypeOf(c), typ))
	}
	return cc
}

func Create[P Plugin](typ any) P {
	f, exists := _pluginCreators[typ]
	if !exists {
		panic(fmt.Sprintf("plugin not exists: %+v", typ))
	}

	p := f()
	pp, ok := p.(P)
	if !ok {
		panic(fmt.Sprintf("plugin type mismatch: want %v, got %v (type: %+v)", reflect.TypeFor[P](), reflect.TypeOf(p), typ))
	}

	return pp
}

func setCfgReflect(p any, cfg any) {
	pValue := reflect.ValueOf(p)
	if pValue.Kind() != reflect.Pointer {
		panic(fmt.Sprintf("plugin is not CfgSetter[%T] or pointer: %T", cfg, p))
	}

	cfgRV := reflect.ValueOf(cfg)
	if isNil(cfg, cfgRV) {
		return
	}

	for _, field := range pValue.Elem().Fields() {
		if field.CanSet() && field.Type() == cfgRV.Type() {
			field.Set(cfgRV)
			return
		}
	}
	panic(fmt.Sprintf("plugin has no exported cfg field or SetCfg method: %T has no %T field or SetCfg method", p, cfg))
}

func SetCfg[C any](p any, cfg C) {
	if p == nil {
		panic("nil plugin")
	}
	if sp, ok := p.(CfgSetter[C]); ok {
		sp.SetCfg(cfg)
		return
	}
	if sp, ok := p.(CfgSetter[*C]); ok {
		sp.SetCfg(&cfg)
		return
	}
	if sp, ok := p.(CfgSetter[any]); ok {
		sp.SetCfg(cfg)
		return
	}

	setCfgReflect(p, cfg)
}

func isNil(v any, rv reflect.Value) bool {
	if v == nil {
		return true
	}
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface,
		reflect.Func, reflect.Chan:
		return rv.IsNil()
	}
	return false
}
