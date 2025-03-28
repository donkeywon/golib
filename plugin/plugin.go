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

type Creator[T any, P Plugin[T]] func() P
type CfgCreator[C any] func() C

type Type string

type CfgSetter[C any] interface {
	SetCfg(cfg C)
}

type Plugin[T any] interface {
	Type() T
}

var (
	_pluginCreators    = make(map[any]any)
	_pluginCfgCreators = make(map[any]any)
)

// 推荐自定义plugin的类型，不要直接使用基础类型，例如
// type DaemonType string
// const DaemonTypeHttpd DaemonType = "httpd"
// Reg(DaemonTypeHttpd, func() any { return NewHttpd() }).
func Reg[T any, P Plugin[T]](typ T, creator Creator[T, P]) {
	var p P
	rt := reflect.TypeOf(p)
	if rt != nil && rt.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("plugin %s is not interface or ptr", rt.String()))
	}
	_pluginCreators[typ] = creator
}

func RegCfg[T any, C any](typ T, creator CfgCreator[C]) {
	var c C
	rt := reflect.TypeOf(c)
	if rt != nil && rt.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("plugin cfg %s is not interface or ptr", rt.String()))
	}
	_pluginCfgCreators[typ] = creator
}

func RegWithCfg[T any, C any, P Plugin[T]](typ T, creator Creator[T, P], cfgCreator CfgCreator[C]) {
	Reg(typ, creator)
	RegCfg(typ, cfgCreator)
}

// 创建一个注册的Plugin
// 这里不返回错误而是直接panic的原因是：
// Create函数只是把plugin创建出来，并把cfg设置到plugin中对应的一个字段里。
// 这里的panic分为两种情况
// 1. plugin不存在，说明没有注册，大部分情况是没有调用RegisterPlugin
// 2. cfg设置失败，说明plugin本身定义的有问题
// 这两种情况下说明代码本身有问题，所以直接panic.
func CreateWithCfg[T any, P Plugin[T], C any](typ T, cfg C) P {
	f, exists := _pluginCreators[typ]
	if !exists {
		panic(fmt.Sprintf("plugin not exists: %+v", typ))
	}

	// 这里为什么不做cfg的validate校验？
	// 校验逻辑应该放到Create之后的Init阶段，例如runner.Init
	// 即使plugin不是Runner类型，也应该有一个统一的类似Init的阶段用来做一些初始化工作

	p := f.(Creator[T, P])()
	SetCfg(p, cfg)

	return p
}

func CreateCfg[T any](typ T) any {
	f, exists := _pluginCfgCreators[typ]
	if !exists {
		return nil
	}

	// TODO
	return f.(CfgCreator)()
}

func Create[T any, P Plugin[T], C any](typ T) P {
	return CreateWithCfg[T, P, C](typ, CreateCfg[T, C](typ))
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
