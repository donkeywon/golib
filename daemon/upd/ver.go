package upd

import (
	"github.com/donkeywon/golib/pipeline/rw"
)

type VerInfo struct {
	Ver             string   `json:"ver"             yaml:"ver"             validate:"required"`
	StoreCfg        *rw.Cfg  `json:"storeCfg"        yaml:"storeCfg"        validate:"required"`
	DownloadDstPath string   `json:"downloadDstPath" yaml:"downloadDstPath" validate:"required"`
	DeployCmd       []string `json:"deployCmd"       yaml:"deployCmd"`
	StartCmd        []string `json:"startCmd"        yaml:"startCmd"`
}
