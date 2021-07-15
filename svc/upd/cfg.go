package upd

const (
	DefaultDownloadDir             = "/tmp"
	DefaultDownloadRateLimit       = 1048576 // 1MB/s
	DefaultExtractDir              = "golib-upgrade-svc-tmp"
	DefaultUpgradeDeployScriptPath = "bin/upgrade_deploy.sh"
	DefaultUpgradeStartScriptPath  = "bin/upgrade_start.sh"
	DefaultHashAlgo                = "xxh3"
)

type Cfg struct {
	DownloadDir             string `env:"UPD_DOWNLOAD_DIR"              yaml:"downloadPath"`
	DownloadRateLimit       int    `env:"UPD_DOWNLOAD_RATE"             yaml:"downloadRateLimit"`
	ExtractDir              string `env:"UPD_EXTRACT_DIR"               validate:"required"            yaml:"extractDir"`       // starting with / means absolute path, otherwise means filepath.Join(DownloadDir, ExtractDir)
	UpgradeDeployScriptPath string `env:"UPD_DEPLOY_SCRIPT_PATH"        validate:"required"            yaml:"deployScriptPath"` // starting with / means absolute path, otherwise means filepath.Join(ExtractDir, DeployScriptPath)
	UpgradeStartScriptPath  string `env:"UPD_UPGRADE_START_SCRIPT_PATH" yaml:"upgradeStartScriptPath"`                          // not exec when empty, starting with / means absolute path, otherwise means filepath.Join(ExtractDir, UpgradeStartScriptPath)
	HashAlgo                string `env:"UPD_HASH_ALGO"                 yaml:"hashAlgo"`
}

func NewCfg() *Cfg {
	return &Cfg{
		DownloadDir:             DefaultDownloadDir,
		DownloadRateLimit:       DefaultDownloadRateLimit,
		ExtractDir:              DefaultExtractDir,
		UpgradeDeployScriptPath: DefaultUpgradeDeployScriptPath,
		UpgradeStartScriptPath:  DefaultUpgradeStartScriptPath,
		HashAlgo:                DefaultHashAlgo,
	}
}
