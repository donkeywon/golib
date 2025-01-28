package pipeline

import "github.com/donkeywon/golib/plugin"

func init() {
	plugin.RegWithCfg(RWTypeNop, func() any { return NewNopRW() }, func() any { return NewNopRWCfg() })
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

func (n *NopRW) Type() any {
	return RWTypeNop
}

func (n *NopRW) GetCfg() any {
	return n.NopRWCfg
}
