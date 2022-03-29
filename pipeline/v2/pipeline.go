package v2

import (
	"errors"
	"io"
	"reflect"
	"sync"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/v"
)

func init() {
	plugin.RegWithCfg(PluginTypePipeline, New, func() any { return NewCfg() })
}

const PluginTypePipeline plugin.Type = "pipeline"

type Type string

type ItemCfg struct {
	Cfg    any         `json:"cfg"    yaml:"cfg" validate:"required"`
	Option *ItemOption `json:"option" yaml:"option"`
	Type   Type        `json:"type"   yaml:"type" validate:"required"`
}

type ItemOption struct {
	BufSize     int  `json:"bufSize" yaml:"bufSize"`
	QueueSize   int  `json:"queueSize" yaml:"queueSize"`
	Deadline    int  `json:"deadline" yaml:"deadline"`
	EnableBuf   bool `json:"enableBuf" yaml:"enableBuf"`
	EnableAsync bool `json:"enableAsync" yaml:"enableAsync"`
}

func (ito *ItemOption) ToOptions() []Option {
	opts := make([]Option, 0, 2)
	if ito.EnableBuf {
		opts = append(opts, EnableBuf(ito.BufSize))
	}
	if ito.EnableAsync {
		if ito.Deadline > 0 {
			opts = append(opts, EnableAsyncDeadline(ito.BufSize, ito.QueueSize, time.Second*time.Duration(ito.Deadline)))
		} else {
			opts = append(opts, EnableAsync(ito.BufSize, ito.QueueSize))
		}
	}

	return opts
}

type Cfg struct {
	Items []*ItemCfg `json:"items" yaml:"items"`
}

func NewCfg() *Cfg {
	return &Cfg{}
}

func (c *Cfg) Add(t Type, cfg any, opt *ItemOption) *Cfg {
	c.Items = append(c.Items, &ItemCfg{
		Cfg:    cfg,
		Option: opt,
		Type:   t,
	})
	return c
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
		pr, pw := io.Pipe()
		if len(p.ws[i].Writers()) > 0 {
			if ww, ok := p.ws[i].LastWriter().(WriterWrapper); !ok {
				return errs.Errorf("worker(%d) %s last writer %s is not WriterWrapper", i, p.ws[i].Type(), reflect.TypeOf(p.ws[i].LastWriter()).String())
			} else {
				ww.Wrap(pw)
			}
		} else {
			p.ws[i].WriteTo(pw)
		}

		if len(p.ws[i+1].Readers()) > 0 {
			if rr, ok := p.ws[i+1].LastReader().(ReaderWrapper); !ok {
				return errs.Errorf("worker(%d) %s last reader %s is not ReaderWrapper", i, p.ws[i+1].Type(), reflect.TypeOf(p.ws[i+1].LastReader()).String())
			} else {
				rr.Wrap(pr)
			}
		} else {
			p.ws[i+1].ReadFrom(pr)
		}
	}

	for i, w := range p.ws {
		w.Inherit(p)
		err = runner.Init(w)
		if err != nil {
			return errs.Wrapf(err, "init worker(%d) %s failed", i, w.Type())
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
			runner.Start(w)
			e := w.Err()
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

func (p *Pipeline) Type() plugin.Type {
	return PluginTypePipeline
}

func (p *Pipeline) SetCfg(cfg any) {
	p.cfg = cfg.(*Cfg)

	items := make([]any, 0, len(p.cfg.Items))
	for _, itemCfg := range p.cfg.Items {
		item := plugin.CreateWithCfg[Type, Common](itemCfg.Type, itemCfg.Cfg)
		typ := typeof(item)
		switch typ {
		case 'r', 'w':
			if itemCfg.Option != nil {
				item.(optionApplier).WithOptions(itemCfg.Option.ToOptions()...)
			}
		}

		items = append(items, item)
	}

	groups := groupItemsByWorker(items)
	for _, group := range groups {
		if !hasWorker(group) {
			panic("invalid pipeline items order")
		}
	}

	p.ws = combineReaderAndWriterToWorker(groups)
}

func combineReaderAndWriterToWorker(groups [][]any) []Worker {
	workers := make([]Worker, 0, len(groups))
	for _, group := range groups {
		readers, worker, writers := splitGroup(group)
		if len(readers) > 0 {
			for i := len(readers) - 1; i >= 0; i-- {
				worker.ReadFrom(readers[i])
			}
		}
		if len(writers) > 0 {
			for i := range writers {
				worker.WriteTo(writers[i])
			}
		}
		workers = append(workers, worker)
	}

	return workers
}

func groupItemsByWorker(items []any) [][]any {
	// W worker
	// w writer
	// r reader

	// RW between starter must like
	// Wrrr...rrrW
	// Wwww...wwwW
	// Wrrr...wwwW
	// Wwww...rrrW

	// rr append
	// rW append
	// rw new group
	// WW new group
	// Wr new group
	// Ww append
	// ww append
	// wr new group
	// wW new group

	var (
		groups    [][]any
		itemGroup []any
	)
	for i := range items {
		if i == 0 {
			itemGroup = append(itemGroup, items[i])
			continue
		}
		cur := typeof(items[i])
		previous := typeof(items[i-1])
		if previous == 'r' && cur == 'r' ||
			previous == 'r' && cur == 'W' ||
			previous == 'W' && cur == 'w' ||
			previous == 'w' && cur == 'w' {
			itemGroup = append(itemGroup, items[i])
			continue
		}

		groups = append(groups, itemGroup)
		itemGroup = make([]any, 1)
		itemGroup[0] = items[i]
	}
	if len(itemGroup) > 0 {
		groups = append(groups, itemGroup)
	}
	return groups
}

func typeof(item any) byte {
	switch item.(type) {
	case Reader:
		return 'r'
	case Writer:
		return 'w'
	case Worker:
		return 'W'
	default:
		panic("invalid pipeline item type: " + reflect.TypeOf(item).String())
	}
}

func splitGroup(group []any) ([]Reader, Worker, []Writer) {
	var (
		readers []Reader
		worker  Worker
		writers []Writer
	)

	for _, item := range group {
		switch typeof(item) {
		case 'r':
			readers = append(readers, item.(Reader))
		case 'w':
			writers = append(writers, item.(Writer))
		case 'W':
			worker = item.(Worker)
		}
	}

	return readers, worker, writers
}

func hasWorker(group []any) bool {
	for _, item := range group {
		if typeof(item) == 'W' {
			return true
		}
	}
	return false
}
