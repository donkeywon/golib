package pipeline

import (
	"context"
	"errors"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/pipeline/rw"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

func init() {
	plugin.RegWithCfg(PluginTypeRWGroup, NewRWGroup, NewRWGroupCfg)
}

const PluginTypeRWGroup plugin.Type = "rwg"

type RWGroupCfg struct {
	Readers []*rw.Cfg `json:"readers" yaml:"readers"`
	Starter *rw.Cfg   `json:"starter" yaml:"starter"`
	Writers []*rw.Cfg `json:"writers" yaml:"writers"`
}

func NewRWGroupCfg() *RWGroupCfg {
	return &RWGroupCfg{}
}

func (c *RWGroupCfg) SetStarter(typ rw.Type, cfg any, commonCfg *rw.ExtraCfg) *RWGroupCfg {
	c.Starter = &rw.Cfg{
		Type:     typ,
		Cfg:      cfg,
		ExtraCfg: commonCfg,
		Role:     rw.RoleStarter,
	}
	return c
}

func (c *RWGroupCfg) FromReader(typ rw.Type, cfg any, commonCfg *rw.ExtraCfg) *RWGroupCfg {
	c.Readers = append(c.Readers, &rw.Cfg{
		Type:     typ,
		Cfg:      cfg,
		ExtraCfg: commonCfg,
		Role:     rw.RoleReader,
	})
	return c
}

func (c *RWGroupCfg) ToWriter(typ rw.Type, cfg any, commonCfg *rw.ExtraCfg) *RWGroupCfg {
	c.Writers = append(c.Writers, &rw.Cfg{
		Type:     typ,
		Cfg:      cfg,
		ExtraCfg: commonCfg,
		Role:     rw.RoleWriter,
	})
	return c
}

type RWGroup struct {
	runner.Runner
	*RWGroupCfg

	readers []rw.RW
	starter rw.RW
	writers []rw.RW
}

func NewRWGroup() *RWGroup {
	return &RWGroup{
		Runner:     runner.Create(string(PluginTypeRWGroup)),
		RWGroupCfg: NewRWGroupCfg(),
	}
}

func (g *RWGroup) Init() error {
	g.starter = g.createRW(g.RWGroupCfg.Starter)

	g.readers = make([]rw.RW, len(g.RWGroupCfg.Readers))
	for i, cfg := range g.RWGroupCfg.Readers {
		g.readers[i] = g.createRW(cfg)
	}

	g.writers = make([]rw.RW, len(g.RWGroupCfg.Writers))
	for i, cfg := range g.RWGroupCfg.Writers {
		g.writers[i] = g.createRW(cfg)
	}

	return g.Runner.Init()
}

func (g *RWGroup) Start() error {
	defer func() {
		defer func() {
			pe := recover()
			if pe != nil {
				g.AppendError(errs.PanicToErrWithMsg(pe, "panic on closing starter"))
			}
		}()

		pe := recover()
		if pe != nil {
			g.AppendError(errs.PanicToErrWithMsg(pe, "panic on init all rws and start"))
		}

		err := g.starter.Close()
		if err != nil {
			g.AppendError(errs.Wrap(err, "close starter failed"))
		}
	}()

	var err error

	err = g.initReaders()
	if err != nil {
		return errs.Wrap(err, "init readers failed")
	}

	if len(g.readers) > 0 {
		lastReader := g.readers[len(g.readers)-1]
		g.starter.NestReader(lastReader)
	}

	err = g.initWriters()
	if err != nil {
		return errs.Wrap(err, "init writers failed")
	}

	if len(g.writers) > 0 {
		firstWriter := g.writers[0]
		g.starter.NestWriter(firstWriter)
	}

	err = runner.Init(g.starter)
	if err != nil {
		return errs.Wrapf(err, "init starter %s failed", g.starter.Type())
	}

	runner.Start(g.starter)

	err = g.starter.Err()
	if !errors.Is(err, context.Canceled) {
		return errs.Wrap(err, "starter failed")
	}

	return nil
}

func (g *RWGroup) Stop() error {
	runner.Stop(g.starter)
	return nil
}

func (g *RWGroup) Size() int {
	return len(g.readers) + 1 + len(g.writers)
}

func (g *RWGroup) Starter() rw.RW {
	return g.starter
}

func (g *RWGroup) Readers() []rw.RW {
	return g.readers
}

func (g *RWGroup) Writers() []rw.RW {
	return g.writers
}

func (g *RWGroup) Type() plugin.Type {
	return PluginTypeRWGroup
}

func (g *RWGroup) LastWriter() rw.RW {
	if len(g.writers) == 0 {
		return nil
	}
	return g.writers[len(g.writers)-1]
}

func (g *RWGroup) FirstReader() rw.RW {
	if len(g.readers) == 0 {
		return nil
	}
	return g.readers[0]
}

func (g *RWGroup) createRW(rwCfg *rw.Cfg) rw.RW {
	rw := rw.CreateRW(rwCfg)
	rw.Inherit(g)
	return rw
}

func (g *RWGroup) initReaders() error {
	var err error
	for i := 0; i < len(g.readers); i++ {
		if i > 0 {
			g.readers[i].NestReader(g.readers[i-1])
		}

		err = runner.Init(g.readers[i])
		if err != nil {
			err = errs.Wrapf(err, "init reader(%d) %s failed", i, g.readers[i].Type())
			closeErr := g.readers[i].Close()
			if closeErr != nil {
				err = errors.Join(err, errs.Wrapf(err, "close reader(%d) %s failed", i, g.readers[i].Type()))
			}
			return err
		}
	}

	return nil
}

func (g *RWGroup) initWriters() error {
	var err error
	for i := len(g.writers) - 1; i >= 0; i-- {
		if i < len(g.writers)-1 {
			g.writers[i].NestWriter(g.writers[i+1])
		}

		err = runner.Init(g.writers[i])
		if err != nil {
			err = errs.Wrapf(err, "init writer(%d) %s failed", i, g.writers[i].Type())
			closeErr := g.writers[i].Close()
			if closeErr != nil {
				err = errors.Join(err, errs.Wrapf(err, "close writer(%d) %s failed", i, g.writers[i].Type()))
			}
			return err
		}
	}
	return nil
}
