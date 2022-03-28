package v2

import (
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/tail"
)

func init() {
	plugin.RegWithCfg(ReaderTail, func() Common { return NewTail() }, func() any { return NewTailCfg() })
}

const ReaderTail Type = "tail"

type TailCfg struct {
	Path   string `json:"path" yaml:"path"`
	Offset int64  `json:"offset" yaml:"offset"`
}

func NewTailCfg() *TailCfg {
	return &TailCfg{}
}

type Tail struct {
	Reader

	c *TailCfg
	t *tail.Reader
}

func NewTail() *Tail {
	return &Tail{
		Reader: CreateReader(string(ReaderTail)),
		c:      NewTailCfg(),
	}
}

func (t *Tail) Init() error {
	var err error
	t.t, err = tail.NewReader(t.c.Path, t.c.Offset)
	if err != nil {
		return errs.Wrapf(err, "create tail reader failed: %s:%d", t.c.Path, t.c.Offset)
	}

	t.WrapReader(t.t)
	return t.Reader.Init()
}

func (t *Tail) WrapReader(io.ReadCloser) {
	panic(ErrInvalidWrap)
}

func (t *Tail) Type() Type {
	return ReaderTail
}

func (t *Tail) SetCfg(c any) {
	t.c = c.(*TailCfg)
}
