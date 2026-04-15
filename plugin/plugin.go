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
	_pluginCreators    = make(map[any]any)
	_pluginCfgCreators = make(map[any]any)
)

// 推荐自定义plugin的类型，不要直接使用基础类型，例如
// type DaemonType string
// const DaemonTypeHttpd DaemonType = "httpd"
// Reg(DaemonTypeHttpd, func() Daemon { return NewHttpd() }, func() any { return NewHttpdCfg() }).
func Reg[P Plugin, C any](typ any, creator Creator[P], cfgCreator CfgCreator[C]) {
	validate(typ, creator, cfgCreator)

	_pluginCreators[typ] = creator
	if cfgCreator != nil {
		_pluginCfgCreators[typ] = cfgCreator
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

	sample := creator()
	pRT := reflect.TypeOf(sample)
	if pRT == nil {
		panic(fmt.Sprintf("plugin creator returned nil: %s(%v)", reflect.TypeOf(typ).String(), typ))
	}
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

	// 这里为什么不做cfg的validate校验？
	// 校验逻辑应该放到Create之后的Init阶段，例如runner.Init
	// 即使plugin不是Runner类型，也应该有一个统一的类似Init的阶段用来做一些初始化工作

	var p P
	if ff, ok := f.(Creator[P]); ok {
		p = ff()
	} else if ff, ok := f.(Creator[any]); ok {
		p = ff().(P)
	} else {
		panic(fmt.Sprintf("plugin creator type mismatch: %s(%v)", reflect.TypeOf(typ).String(), typ))
	}

	SetCfg(p, cfg)

	return p
}

func CreateCfg[C any](typ any) C {
	var emptyC C
	f, exists := _pluginCfgCreators[typ]
	if !exists {
		return emptyC
	}

	if ff, ok := f.(CfgCreator[C]); ok {
		return ff()
	}

	if ff, ok := f.(CfgCreator[*C]); ok {
		return *ff()
	}

	return f.(CfgCreator[any])().(C)
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
		panic(fmt.Sprintf("plugin is not CfgSetter[%+v] or pointer: %+v", reflect.TypeOf(cfg), pValue.Type()))
	}

	cfgRV := reflect.ValueOf(cfg)
	if isNil(cfg, cfgRV) {
		return
	}

	found := false
	for i := range pValue.Elem().NumField() {
		f := pValue.Elem().Field(i)
		if f.CanSet() && cfgRV.Type().AssignableTo(f.Type()) {
			f.Set(cfgRV)
			found = true
			break
		}
	}
	if !found {
		panic(fmt.Sprintf("plugin has no exported cfg field: %+v %+v", pValue.Type(), cfgRV.Type()))
	}
}

func isNil(v any, rv reflect.Value) bool {
	if v == nil {
		return true
	}
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface,
		reflect.Func, reflect.Chan:
		return rv.IsNil()
	}
	return false
}
