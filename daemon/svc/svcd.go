package svc

import (
	"fmt"
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

var _s = &svcd{
	Runner: runner.Create("svc"),
}

func D() boot.Daemon {
	return _s
}

type svcd struct {
	runner.Runner
	*Cfg

	svcCreators map[string]Creator
	svcs        map[string]interface{}
}

func (s *svcd) Init() error {
	for fqn, creator := range s.svcCreators {
		s.Debug("create svc", "fqn", fqn)
		ins := creator()
		s.svcs[fqn] = ins
		bs := &baseSvc{Logger: s.WithLoggerName(fqn)}
		success := reflects.SetFirstMatchedField(ins, bs)
		if !success {
			return errs.Errorf("svc must have an exported field of type Svc, FQN: %s", fqn)
		}
	}

	return s.Runner.Init()
}

func (s *svcd) Reg(ns Namespace, m Module, svcName string, creator Creator) {
	if string(ns) == "" || string(m) == "" || svcName == "" {
		panic("namespace and module and svcName must not empty")
	}
	fqn := buildFQN(ns, m, svcName)
	if _, ok := s.svcCreators[fqn]; ok {
		panic(fmt.Errorf("svc creator already exists, FQN: %s", fqn))
	}

	s.svcCreators[fqn] = creator
}

func (s *svcd) RegWithCfg(ns Namespace, m Module, svcName string, creator Creator, cfg CfgCreator) {

}

func (s *svcd) Get(ns Namespace, m Module, svcName string) Svc {
	// TODO
	return nil
}

func (s *svcd) Type() interface{} {
	return DaemonTypeSvc
}

func (s *svcd) GetCfg() interface{} {
	return s.Cfg
}

func buildFQN(ns Namespace, m Module, svcName string) string {
	return fmt.Sprintf("%s.%s.%s", ns, m, svcName)
}
