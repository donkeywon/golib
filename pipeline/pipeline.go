package pipeline

import (
	"errors"
	"io"
	"sync"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/pipeline/rw"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/v"
)

func init() {
	plugin.RegWithCfg(PluginTypePipeline, New, func() any { return NewCfg() })
}

const PluginTypePipeline plugin.Type = "ppl"

type Cfg struct {
	RWs []*rw.Cfg `json:"rws" validate:"required" yaml:"rws"`
}

func NewCfg() *Cfg {
	return &Cfg{}
}

func (c *Cfg) Add(role rw.Role, typ rw.Type, cfg any, extraCfg *rw.ExtraCfg) *Cfg {
	c.RWs = append(c.RWs, &rw.Cfg{
		Type:     typ,
		Cfg:      cfg,
		ExtraCfg: extraCfg,
		Role:     role,
	})
	return c
}

func (c *Cfg) AddCfg(cfg *rw.Cfg) *Cfg {
	c.RWs = append(c.RWs, cfg)
	return c
}

type Result struct {
	Cfg     *Cfg             `json:"cfg"     yaml:"cfg"`
	Data    map[string]any   `json:"data"    yaml:"data"`
	RWsData []map[string]any `json:"rwsData" yaml:"rwsData"`
}

type Pipeline struct {
	runner.Runner
	*Cfg

	rwGroups []*RWGroup
}

func New() *Pipeline {
	return &Pipeline{
		Runner: runner.Create(string(PluginTypePipeline)),
		Cfg:    NewCfg(),
	}
}

func (p *Pipeline) Init() error {
	err := v.Struct(p.Cfg)
	if err != nil {
		return errs.Wrap(err, "validate failed")
	}

	rwCfgGroups := groupRWCfgByStarter(p.Cfg.RWs)
	for _, rwGroup := range rwCfgGroups {
		if !hasStarter(rwGroup) {
			return errs.New("invalid pipeline cfg order")
		}
	}

	p.rwGroups = createRWGroups(rwCfgGroups)

	for i, rwGroup := range p.rwGroups {
		rwGroup.Inherit(p)
		err = runner.Init(rwGroup)
		if err != nil {
			return errs.Wrapf(err, "init rw group fail: %d", i)
		}
	}

	for i := 0; i < len(p.rwGroups)-1; i++ {
		pr, pw := io.Pipe()
		if len(p.rwGroups[i].Writers()) > 0 {
			p.rwGroups[i].LastWriter().NestWriter(pw)
		} else {
			p.rwGroups[i].Starter().NestWriter(pw)
		}

		if len(p.rwGroups[i+1].Readers()) > 0 {
			p.rwGroups[i+1].FirstReader().NestReader(pr)
		} else {
			p.rwGroups[i+1].Starter().NestReader(pr)
		}
	}

	return p.Runner.Init()
}

func (p *Pipeline) Start() error {
	var (
		err   error
		errMu sync.Mutex
	)

	wg := &sync.WaitGroup{}
	wg.Add(len(p.rwGroups))
	for _, rwGroup := range p.rwGroups {
		go func(rwGroup *RWGroup) {
			defer wg.Done()
			runner.Start(rwGroup)
			re := rwGroup.Err()
			errMu.Lock()
			if re != nil {
				err = errors.Join(err, re)
			}
			errMu.Unlock()
		}(rwGroup)
	}

	wg.Wait()

	return err
}

func (p *Pipeline) Stop() error {
	runner.Stop(p.rwGroups[0])
	return nil
}

func (p *Pipeline) Type() plugin.Type {
	return PluginTypePipeline
}

func (p *Pipeline) GetCfg() any {
	return p.Cfg
}

func (p *Pipeline) RWGroups() []*RWGroup {
	return p.rwGroups
}

func (p *Pipeline) Result() *Result {
	r := &Result{
		Cfg: p.Cfg,
	}
	r.Data = p.LoadAll()

	var data map[string]any
	for _, rwg := range p.rwGroups {
		for _, rw := range rwg.Readers() {
			data = rw.LoadAll()
			r.RWsData = append(r.RWsData, data)
		}
		data = rwg.Starter().LoadAll()
		r.RWsData = append(r.RWsData, data)
		for _, rw := range rwg.Writers() {
			data = rw.LoadAll()
			r.RWsData = append(r.RWsData, data)
		}
	}
	return r
}

func createRWGroups(rwCfgGroups [][]*rw.Cfg) []*RWGroup {
	var rwGroups []*RWGroup
	for _, rwCfgGroup := range rwCfgGroups {
		rwGroupCfg := NewRWGroupCfg()
		readers, starter, writers := splitRWCfgGroup(rwCfgGroup)
		for _, reader := range readers {
			rwGroupCfg.FromReader(reader.Type, reader.Cfg, reader.ExtraCfg)
		}
		rwGroupCfg.SetStarter(starter.Type, starter.Cfg, starter.ExtraCfg)
		for _, writer := range writers {
			rwGroupCfg.ToWriter(writer.Type, writer.Cfg, writer.ExtraCfg)
		}

		rwGroup := NewRWGroup()
		rwGroup.RWGroupCfg = rwGroupCfg
		rwGroups = append(rwGroups, rwGroup)
	}
	return rwGroups
}

func groupRWCfgByStarter(rws []*rw.Cfg) [][]*rw.Cfg {
	// RW between starter must like
	// Srrr...rrrS
	// Swww...wwwS
	// Srrr...wwwS
	// Swww...rrrS

	// RR append
	// RS append
	// RW new group
	// SS new group
	// SR new group
	// SW append
	// WW append
	// WR new group
	// WS new group
	var rwGroups [][]*rw.Cfg
	var group []*rw.Cfg
	for i := range rws {
		if i == 0 {
			group = append(group, rws[i])
			continue
		}
		curRole := rws[i].Role
		previousRole := rws[i-1].Role
		if previousRole == rw.RoleReader && curRole == rw.RoleReader ||
			previousRole == rw.RoleReader && curRole == rw.RoleStarter ||
			previousRole == rw.RoleStarter && curRole == rw.RoleWriter ||
			previousRole == rw.RoleWriter && curRole == rw.RoleWriter {
			group = append(group, rws[i])
			continue
		}

		rwGroups = append(rwGroups, group)
		group = make([]*rw.Cfg, 1)
		group[0] = rws[i]
	}
	if len(group) != 0 {
		rwGroups = append(rwGroups, group)
	}
	return rwGroups
}

func splitRWCfgGroup(rwCfgGroup []*rw.Cfg) ([]*rw.Cfg, *rw.Cfg, []*rw.Cfg) {
	var (
		readers []*rw.Cfg
		starter *rw.Cfg
		writers []*rw.Cfg
	)
	for _, rwCfg := range rwCfgGroup {
		switch rwCfg.Role {
		case rw.RoleReader:
			readers = append(readers, rwCfg)
		case rw.RoleStarter:
			starter = rwCfg
		case rw.RoleWriter:
			writers = append(writers, rwCfg)
		}
	}
	return readers, starter, writers
}

func hasStarter(rws []*rw.Cfg) bool {
	for _, _rw := range rws {
		if _rw.Role == rw.RoleStarter {
			return true
		}
	}
	return false
}
