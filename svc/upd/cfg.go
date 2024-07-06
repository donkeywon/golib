package upd

const (
	DefaultDownloadDir       = "/tmp"
	DefaultDownloadRateLimit = 1048576 // 1MB/s
)

type Cfg struct {
	DownloadDir       string `env:"UPD_DOWNLOAD_DIR"        yaml:"downloadPath"`
	DownloadRateLimit int    `env:"UPD_DOWNLOAD_RATE"       yaml:"downloadRateLimit"`
	ExtractDir        string `env:"UPD_EXTRACT_DIR"         validate:"required"      yaml:"extractDir"`
	UpgradeScriptPath string `env:"UPD_UPGRADE_SCRIPT_PATH" validate:"required"      yaml:"upgradeScriptPath"`
}

func NewCfg() *Cfg {
	return &Cfg{
		DownloadDir:       DefaultDownloadDir,
		DownloadRateLimit: DefaultDownloadRateLimit,
	}
}
