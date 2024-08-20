package task

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/pipeline"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util"
)

func init() {
	plugin.Register(StepTypePipeline, func() interface{} { return NewPipelineStep() })
	plugin.RegisterCfg(StepTypePipeline, func() interface{} { return NewPipelineCfg() })
}

const StepTypePipeline StepType = "pipeline"

func NewPipelineCfg() *pipeline.Cfg {
	return pipeline.NewCfg()
}

type Pipeline struct {
	Step
	*pipeline.Cfg

	p *pipeline.Pipeline
}

func NewPipelineStep() *Pipeline {
	return &Pipeline{
		Step: newBase("step"),
	}
}

func (p *Pipeline) Init() error {
	err := util.V.Struct(p.Cfg)
	if err != nil {
		return err
	}

	p.p = pipeline.New()
	p.p.Cfg = p.Cfg

	return p.Step.Init()
}

func (p *Pipeline) Start() error {
	err := runner.Init(p.p)
	if err != nil {
		return errs.Wrap(err, "init pipeline fail")
	}

	runner.Start(p.p)
	return p.p.Err()
}

func (p *Pipeline) Stop() error {
	runner.Stop(p.p)
	return nil
}

func (p *Pipeline) Type() interface{} {
	return StepTypePipeline
}

func (p *Pipeline) GetCfg() interface{} {
	return p.Cfg
}
