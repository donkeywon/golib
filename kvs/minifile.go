package kvs

import (
	"os"

	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegisterWithCfg(TypeMiniFile, func() interface{} { return NewMiniFileKVS() }, func() interface{} { return NewMiniFileKVSCfg() })
}

const TypeMiniFile Type = "minifile"

type MiniFileKVSCfg struct {
	Path string `json:"path" yaml:"path"`
	Perm uint32 `json:"perm" yaml:"perm"`
}

func NewMiniFileKVSCfg() *MiniFileKVSCfg {
	return &MiniFileKVSCfg{}
}

type MiniFileKVS struct {
	*InMemKVS
	Cfg *MiniFileKVSCfg

	f *os.File
}

func NewMiniFileKVS() *MiniFileKVS {
	return &MiniFileKVS{
		InMemKVS: NewInMemKVS(),
	}
}

func (m *MiniFileKVS) Store(k string, v any) {
	
}