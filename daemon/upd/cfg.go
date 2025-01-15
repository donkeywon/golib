package upd

const (
	DefaultDownloadDir             = "/tmp"
	DefaultExtractDir              = "golib-upgrade-svc-tmp"
	DefaultUpgradeDeployScriptPath = "bin/upgrade_deploy.sh"
	DefaultUpgradeStartScriptPath  = "bin/upgrade_start.sh"
	DefaultHashAlgo                = "xxh3"
)

type Cfg struct {
	DownloadDir             string `env:"UPD_DOWNLOAD_DIR"              yaml:"downloadDir"            flag-long:"upd-download-dir"        flag-description:"new version package download destination directory"`
	ExtractDir              string `env:"UPD_EXTRACT_DIR"               yaml:"extractDir"             flag-long:"upd-extract-dir"         flag-description:"extract package destination directory after download complete, starting with / means absolute path, otherwise means path.Join(downloadDir, extractDir)"`
	UpgradeDeployScriptPath string `env:"UPD_DEPLOY_SCRIPT_PATH"        yaml:"deployScriptPath"       flag-long:"upd-deploy-script-path"  flag-description:"path of the script to be executed after extract, starting with / means absolute path, otherwise means path.Join(extractDir, deployScriptPath)"`
	UpgradeStartScriptPath  string `env:"UPD_UPGRADE_START_SCRIPT_PATH" yaml:"upgradeStartScriptPath" flag-long:"upd-start-script-path"   flag-description:"path of the script to be executed after deploy is complete, will not executed when empty, starting with / means absolute path, otherwise means path.Join(extractDir, startScriptPath)"`
	HashAlgo                string `env:"UPD_HASH_ALGO"                 yaml:"hashAlgo"               flag-long:"upd-hash-algo"           flag-description:"hash algorithm for checking the checksum of the new version package"`
}

func NewCfg() *Cfg {
	return &Cfg{
		DownloadDir:             DefaultDownloadDir,
		ExtractDir:              DefaultExtractDir,
		UpgradeDeployScriptPath: DefaultUpgradeDeployScriptPath,
		UpgradeStartScriptPath:  DefaultUpgradeStartScriptPath,
		HashAlgo:                DefaultHashAlgo,
	}
}
