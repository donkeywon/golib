package svc

import "github.com/donkeywon/golib/log"

type Creator func() interface{}
type CfgCreator func() interface{}

type Svc interface {
	log.Logger
}

type baseSvc struct {
	log.Logger
}
