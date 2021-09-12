package pipeline

import "github.com/donkeywon/golib/plugin"

func init() {
	plugin.RegisterWithCfg(RWTypeNop, func() interface{} { return NewNopRW() }, func() interface{} { return NewNopRWCfg() })
}

const RWTypeNop RWType = "nop"

type NopRWCfg struct{}

func NewNopRWCfg() *NopRWCfg {
	return &NopRWCfg{}
}

type NopRW struct {
	RW
	*NopRWCfg
}

func NewNopRW() *NopRW {
	return &NopRW{
		RW: CreateBaseRW(string(RWTypeNop)),
	}
}

func (n *NopRW) Type() interface{} {
	return RWTypeNop
}

func (n *NopRW) GetCfg() interface{} {
	return n.NopRWCfg
}
