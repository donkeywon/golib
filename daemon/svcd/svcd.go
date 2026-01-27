package svcd

import (
	"fmt"
	"strings"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/reflects"
)

const (
	DaemonTypeSvcd boot.DaemonType = "svcd"
)

type Namespace string
type Module string
type Name string

var (
	_svcCreators = make([]*svcCreatorWithFQN, 0, 64)
	_svcMap      = make(map[string]Svc)
	_svcCfgMap   = make(map[string]any)
)

var D boot.Daemon = &svcd{
	Runner: runner.Create("svc"),
}

type svcCreatorWithFQN struct {
	fqn     string
	creator Creator
}

type svcd struct {
	runner.Runner
	*Cfg
}

func (s *svcd) Init() error {
	for _, fqnWithCreator := range _svcCreators {
		fqn := fqnWithCreator.fqn
		s.Debug("create svc", "fqn", fqn)

		ins := fqnWithCreator.creator()
		if ins == nil {
			return errs.Errorf("svc is nil, FQN: %s", fqn)
		}

		if _, exists := _svcMap[fqn]; exists {
			return errs.Errorf("svc already exists, FQN: %s", fqn)
		}

		_svcMap[fqn] = ins
		bs := &baseSvc{Logger: s.WithLoggerName(fqn)}
		success := reflects.SetFirstMatchedField(ins, bs)
		if !success {
			return errs.Errorf("svc must have an exported field of type Svc, FQN: %s", fqn)
		}

		if cfg, hasCfg := _svcCfgMap[fqn]; hasCfg {
			s.Debug("set cfg to svc", "fqn", fqn, "cfg", cfg)
			if cs, ok := ins.(plugin.CfgSetter[any]); ok {
				cs.SetCfg(cfg)
			} else {
				success = reflects.SetFirstMatchedField(ins, cfg)
				if !success {
					return errs.Errorf("svc must have an exported field of type Cfg, FQN: %s", fqn)
				}
			}
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

	fqn := buildFQN(ns, m, n)
	_, exists := _svcMap[fqn]
	if exists {
		panic("duplicate reg")
	}
}

func Get[S Svc](ns Namespace, m Module, n Name) S {
	fqn := buildFQN(ns, m, n)
	ins, exists := _svcMap[fqn]
	if !exists {
		panic(fmt.Errorf("svc not exists, maybe dependencies order is invalid, FQN: %s", fqn))
	}

	return ins.(S)
}

func Reg(ns Namespace, m Module, n Name, creator Creator, cfgCreator CfgCreator) {
	validate(ns, m, n, creator, cfgCreator)

	fqn := buildFQN(ns, m, n)
	_svcCreators = append(_svcCreators, &svcCreatorWithFQN{fqn: fqn, creator: creator})

	if cfgCreator != nil {
		cfg := cfgCreator()
		if cfg != nil {
			_svcCfgMap[fqn] = cfg
			boot.RegCfg(fqn, cfg)
		}
	}
}
