package svc

import (
	"fmt"
	"github.com/donkeywon/golib/plugin"
	"strings"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/reflects"
)

const (
	DaemonTypeSvc boot.DaemonType = "svc"
)

type Namespace string
type Module string
type Name string

var _s = &svcd{
	Runner:      runner.Create("svc"),
	svcCreators: make([]svcCreatorWithFQN, 0, 64),
	svcMap:      make(map[string]any),
	svcCfgMap:   make(map[string]any),
}

func D() boot.Daemon {
	return _s
}

type svcCreatorWithFQN struct {
	fqn     string
	creator Creator
}

type svcd struct {
	runner.Runner
	*Cfg

	svcCreators []svcCreatorWithFQN
	svcMap      map[string]any
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
			s.Debug("apply cfg to svc", "fqn", fqn, "cfg", cfg)
			if cs, ok := ins.(plugin.CfgSetter); ok {
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

func (s *svcd) Reg(ns Namespace, m Module, svcName Name, creator Creator) {
	checkValid(ns, m, svcName)

	fqn := buildFQN(ns, m, svcName)
	s.svcCreators = append(s.svcCreators, svcCreatorWithFQN{fqn: fqn, creator: creator})
}

func (s *svcd) RegWithCfg(ns Namespace, m Module, svcName Name, creator Creator, cfgCreator CfgCreator) {
	s.Reg(ns, m, svcName, creator)

	fqn := buildFQN(ns, m, svcName)
	cfg := cfgCreator()
	if cfg == nil {
		panic(fmt.Errorf("cfg is nil, FQN: %s", fqn))
	}

	s.svcCfgMap[fqn] = cfg
	boot.RegCfg(fqn, cfg)
}

func (s *svcd) Get(ns Namespace, m Module, svcName Name) Svc {
	fqn := buildFQN(ns, m, svcName)
	ins, exists := s.svcMap[fqn]
	if !exists {
		panic(fmt.Errorf("svc not exists, maybe dependencies order is invalid, FQN: %s", fqn))
	}

	return ins.(Svc)
}

func (s *svcd) Type() any {
	return DaemonTypeSvc
}

func (s *svcd) GetCfg() any {
	return s.Cfg
}

func Reg(ns Namespace, m Module, svcName Name, creator Creator) {
	_s.Reg(ns, m, svcName, creator)
}

func RegWithCfg(ns Namespace, m Module, svcName Name, creator Creator, cfgCreator CfgCreator) {
	_s.RegWithCfg(ns, m, svcName, creator, cfgCreator)
}

func Get(ns Namespace, m Module, svcName Name) Svc {
	return _s.Get(ns, m, svcName)
}

func buildFQN(ns Namespace, m Module, svcName Name) string {
	return fmt.Sprintf("%s.%s.%s", ns, m, svcName)
}

func checkValid(ns Namespace, m Module, svcName Name) {
	if strings.Contains(string(ns), ".") || strings.Contains(string(m), ".") || strings.Contains(string(svcName), ".") {
		panic("namespace or module or svcName must not contain dot(.) character")
	}
	if string(ns) == "" || string(m) == "" || svcName == "" {
		panic("namespace and module and svcName must not empty")
	}
}
