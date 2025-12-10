package upd

import (
	"github.com/donkeywon/golib/pipeline"
)

type VerInfo struct {
	Ver               string              `json:"ver"               yaml:"ver"             validate:"required"`
	StoreCfg          *pipeline.ReaderCfg `json:"storeCfg"          yaml:"storeCfg"        validate:"required"`
	DownloadDstPath   string              `json:"downloadDstPath"   yaml:"downloadDstPath" validate:"required"`
	UpgradeCmd        []string            `json:"upgradeCmd"        yaml:"upgradeCmd"`
	UpgradeOutputPath string              `json:"upgradeOutputPath" yaml:"upgradeOutputPath"`
}
