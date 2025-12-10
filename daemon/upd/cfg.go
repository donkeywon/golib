package upd

import (
	"os"
	"path/filepath"
)

var (
	DefaultUpgradeOutputPath = filepath.Join(os.TempDir(), "upgrade.out")
)

type Cfg struct {
	UpgradeCmd        []string `yaml:"upgradeCmd"`
	UpgradeOutputPath string   `yaml:"upgradeOutputPath"`
}

func NewCfg() *Cfg {
	return &Cfg{
		UpgradeOutputPath: DefaultUpgradeOutputPath,
	}
}
