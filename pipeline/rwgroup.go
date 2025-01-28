package pipeline

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

func init() {
	plugin.Reg(PluginTypeRWGroup, func() any { return NewRWGroup() })
	plugin.RegCfg(PluginTypeRWGroup, func() any { return NewRWGroupCfg() })
}

const PluginTypeRWGroup plugin.Type = "rwgroup"

type RWGroupCfg struct {
	Readers []*RWCfg `json:"readers" yaml:"readers"`
	Starter *RWCfg   `json:"starter" yaml:"starter"`
	Writers []*RWCfg `json:"writers" yaml:"writers"`
}

func NewRWGroupCfg() *RWGroupCfg {
	return &RWGroupCfg{}
}

func (c *RWGroupCfg) SetStarter(typ RWType, cfg any, commonCfg *RWCommonCfg) *RWGroupCfg {
	c.Starter = &RWCfg{
		Type:      typ,
		Cfg:       cfg,
		CommonCfg: commonCfg,
		Role:      RWRoleStarter,
	}
	return c
}

func (c *RWGroupCfg) FromReader(typ RWType, cfg any, commonCfg *RWCommonCfg) *RWGroupCfg {
	c.Readers = append(c.Readers, &RWCfg{
		Type:      typ,
		Cfg:       cfg,
		CommonCfg: commonCfg,
		Role:      RWRoleReader,
	})
	return c
}

func (c *RWGroupCfg) ToWriter(typ RWType, cfg any, commonCfg *RWCommonCfg) *RWGroupCfg {
	c.Writers = append(c.Writers, &RWCfg{
		Type:      typ,
		Cfg:       cfg,
		CommonCfg: commonCfg,
		Role:      RWRoleWriter,
	})
	return c
}

type RWGroup struct {
	runner.Runner
	*RWGroupCfg

	readers []RW
	starter RW
	writers []RW
}

func NewRWGroup() *RWGroup {
	return &RWGroup{
		Runner:     runner.Create("rwg"),
		RWGroupCfg: NewRWGroupCfg(),
	}
}

func (g *RWGroup) Init() error {
	var err error
	g.starter, err = g.createRW(g.RWGroupCfg.Starter)
	if err != nil {
		return errs.Wrapf(err, "create starter rw(%s) fail", g.RWGroupCfg.Starter.Type)
	}

	g.readers = make([]RW, len(g.RWGroupCfg.Readers))
	for i, cfg := range g.RWGroupCfg.Readers {
		g.readers[i], err = g.createRW(cfg)
		if err != nil {
			return errs.Wrapf(err, "create %d reader rw(%s) fail", i, cfg.Type)
		}
	}

	g.writers = make([]RW, len(g.RWGroupCfg.Writers))
	for i, cfg := range g.RWGroupCfg.Writers {
		g.writers[i], err = g.createRW(cfg)
		if err != nil {
			return errs.Wrapf(err, "create %d writer rw(%s) fail", i, cfg.Type)
		}
	}

	return g.Runner.Init()
}

func (g *RWGroup) Start() error {
	defer func() {
		defer func() {
			err := recover()
			if err != nil {
				g.AppendError(errs.PanicToErrWithMsg(err, "panic when close starter"))
			}
		}()

		err := g.starter.Close()
		if err != nil {
			g.AppendError(errs.Wrap(err, "close starter fail"))
		}
	}()

	var err error

	err = g.initReaders()
	if err != nil {
		return errs.Wrap(err, "init readers fail")
	}

	if len(g.readers) > 0 {
		lastReader := g.readers[len(g.readers)-1]
		err = g.starter.NestReader(lastReader)
		if err != nil {
			return errs.Wrapf(err, "starter %s nest reader %s(%d) fail", g.starter.Type(), lastReader.Type(), len(g.readers)-1)
		}
	}

	err = g.initWriters()
	if err != nil {
		return errs.Wrap(err, "init writers fail")
	}

	if len(g.writers) > 0 {
		firstWriter := g.writers[0]
		err = g.starter.NestWriter(firstWriter)
		if err != nil {
			return errs.Wrapf(err, "starter %s nest writer %s(%d) fail", g.starter.Type(), firstWriter.Type(), 0)
		}
	}

	err = runner.Init(g.starter)
	if err != nil {
		return errs.Wrapf(err, "init starter %s fail", g.starter.Type())
	}

	runner.Start(g.starter)

	return g.starter.Err()
}

func (g *RWGroup) Stop() error {
	runner.Stop(g.starter)
	return nil
}

func (g *RWGroup) Size() int {
	return len(g.readers) + 1 + len(g.writers)
}

func (g *RWGroup) Starter() RW {
	return g.starter
}

func (g *RWGroup) Readers() []RW {
	return g.readers
}

func (g *RWGroup) Writers() []RW {
	return g.writers
}

func (g *RWGroup) LastWriter() RW {
	if len(g.writers) == 0 {
		return nil
	}
	return g.writers[len(g.writers)-1]
}

func (g *RWGroup) FirstReader() RW {
	if len(g.readers) == 0 {
		return nil
	}
	return g.readers[0]
}

func (g *RWGroup) createRW(rwCfg *RWCfg) (RW, error) {
	rw, err := CreateRW(rwCfg)
	if err != nil {
		return nil, errs.Wrapf(err, "create rw(%s) fail", rwCfg.Type)
	}
	rw.Inherit(g)
	return rw, nil
}

func (g *RWGroup) initReaders() error {
	var err error
	for i := 0; i < len(g.readers); i++ {
		if i > 0 {
			err = g.readers[i].NestReader(g.readers[i-1])
			if err != nil {
				return errs.Wrapf(err, "reader(%d) %s nest reader(%d) %s fail", i, g.readers[i].Type(), i+1, g.readers[i+1].Type())
			}
		}

		err = runner.Init(g.readers[i])
		if err != nil {
			return errs.Wrapf(err, "init reader(%d) %s fail", i, g.readers[i].Type())
		}
	}

	return nil
}

func (g *RWGroup) initWriters() error {
	var err error
	for i := len(g.writers) - 1; i >= 0; i-- {
		if i < len(g.writers)-1 {
			err = g.writers[i].NestWriter(g.writers[i+1])
			if err != nil {
				return errs.Wrapf(err, "writer(%d) %s nest writer(%d) %s fail", i, g.writers[i].Type(), i+1, g.writers[i+1].Type())
			}
		}

		err = runner.Init(g.writers[i])
		if err != nil {
			return errs.Wrapf(err, "init writer(%d) %s fail", i, g.writers[i].Type())
		}
	}
	return nil
}
