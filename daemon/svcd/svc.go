package svcd

import (
	"github.com/donkeywon/golib/log"
	"github.com/donkeywon/golib/plugin"
)

type Creator plugin.Creator[Svc]
type CfgCreator plugin.CfgCreator[any]

type Svc interface {
	log.Logger
}

type baseSvc struct {
	log.Logger
}
