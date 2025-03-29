package rw

import "github.com/donkeywon/golib/plugin"

func init() {
	plugin.RegWithCfg(TypeNop, func() RW { return NewNop() }, func() any { return NewNopCfg() })
}

const TypeNop Type = "nop"

type NopCfg struct{}

func NewNopCfg() *NopCfg {
	return &NopCfg{}
}

type Nop struct {
	RW
	*NopCfg
}

func NewNop() *Nop {
	return &Nop{
		RW: CreateBase(string(TypeNop)),
	}
}

func (n *Nop) Type() Type {
	return TypeNop
}
