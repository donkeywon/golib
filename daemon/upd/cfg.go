package upd

import (
	"os"
	"path/filepath"
)

var (
	DefaultUpgradeOutputPath = filepath.Join(os.TempDir(), "upgrade.out")
)

type Cfg struct {
	UpgradeCmd        []string `env:"UPGRADE_CMD"         long:"upgrade-cmd"         yaml:"upgradeCmd"        description:"exec cmd after download completed"`
	UpgradeOutputPath string   `env:"UPGRADE_OUTPUT_PATH" long:"upgrade-output-path" yaml:"upgradeOutputPath" description:"upgrade cmd output path"`
}

func NewCfg() *Cfg {
	return &Cfg{
		UpgradeOutputPath: DefaultUpgradeOutputPath,
	}
}
