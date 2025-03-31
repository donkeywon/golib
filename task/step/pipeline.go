package step

import (
	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/pipeline"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/v"
)

func init() {
	plugin.RegWithCfg(TypePipeline, func() Step { return NewPipelineStep() }, func() any { return NewPipelineCfg() })
}

const TypePipeline Type = "pipeline"

func NewPipelineCfg() *pipeline.Cfg {
	return pipeline.NewCfg()
}

type PipelineStep struct {
	Step
	*pipeline.Cfg

	p *pipeline.Pipeline
}

func NewPipelineStep() *PipelineStep {
	return &PipelineStep{
		Step: CreateBase(string(TypePipeline)),
		p:    pipeline.New(),
	}
}

func (p *PipelineStep) Init() error {
	err := v.Struct(p.Cfg)
	if err != nil {
		return err
	}

	err = runner.Init(p.p)
	if err != nil {
		return errs.Wrap(err, "init pipeline failed")
	}

	return p.Step.Init()
}

func (p *PipelineStep) Start() error {
	runner.Start(p.p)
	p.Store(consts.FieldResult, p.p.Result())
	return p.p.Err()
}

func (p *PipelineStep) Stop() error {
	runner.Stop(p.p)
	return nil
}

func (p *PipelineStep) Type() Type {
	return TypePipeline
}

func (p *PipelineStep) SetCfg(cfg any) {
	p.p.SetCfg(cfg)
}

func (p *PipelineStep) Pipeline() *pipeline.Pipeline {
	return p.p
}
