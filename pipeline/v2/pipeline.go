package v2

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/v"
)

const PluginTypePipeline plugin.Type = "pipeline"

type Kind string
type Type string

const (
	KindWorker Kind = "worker"
	KindReader Kind = "reader"
	KindWriter Kind = "writer"
)

type ItemCfg struct {
	Cfg    any         `json:"cfg"    yaml:"cfg" validate:"required"`
	Option *ItemOption `json:"option" yaml:"option"`
	Type   any         `json:"type"   yaml:"type" validate:"required"`
	Kind   Kind        `json:"kind"   yaml:"kind" validate:"required"`
}

type ItemOption struct {
	BufSize     int  `json:"bufSize" yaml:"bufSize"`
	QueueSize   int  `json:"queueSize" yaml:"queueSize"`
	Deadline    int  `json:"deadline" yaml:"deadline"`
	EnableBuf   bool `json:"enableBuf" yaml:"enableBuf"`
	EnableAsync bool `json:"enableAsync" yaml:"enableAsync"`
}

type Cfg struct {
	Items []*ItemCfg `json:"items" yaml:"items"`
}

func (c *Cfg) AddReader(t ReaderType, cfg any, opt *ItemOption) {
	c.Items = append(c.Items, &ItemCfg{
		Cfg:     cfg,
		Option:  opt,
		Kind:    KindReader,
		SubType: t,
	})
}

func (c *Cfg) AddWorker(t WorkerType, cfg any, opt *ItemOption) {
	c.Items = append(c.Items, &ItemCfg{
		Cfg:     cfg,
		Option:  opt,
		Kind:    KindWorker,
		SubType: t,
	})
}

func (c *Cfg) AddWriter(t WriterType, cfg any) {
	c.Items = append(c.Items, &ItemCfg{
		Cfg:     cfg,
		Kind:    KindWriter,
		SubType: t,
	})
}

type Pipeline struct {
	runner.Runner

	cfg *Cfg
	ws  []Worker
}

func NewPipeline() *Pipeline {
	return &Pipeline{
		Runner: runner.Create(string(PluginTypePipeline)),
	}
}

func (p *Pipeline) Init() error {
	err := v.Struct(p.cfg)
	if err != nil {
		return errs.Wrap(err, "validate failed")
	}

}

func (p *Pipeline) Start() error {}

func (p *Pipeline) Stop() error {

}

func (p *Pipeline) Type() any {
	return PluginTypePipeline
}

func (p *Pipeline) GetCfg() any {
	return p.cfg
}

func (p *Pipeline) SetCfg(cfg any) {
	p.cfg = cfg.(*Cfg)

}
