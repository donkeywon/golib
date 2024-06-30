package pipeline

import (
	"errors"
	"io"
	"sync"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util"
)

func init() {
	plugin.Register(PluginTypePipeline, func() interface{} { return New() })
	plugin.RegisterCfg(PluginTypePipeline, func() interface{} { return NewCfg() })
}

const PluginTypePipeline plugin.Type = "pipeline"

type RWCfg struct {
	Type      RWType       `json:"rwType"    validate:"required" yaml:"rwType"`
	Cfg       interface{}  `json:"cfg"       validate:"required" yaml:"cfg"`
	CommonCfg *RWCommonCfg `json:"commonCfg" yaml:"commonCfg"`
	Role      RWRole       `json:"role"      validate:"required" yaml:"role"`
}

type Cfg struct {
	RWs []*RWCfg `json:"rws" validate:"min=1" yaml:"rws"`
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

type Pipeline struct {
	runner.Runner
	*Cfg

	rwGroups []*RWGroup
}

func New() *Pipeline {
	return &Pipeline{
		Runner: runner.NewBase("pipeline"),
		Cfg:    NewCfg(),
	}
}

func (p *Pipeline) Init() error {
	err := util.V.Struct(p.Cfg)
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
		runner.Inherit(rwGroup, p)
		err = runner.Init(rwGroup)
		if err != nil {
			return errs.Wrapf(err, "init rw group fail: %d", i)
		}
	}

	for i := range len(p.rwGroups) - 1 {
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
			errMu.Lock()
			err = errors.Join(err, rwGroup.Err())
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
