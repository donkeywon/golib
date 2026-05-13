package svcd

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

const DaemonTypeSvcd boot.DaemonType = "svcd"

type Namespace string
type Module string
type Name string

const initSize = 64

var (
	_svcFQNs        = make([]string, 0, initSize)
	_svcCreatorsMap = make(map[string]Creator, initSize)
	_svcMap         = make(map[string]Svc, initSize)
	_svcCfgMap      = make(map[string]any, initSize)
	_svcd           = &svcd{
		Runner: runner.Create("svc"),
	}
)

type svcd struct {
	runner.Runner
	*Cfg
}

func New() boot.Daemon {
	return _svcd
}

func (s *svcd) Init() error {
	for _, fqn := range _svcFQNs {
		creator := _svcCreatorsMap[fqn]
		s.Debug("create svc", "fqn", fqn)

		ins := creator()
		if ins == nil {
			return errs.Errorf("nil svc: %s", fqn)
		}

		_svcMap[fqn] = ins
		if cfg, hasCfg := _svcCfgMap[fqn]; hasCfg {
			plugin.SetCfg(ins, cfg)
		}
	}

	return s.Runner.Init()
}

func buildFQN(ns Namespace, m Module, n Name) string {
	return fmt.Sprintf("%s.%s.%s", ns, m, n)
}

func validate(ns Namespace, m Module, n Name, creator Creator, cfgCreator CfgCreator) {
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

func Reg(ns Namespace, m Module, n Name, creator Creator, cfgCreator CfgCreator) {
	validate(ns, m, n, creator, cfgCreator)

	fqn := buildFQN(ns, m, n)
	if _, exists := _svcCreatorsMap[fqn]; !exists {
		_svcFQNs = append(_svcFQNs, fqn)
	}
	_svcCreatorsMap[fqn] = creator

	if cfgCreator != nil {
		cfg := cfgCreator()
		if cfg != nil {
			_svcCfgMap[fqn] = cfg
			boot.RegCfg(fqn, cfg)
		}
	}
}
