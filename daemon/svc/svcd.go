package svc

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
	DaemonTypeSvc boot.DaemonType = "svc"
)

type Namespace string
type Module string
type Name string

var D Svcd = &svcd{
	Runner:      runner.Create("svc"),
	svcCreators: make([]svcCreatorWithFQN, 0, 64),
	svcMap:      make(map[string]Svc),
	svcCfgMap:   make(map[string]any),
}

type Svcd interface {
	boot.Daemon
	Get(ns Namespace, m Module, n Name) Svc
	Reg(ns Namespace, m Module, n Name, creator Creator, cfgCreator CfgCreator)
}

type svcCreatorWithFQN struct {
	fqn     string
	creator Creator
}

type svcd struct {
	runner.Runner
	*Cfg

	svcCreators []svcCreatorWithFQN
	svcMap      map[string]Svc
	svcCfgMap   map[string]any
}

func (s *svcd) Init() error {
	for _, fqnWithCreator := range s.svcCreators {
		fqn := fqnWithCreator.fqn
		s.Debug("create svc", "fqn", fqn)

		ins := fqnWithCreator.creator()
		if ins == nil {
			return errs.Errorf("svc is nil, FQN: %s", fqn)
		}

		if _, exists := s.svcMap[fqn]; exists {
			return errs.Errorf("svc already exists, FQN: %s", fqn)
		}

		s.svcMap[fqn] = ins
		bs := &baseSvc{Logger: s.WithLoggerName(fqn)}
		success := reflects.SetFirstMatchedField(ins, bs)
		if !success {
			return errs.Errorf("svc must have an exported field of type Svc, FQN: %s", fqn)
		}

		if cfg, hasCfg := s.svcCfgMap[fqn]; hasCfg {
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

func (s *svcd) Reg(ns Namespace, m Module, n Name, creator Creator, cfgCreator CfgCreator) {
	s.validate(ns, m, n, creator, cfgCreator)

	fqn := buildFQN(ns, m, n)
	s.svcCreators = append(s.svcCreators, svcCreatorWithFQN{fqn: fqn, creator: creator})

	if cfgCreator != nil {
		cfg := cfgCreator()
		if cfg != nil {
			s.svcCfgMap[fqn] = cfg
			boot.RegCfg(fqn, cfg)
		}
	}
}

func (s *svcd) Get(ns Namespace, m Module, n Name) Svc {
	fqn := buildFQN(ns, m, n)
	ins, exists := s.svcMap[fqn]
	if !exists {
		panic(fmt.Errorf("svc not exists, maybe dependencies order is invalid, FQN: %s", fqn))
	}

	return ins
}

func buildFQN(ns Namespace, m Module, n Name) string {
	return fmt.Sprintf("%s.%s.%s", ns, m, n)
}

func (s *svcd) validate(ns Namespace, m Module, n Name, creator Creator, cfgCreator CfgCreator) {
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
	_, exists := s.svcMap[fqn]
	if exists {
		panic("duplicate reg")
	}

	sample := creator()
	if sample == nil {
		panic(fmt.Sprintf("svc %s creator return nil", fqn))
	}
}
