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
	SetCfg(cfg C)
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
func Reg[T any, P Plugin](typ T, creator Creator[P], cfgCreator CfgCreator[any]) {
	validate(typ, creator, cfgCreator)

	_pluginCreators[typ] = creator
	if cfgCreator != nil {
		_pluginCfgCreators[typ] = cfgCreator
	}
}

func validate[T any, P Plugin](typ T, creator Creator[P], cfgCreator CfgCreator[any]) {
	if creator == nil {
		panic("nil plugin creator")
	}
	if isNil(typ) {
		panic("nil plugin type")
	}
	_, exists := _pluginCreators[typ]
	if exists {
		panic("duplicate reg")
	}

	typRT := reflect.TypeOf(typ)

	sample := creator()
	pRT := reflect.TypeOf(sample)
	if pRT == nil {
		panic(fmt.Sprintf("plugin %s(%v) creator return nil", typRT.String(), typ))
	}
	pRV := reflect.ValueOf(sample)
	if !isStruct(pRV) && !isStructPointer(pRV) {
		panic(fmt.Sprintf("plugin %s(%v) is not struct or struct pointer", typRT.String(), typ))
	}

	if cfgCreator != nil {
		sampleCfg := cfgCreator()
		if sampleCfg != nil {
			cRV := reflect.ValueOf(sampleCfg)
			if !isStruct(cRV) && !isStructPointer(cRV) {
				panic(fmt.Sprintf("plugin %s(%v) cfg %s is not struct or struct pointer", typRT.String(), typ, cRV.Type().String()))
			}
		}
	}
}

func isNil(a any) bool {
	return a == nil
}

func isStruct(rv reflect.Value) bool {
	return rv.Kind() == reflect.Struct
}

func isStructPointer(rv reflect.Value) bool {
	if rv.Kind() != reflect.Pointer {
		return false
	}
	if rv.Elem().Kind() != reflect.Struct {
		return false
	}
	return true
}

// 创建一个注册的Plugin
// 这里不返回错误而是直接panic的原因是：
// Create函数只是把plugin创建出来，并把cfg设置到plugin中对应的一个字段里。
// 这里的panic分为两种情况
// 1. plugin不存在，说明没有注册，大部分情况是没有调用RegisterPlugin
// 2. cfg设置失败，说明plugin本身定义的有问题
// 这两种情况下说明代码本身有问题，所以直接panic.
func CreateWithCfg[T any, P Plugin](typ T, cfg any) P {
	f, exists := _pluginCreators[typ]
	if !exists {
		panic(fmt.Sprintf("plugin not exists: %+v", typ))
	}

	// 这里为什么不做cfg的validate校验？
	// 校验逻辑应该放到Create之后的Init阶段，例如runner.Init
	// 即使plugin不是Runner类型，也应该有一个统一的类似Init的阶段用来做一些初始化工作

	p := f.(Creator[P])()
	if cfg != nil {
		SetCfg(p, cfg)
	}

	return p
}

func CreateCfg[T any](typ T) any {
	f, exists := _pluginCfgCreators[typ]
	if !exists {
		return nil
	}

	return f.(CfgCreator[any])()
}

func Create[T any, P Plugin](typ T) P {
	return CreateWithCfg[T, P](typ, CreateCfg(typ))
}

func SetCfg[C any](p any, cfg C) {
	if sp, ok := p.(CfgSetter[C]); ok {
		sp.SetCfg(cfg)
		return
	}

	pValue := reflect.ValueOf(p)
	if pValue.Kind() != reflect.Pointer {
		panic(fmt.Sprintf("plugin(%+v) must be a pointer", pValue.Type()))
	}

	cfgType := reflect.TypeOf(cfg)
	if cfgType.Kind() != reflect.Pointer {
		panic(fmt.Sprintf("plugin(%s) cfg must be a pointer: %s", pValue.Type(), cfgType))
	}

	found := false
	i := 0
	for ; i < pValue.Elem().NumField(); i++ {
		f := pValue.Elem().Field(i)
		if f.CanSet() && f.Type() == cfgType {
			found = true
			break
		}
	}
	if !found {
		panic(fmt.Sprintf("plugin(%+v) must has a exported cfg field", pValue.Type()))
	}

	pValue.Elem().Field(i).Set(reflect.ValueOf(cfg))
}
