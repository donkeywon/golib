package svc

import (
	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/runner"
)

const DaemonTypeSvc boot.DaemonType = "svc"

type Namespace string
type Module string

var _s = &svcd{
	Runner: runner.Create("svc"),
}

type svcd struct {
	runner.Runner
	*Cfg
}

func (s *svcd) Reg(ns Namespace, m Module, svcName string) {

}

func (s *svcd) Get(ns Namespace, m Module, svcName string) Svc {

}

func Reg(ns Namespace, m Module, svcName string) {

}

func Get(ns Namespace, m Module, svcName string) Svc {

}
