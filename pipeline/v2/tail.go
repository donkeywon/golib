package v2

import (
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/tail"
)

const TypeTail ReaderType = "tail"

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
		Reader:  CreateReader(string(TypeTail)),
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
	return TypeTail
}

func (t *Tail) GetCfg() any {
	return t.TailCfg
}
