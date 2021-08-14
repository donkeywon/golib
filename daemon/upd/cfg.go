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
	DownloadDir             string `env:"UPD_DOWNLOAD_DIR"              flag-long:"upd-download-dir"        yaml:"downloadDir"       flag-description:"new version package download destination directory"`
	DownloadRateLimit       int    `env:"UPD_DOWNLOAD_RATE"             flag-long:"upd-download-rate-limit" yaml:"downloadRateLimit" flag-description:"download rate limit in Byte/s"`
	ExtractDir              string `env:"UPD_EXTRACT_DIR"               flag-long:"upd-extract-dir"         validate:"required"      yaml:"extractDir" flag-description:"extract package destination directory after download complete, starting with / means absolute path, otherwise means path.Join(downloadDir, extractDir)"`
	UpgradeDeployScriptPath string `env:"UPD_DEPLOY_SCRIPT_PATH"        flag-long:"upd-deploy-script-path"  validate:"required"      yaml:"deployScriptPath" flag-description:"path of the script to be executed after extract, starting with / means absolute path, otherwise means path.Join(extractDir, deployScriptPath)"`
	UpgradeStartScriptPath  string `env:"UPD_UPGRADE_START_SCRIPT_PATH" flag-long:"upd-start-script-path"   yaml:"upgradeStartScriptPath" flag-description:"path of the script to be executed after deploy is complete, will not executed when empty, starting with / means absolute path, otherwise means path.Join(extractDir, startScriptPath)"`
	HashAlgo                string `env:"UPD_HASH_ALGO"                 flag-long:"upd-hash-algo"           yaml:"hashAlgo" flag-description:"hash algorithm for checking the checksum of the new version package"`
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
