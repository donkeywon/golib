package svc

import (
	"github.com/donkeywon/golib/log"
)

type Creator func() Svc
type CfgCreator func() any

type Svc interface {
	log.Logger
}

type baseSvc struct {
	log.Logger
}
