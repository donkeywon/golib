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

type Creator func() any
type CfgCreator func() any

type Type string

type CfgSetter[CFG any] interface {
	SetCfg(cfg CFG)
}

type Plugin[T any] interface {
	Type() T
}

var (
	_plugins    = make(map[any]Creator)
	_pluginCfgs = make(map[any]CfgCreator)
)

// 推荐自定义plugin的类型，不要直接使用基础类型，例如
// type DaemonType string
// const DaemonTypeHttpd DaemonType = "httpd"
// Reg(DaemonTypeHttpd, func() any { return NewHttpd() }).
func Reg[T any](typ T, creator Creator) {
	_plugins[typ] = creator
}

func RegCfg[T any](typ T, creator CfgCreator) {
	_pluginCfgs[typ] = creator
}

func RegWithCfg[T any](typ T, creator Creator, cfgCreator CfgCreator) {
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
func CreateWithCfg[CFG any, T any](typ T, cfg CFG) any {
	f, exists := _plugins[typ]
	if !exists {
		panic(fmt.Sprintf("plugin not exists: %+v", typ))
	}

	// 这里为什么不做cfg的validate校验？
	// 校验逻辑应该放到Create之后的Init阶段，例如runner.Init
	// 即使plugin不是Runner类型，也应该有一个统一的类似Init的阶段用来做一些初始化工作

	p := f()
	if p == nil {
		panic(fmt.Sprintf("plugin created is nil: %+v", typ))
	}

	if cfg != nil {
		SetCfg(p, cfg)
	}

	return p
}

func CreateCfg[T any](typ T) any {
	f, exists := _pluginCfgs[typ]
	if !exists {
		return nil
	}

	cfg := f()
	if cfg == nil {
		return nil
	}

	rt := reflect.TypeOf(cfg)
	if rt.Kind() != reflect.Pointer {
		panic(fmt.Sprintf("plugin(%+v) cfg must be a pointer: %s", typ, rt))
	}
	return cfg
}

func Create[T any](typ T) any {
	return CreateWithCfg(typ, CreateCfg(typ))
}

func SetCfg[T any](p any, cfg T) {
	if sp, ok := p.(CfgSetter[T]); ok {
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
