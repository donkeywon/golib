package pipeline

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegisterWithCfg(RWTypeNull, func() interface{} { return NewNullRW() }, func() interface{} { return NewNullRWCfg() })
}

type null struct{}

func (n *null) Write(b []byte) (int, error) {
	return len(b), nil
}

func (n *null) Read(_ []byte) (int, error) {
	return 0, nil
}

func (n *null) Close() error {
	return nil
}

var nrw = &null{}

const RWTypeNull RWType = "null"

type NullRWCfg struct {
}

func NewNullRWCfg() *NullRWCfg {
	return &NullRWCfg{}
}

type NullRW struct {
	RW
	*NullRWCfg
}

func NewNullRW() *NullRW {
	return &NullRW{
		RW: CreateBaseRW(string(RWTypeFile)),
	}
}

func (f *NullRW) Init() error {
	if f.IsStarter() {
		return errs.New("nullRW cannot be Starter")
	}

	if f.IsReader() {
		f.NestReader(nrw)
	} else {
		f.NestWriter(nrw)
	}

	return f.RW.Init()
}

func (f *NullRW) Type() interface{} {
	return RWTypeFile
}

func (f *NullRW) GetCfg() interface{} {
	return f.NullRWCfg
}
