package plugin

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrInvalidPluginCreator    = errors.New("invalid plugin creator")
	ErrInvalidPluginCfgCreator = errors.New("invalid plugin cfg creator")
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
)

func Reg[P Plugin, C any](typ any, creator Creator[P], cfgCreator CfgCreator[C]) {
	validate(typ, creator, cfgCreator)

	_pluginCreators[typ] = func() any { return creator() }
	if cfgCreator != nil {
		_pluginCfgCreators[typ] = func() any { return cfgCreator() }
	}
}

func validate[P Plugin, C any](typ any, creator Creator[P], cfgCreator CfgCreator[C]) {
	if creator == nil {
		panic("nil plugin creator")
	}
	if typ == nil {
		panic("nil plugin type")
	}
	// allow duplicate reg for replacing or testing
	// _, exists := _pluginCreators[typ]
	// if exists {
	// 	panic("duplicate reg")
	// }
}

// 创建一个注册的Plugin
// 这里不返回错误而是直接panic的原因是：
// Create函数只是把plugin创建出来，并把cfg设置到plugin中对应的一个字段里。
// 这里的panic分为两种情况
// 1. plugin不存在，说明没有注册，大部分情况是没有调用Reg
// 2. cfg设置失败，说明plugin本身定义的有问题
// 这两种情况下说明代码本身有问题，所以直接panic.
func CreateWithCfg[P Plugin, C any](typ any, cfg C) P {
	f, exists := _pluginCreators[typ]
	if !exists {
		panic(fmt.Sprintf("plugin not exists: %+v", typ))
	}

	p := f()
	pp, ok := p.(P)
	if !ok {
		panic(fmt.Sprintf("plugin type mismatch: want %v, got %v (type: %+v)", reflect.TypeFor[P](), reflect.TypeOf(p), typ))
	}

	SetCfg(pp, cfg)

	return pp
}

func CreateCfg[C any](typ any) C {
	var zero C
	f, exists := _pluginCfgCreators[typ]
	if !exists {
		return zero
	}

	c := f()
	switch cc := c.(type) {
	case C:
		return cc
	case *C:
		return *cc
	default:
		panic(fmt.Sprintf("plugin cfg type mismatch: want %v, got %v (type: %+v)", reflect.TypeFor[C](), reflect.TypeOf(c), typ))
	}
}

func Create[P Plugin, C any](typ any) P {
	return CreateWithCfg[P](typ, CreateCfg[C](typ))
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
	panic(fmt.Sprintf("plugin has no exported cfg field: %T %T", p, cfg))
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
