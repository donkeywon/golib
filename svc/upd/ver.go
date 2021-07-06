package upd

import "github.com/donkeywon/golib/pipeline"

type VerInfo struct {
	Filename string             `json:"filename" yaml:"filename"`
	Ver      string             `json:"ver"      yaml:"ver"`
	StoreCfg *pipeline.StoreCfg `json:"storeCfg" yaml:"storeCfg"`
}
