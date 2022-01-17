package pipeline

import (
	"errors"
	"io"
	"sync"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/v"
)

func init() {
	plugin.RegisterWithCfg(PluginTypePipeline, func() interface{} { return New() }, func() interface{} { return NewCfg() })
}

const PluginTypePipeline plugin.Type = "ppl"

type Cfg struct {
	RWs []*RWCfg `json:"rws" validate:"required" yaml:"rws"`
}

func NewCfg() *Cfg {
	return &Cfg{}
}

func (c *Cfg) Add(role RWRole, typ RWType, cfg interface{}, commonCfg *RWCommonCfg) *Cfg {
	c.RWs = append(c.RWs, &RWCfg{
		Type:      typ,
		Cfg:       cfg,
		CommonCfg: commonCfg,
		Role:      role,
	})
	return c
}

func (c *Cfg) AddCfg(cfg *RWCfg) *Cfg {
	c.RWs = append(c.RWs, cfg)
	return c
}

type Result struct {
	Cfg     *Cfg                     `json:"cfg"     yaml:"cfg"`
	Data    map[string]interface{}   `json:"data"    yaml:"data"`
	RWsData []map[string]interface{} `json:"rwsData" yaml:"rwsData"`
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
		return errs.Wrap(err, "validate fail")
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
			err = p.rwGroups[i].LastWriter().NestWriter(pw)
		} else {
			err = p.rwGroups[i].Starter().NestWriter(pw)
		}
		if err != nil {
			return errs.Wrapf(err, "rwGroup(%d) nest pipe writer fail", i)
		}

		if len(p.rwGroups[i+1].Readers()) > 0 {
			err = p.rwGroups[i+1].FirstReader().NestReader(pr)
		} else {
			err = p.rwGroups[i+1].Starter().NestReader(pr)
		}
		if err != nil {
			return errs.Wrapf(err, "rwGroup(%d) nest pipe reader fail", i+1)
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

func (p *Pipeline) Type() interface{} {
	return PluginTypePipeline
}

func (p *Pipeline) GetCfg() interface{} {
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

	for _, rwg := range p.rwGroups {
		for _, rw := range rwg.Readers() {
			v := rw.LoadAll()
			r.RWsData = append(r.RWsData, v)
		}
		v := rwg.Starter().LoadAll()
		r.RWsData = append(r.RWsData, v)
		for _, rw := range rwg.Writers() {
			v := rw.LoadAll()
			r.RWsData = append(r.RWsData, v)
		}
	}
	return r
}

func createRWGroups(rwCfgGroups [][]*RWCfg) []*RWGroup {
	var rwGroups []*RWGroup
	for _, rwCfgGroup := range rwCfgGroups {
		rwGroupCfg := NewRWGroupCfg()
		readers, starter, writers := splitRWCfgGroup(rwCfgGroup)
		for _, reader := range readers {
			rwGroupCfg.FromReader(reader.Type, reader.Cfg, reader.CommonCfg)
		}
		rwGroupCfg.SetStarter(starter.Type, starter.Cfg, starter.CommonCfg)
		for _, writer := range writers {
			rwGroupCfg.ToWriter(writer.Type, writer.Cfg, writer.CommonCfg)
		}

		rwGroup := NewRWGroup()
		rwGroup.RWGroupCfg = rwGroupCfg
		rwGroups = append(rwGroups, rwGroup)
	}
	return rwGroups
}

func groupRWCfgByStarter(rws []*RWCfg) [][]*RWCfg {
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
	var rwGroups [][]*RWCfg
	var group []*RWCfg
	for i := range rws {
		if i == 0 {
			group = append(group, rws[i])
			continue
		}
		curRole := rws[i].Role
		previousRole := rws[i-1].Role
		if previousRole == RWRoleReader && curRole == RWRoleReader ||
			previousRole == RWRoleReader && curRole == RWRoleStarter ||
			previousRole == RWRoleStarter && curRole == RWRoleWriter ||
			previousRole == RWRoleWriter && curRole == RWRoleWriter {
			group = append(group, rws[i])
			continue
		}

		rwGroups = append(rwGroups, group)
		group = make([]*RWCfg, 1)
		group[0] = rws[i]
	}
	if len(group) != 0 {
		rwGroups = append(rwGroups, group)
	}
	return rwGroups
}

func splitRWCfgGroup(rwCfgGroup []*RWCfg) ([]*RWCfg, *RWCfg, []*RWCfg) {
	var (
		readers []*RWCfg
		starter *RWCfg
		writers []*RWCfg
	)
	for _, rwCfg := range rwCfgGroup {
		switch rwCfg.Role {
		case RWRoleReader:
			readers = append(readers, rwCfg)
		case RWRoleStarter:
			starter = rwCfg
		case RWRoleWriter:
			writers = append(writers, rwCfg)
		}
	}
	return readers, starter, writers
}

func hasStarter(rws []*RWCfg) bool {
	for _, rw := range rws {
		if rw.Role == RWRoleStarter {
			return true
		}
	}
	return false
}
