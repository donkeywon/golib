package svcd

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"codeberg.org/donkeywon/golib/boot"
	"codeberg.org/donkeywon/golib/errs"
	"codeberg.org/donkeywon/golib/plugin"
)

const DaemonTypeSvcd boot.DaemonType = "svcd"

type Namespace string
type Module string
type Name string

const initSize = 64

var (
	_svcFQNs        = make([]string, 0, initSize)
	_svcCreatorsMap = make(map[string]plugin.Creator[Svc], initSize)
	_svcMap         = make(map[string]Svc, initSize)
	_svcCfgMap      = make(map[string]any, initSize)
	_svcCfgSetters  = make(map[string]func(s Svc, cfg any), initSize)
	_svcd           = &svcd{}
)

type svcd struct {
	Cfg
}

func New() boot.Daemon {
	return _svcd
}

func (s *svcd) Init(ctx context.Context) error {
	for _, fqn := range _svcFQNs {
		creator := _svcCreatorsMap[fqn]

		ins := creator()
		if ins == nil {
			return errs.Errorf("nil svc: %s", fqn)
		}

		_svcMap[fqn] = ins
		if cfg, hasCfg := _svcCfgMap[fqn]; hasCfg {
			_svcCfgSetters[fqn](ins, reflect.ValueOf(cfg).Elem().Interface())
		}
	}

	return nil
}

func (s *svcd) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func buildFQN(ns Namespace, m Module, n Name) string {
	return fmt.Sprintf("%s.%s.%s", ns, m, n)
}

func validate[C any](ns Namespace, m Module, n Name, creator plugin.Creator[Svc], cfgCreator plugin.CfgCreator[C]) {
	if creator == nil {
		panic("nil svc creator")
	}

	if strings.Contains(string(ns), ".") || strings.Contains(string(m), ".") || strings.Contains(string(n), ".") {
		panic("namespace or module or name must not contain dot(.) character")
	}
	if string(ns) == "" {
		panic("empty svc namespace")
	}
	if string(m) == "" {
		panic("empty svc module")
	}
	if string(n) == "" {
		panic("empty svc name")
	}
	if cfgCreator != nil && reflect.TypeFor[C]().Kind() != reflect.Struct {
		panic(fmt.Sprintf("svc cfg creator return non-struct type, FQN: %s.%s.%s", ns, m, n))
	}

	// allow duplicate reg for replacing or testing
	// fqn := buildFQN(ns, m, n)
	// _, exists := _svcMap[fqn]
	// if exists {
	// 	panic("duplicate reg")
	// }
}

func Get[S Svc](ns Namespace, m Module, n Name) S {
	fqn := buildFQN(ns, m, n)
	ins, exists := _svcMap[fqn]
	if !exists {
		panic(fmt.Errorf("svc not exists, register first or incorrect dependencies order, FQN: %s", fqn))
	}

	s, ok := ins.(S)
	if !ok {
		panic(fmt.Errorf("svc %s is not type of %s", fqn, reflect.TypeFor[S]()))
	}

	return s
}

func Reg[C any](ns Namespace, m Module, n Name, creator plugin.Creator[Svc], cfgCreator plugin.CfgCreator[C]) {
	validate(ns, m, n, creator, cfgCreator)

	fqn := buildFQN(ns, m, n)
	if _, exists := _svcCreatorsMap[fqn]; !exists {
		_svcFQNs = append(_svcFQNs, fqn)
	}
	_svcCreatorsMap[fqn] = creator

	if cfgCreator != nil {
		cfg := cfgCreator()
		cfgPtr := &cfg
		boot.RegCfg(fqn, cfgPtr)
		_svcCfgMap[fqn] = cfgPtr
		_svcCfgSetters[fqn] = func(s Svc, cfg any) { plugin.SetCfg(s, cfg.(C)) }
	} else {
		delete(_svcCfgMap, fqn)
		delete(_svcCfgSetters, fqn)
	}
}
