package svc

import (
	"context"
	"github.com/donkeywon/golib/log"
)

type Creator func() any
type CfgCreator func() any

type Svc interface {
	log.Logger
	Ctx() context.Context
	SetCtx(context.Context)
}

type baseSvc struct {
	log.Logger

	ctx context.Context
}

func (s *baseSvc) Ctx() context.Context {
	return s.ctx
}

func (s *baseSvc) SetCtx(ctx context.Context) {
	if ctx == nil {
		panic("nil context")
	}
	s.ctx = ctx
}
