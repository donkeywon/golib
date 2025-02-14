package rw

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/tail"
)

func init() {
	plugin.RegWithCfg(TypeTail, func() any { return NewTail() }, func() any { return NewTailCfg() })
}

const TypeTail Type = "tail"

type TailCfg struct {
	Path string `json:"path" yaml:"path"`
	Pos  int64  `json:"pos"  yaml:"pos"`
}

func NewTailCfg() *TailCfg {
	return &TailCfg{}
}

type Tail struct {
	RW
	*TailCfg

	t *tail.Reader
}

func NewTail() *Tail {
	return &Tail{
		RW: CreateBase(string(TypeTail)),
	}
}

func (t *Tail) Init() error {
	if !t.IsReader() {
		return errs.New("tail must be Reader")
	}

	var err error
	t.t, err = tail.NewReader(t.Path, t.Pos)
	if err != nil {
		return errs.Wrap(err, "create tail reader failed")
	}

	t.NestReader(t.t)

	return t.RW.Init()
}

func (t *Tail) Stop() error {
	return t.t.Close()
}

func (t *Tail) Type() any {
	return TypeTail
}

func (t *Tail) GetCfg() any {
	return t.TailCfg
}
