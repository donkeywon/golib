package pipeline

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/tail"
)

func init() {
	plugin.Register(RWTypeTail, func() interface{} { return NewTailRW() })
	plugin.RegisterCfg(RWTypeTail, func() interface{} { return NewTailRWCfg() })
}

const RWTypeTail RWType = "tail"

type TailRWCfg struct {
	Path string `json:"path" yaml:"path"`
	Pos  int64  `json:"pos"  yaml:"pos"`
}

func NewTailRWCfg() *TailRWCfg {
	return &TailRWCfg{}
}

type TailRW struct {
	RW
	*TailRWCfg

	t *tail.Reader
}

func NewTailRW() *TailRW {
	return &TailRW{
		RW: NewBaseRW(string(RWTypeTail)),
	}
}

func (t *TailRW) Init() error {
	if !t.IsReader() {
		return errs.New("tailRW must be Reader")
	}

	var err error
	t.t, err = tail.NewReader(t.Path, t.Pos)
	if err != nil {
		return errs.Wrap(err, "create tail reader fail")
	}

	_ = t.NestReader(t.t)

	return t.RW.Init()
}

func (t *TailRW) Stop() error {
	return t.t.Close()
}

func (t *TailRW) Type() interface{} {
	return RWTypeTail
}

func (t *TailRW) GetCfg() interface{} {
	return t.TailRWCfg
}
