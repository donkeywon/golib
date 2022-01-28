package pipeline

import (
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(RWTypeNull, func() any { return NewNullRW() }, func() any { return NewNullRWCfg() })
}

type null struct{}

func (n *null) Write(b []byte) (int, error) {
	return len(b), nil
}

func (n *null) Read(_ []byte) (int, error) {
	return 0, io.EOF
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

func (f *NullRW) Type() any {
	return RWTypeFile
}

func (f *NullRW) GetCfg() any {
	return f.NullRWCfg
}
