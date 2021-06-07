package pipeline

import "github.com/donkeywon/golib/plugin"

func init() {
	plugin.Register(RWTypeNop, func() interface{} { return NewNopRW() })
	plugin.RegisterCfg(RWTypeNop, func() interface{} { return NewNopRWCfg() })
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
		RW: NewBaseRW(string(RWTypeNop)),
	}
}

func (n *NopRW) Type() interface{} {
	return RWTypeNop
}

func (n *NopRW) GetCfg() interface{} {
	return n.NopRWCfg
}
