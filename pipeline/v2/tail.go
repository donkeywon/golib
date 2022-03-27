package v2

import (
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/tail"
)

func init() {
	plugin.RegWithCfg(ReaderTail, func() any { return NewTail() }, func() any { return NewTailCfg() })
}

const ReaderTail ReaderType = "tail"

type TailCfg struct {
	Path   string `json:"path" yaml:"path"`
	Offset int64  `json:"offset" yaml:"offset"`
}

func NewTailCfg() *TailCfg {
	return &TailCfg{}
}

type Tail struct {
	Reader
	*TailCfg

	t *tail.Reader
}

func NewTail() *Tail {
	return &Tail{
		Reader:  CreateReader(string(ReaderTail)),
		TailCfg: NewTailCfg(),
	}
}

func (t *Tail) Init() error {
	var err error
	t.t, err = tail.NewReader(t.TailCfg.Path, t.Offset)
	if err != nil {
		return errs.Wrapf(err, "create tail reader failed: %s:%d", t.TailCfg.Path, t.Offset)
	}

	t.Wrap(t.t)
	return t.Reader.Init()
}

func (t *Tail) Wrap(io.ReadCloser) {
	panic(ErrInvalidWrap)
}

func (t *Tail) Type() any {
	return ReaderTail
}

func (t *Tail) GetCfg() any {
	return t.TailCfg
}
