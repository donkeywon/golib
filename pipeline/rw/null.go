package rw

import (
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(TypeNull, func() any { return NewNull() }, func() any { return NewNullCfg() })
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

const TypeNull Type = "null"

type NullCfg struct {
}

func NewNullCfg() *NullCfg {
	return &NullCfg{}
}

type Null struct {
	RW
	*NullCfg
}

func NewNull() *Null {
	return &Null{
		RW: CreateBase(string(TypeFile)),
	}
}

func (f *Null) Init() error {
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

func (f *Null) Type() any {
	return TypeFile
}

func (f *Null) GetCfg() any {
	return f.NullCfg
}
