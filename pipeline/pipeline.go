package pipeline

import (
	"errors"
	"io"
	"os"
	"reflect"
	"sync"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/v"
)

func init() {
	plugin.Reg(PluginTypePipeline, New, func() any { return NewCfg() })
}

const PluginTypePipeline plugin.Type = "ppl"

type Type string

type Result struct {
	Cfg           *Cfg            `json:"cfg" yaml:"cfg"`
	Data          map[string]any  `json:"data" yaml:"data"`
	WorkersResult []*WorkerResult `json:"workersResult" yaml:"workersResult"`
}

type Cfg struct {
	Workers []*WorkerCfg `json:"workers" yaml:"workers"`
}

func (c *Cfg) build() []Worker {
	ws := make([]Worker, 0, len(c.Workers))
	for _, workerCfg := range c.Workers {
		ws = append(ws, workerCfg.build())
	}
	return ws
}

func NewCfg() *Cfg {
	return &Cfg{}
}

func (c *Cfg) AddWorker(w *WorkerCfg) *Cfg {
	c.Workers = append(c.Workers, w)
	return c
}

func (c *Cfg) Add(typ Type, cfg any, opt *CommonOption) *WorkerCfg {
	workerCfg := &WorkerCfg{
		CommonCfgWithOption: &CommonCfgWithOption{
			CommonCfg: &CommonCfg{
				Type: typ,
				Cfg:  cfg,
			},
			CommonOption: opt,
		},
	}

	c.Workers = append(c.Workers, workerCfg)

	return workerCfg
}

type Pipeline struct {
	runner.Runner

	cfg *Cfg
	ws  []Worker
}

func New() *Pipeline {
	return &Pipeline{
		Runner: runner.Create(string(PluginTypePipeline)),
	}
}

func (p *Pipeline) Init() error {
	err := v.Struct(p.cfg)
	if err != nil {
		return errs.Wrap(err, "pipeline cfg validate failed")
	}

	for i := 0; i < len(p.ws)-1; i++ {
		var (
			pr io.ReadCloser
			pw io.WriteCloser
		)
		if p.ws[i].Name() == string(WorkerCmd) && p.ws[i+1].Name() == string(WorkerCmd) &&
			len(p.ws[i].Writers()) == 0 && len(p.ws[i+1].Readers()) == 0 {
			pr, pw, err = os.Pipe()
			if err != nil {
				return errs.Wrap(err, "create os pipe failed")
			}
		} else {
			pr, pw = io.Pipe()
		}

		if len(p.ws[i].Writers()) > 0 {
			if ww, ok := p.ws[i].LastWriter().(writerWrapper); !ok {
				return errs.Errorf("worker(%d) %s last writer %s is not WriterWrapper", i, p.ws[i].Name(), reflect.TypeOf(p.ws[i].LastWriter()).String())
			} else {
				ww.WrapWriter(pw)
			}
		} else {
			p.ws[i].WriteTo(pw)
		}

		if len(p.ws[i+1].Readers()) > 0 {
			if rr, ok := p.ws[i+1].LastReader().(readerWrapper); !ok {
				return errs.Errorf("worker(%d) %s last reader %s is not ReaderWrapper", i, p.ws[i+1].Name(), reflect.TypeOf(p.ws[i+1].LastReader()).String())
			} else {
				rr.WrapReader(pr)
			}
		} else {
			p.ws[i+1].ReadFrom(pr)
		}
	}

	for i, w := range p.ws {
		w.Inherit(p)
		err = runner.Init(w)
		if err != nil {
			return errs.Wrapf(err, "init worker(%d) %s failed", i, w.Name())
		}
	}

	return p.Runner.Init()
}

// AddWorker for no cfg scene.
func (p *Pipeline) AddWorker(w Worker) {
	p.ws = append(p.ws, w)
}

func (p *Pipeline) Workers() []Worker {
	return p.ws
}

func (p *Pipeline) Start() error {
	var (
		err   error
		errMu sync.Mutex
	)

	wg := &sync.WaitGroup{}
	wg.Add(len(p.ws))
	for _, w := range p.ws {
		go func(w Worker) {
			defer wg.Done()
			e := runner.Run(w)
			if e != nil {
				errMu.Lock()
				err = errors.Join(err, e)
				errMu.Unlock()
			}
		}(w)
	}

	wg.Wait()

	return err
}

func (p *Pipeline) Stop() error {
	runner.Stop(p.ws[0])
	return nil
}

func (p *Pipeline) SetCfg(cfg any) {
	p.cfg = cfg.(*Cfg)

	p.ws = p.cfg.build()
}

func (p *Pipeline) Result() *Result {
	r := &Result{
		Cfg:           p.cfg,
		Data:          p.LoadAll(),
		WorkersResult: make([]*WorkerResult, len(p.ws)),
	}
	for i, w := range p.ws {
		r.WorkersResult[i] = w.Result()
	}
	return r
}
