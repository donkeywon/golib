package upd

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/buildinfo"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/pipeline"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/paths"
)

const DaemonTypeUpd boot.DaemonType = "upd"

var _u = New()

type Upd struct {
	runner.Runner
	*Cfg

	upgrading          atomic.Bool
	upgradingBlockChan chan struct{}
}

func D() *Upd {
	return _u
}

func New() *Upd {
	return &Upd{
		Runner:             runner.Create(string(DaemonTypeUpd)),
		upgradingBlockChan: make(chan struct{}),
	}
}

func (u *Upd) Stop() error {
	u.Cancel()
	if u.isUpgrading() {
		<-u.upgradingBlockChan
	}
	return u.Runner.Stop()
}

func (u *Upd) Type() interface{} {
	return DaemonTypeUpd
}

func (u *Upd) GetCfg() interface{} {
	return u.Cfg
}

func (u *Upd) markUpgrading() bool {
	return u.upgrading.CompareAndSwap(false, true)
}

func (u *Upd) unmarkUpgrading() {
	u.upgrading.Store(false)
}

func (u *Upd) isUpgrading() bool {
	return u.upgrading.Load()
}

func (u *Upd) Upgrade(vi *VerInfo) {
	go func() {
		err := u.upgrade(vi)
		if err != nil {
			u.Error("upgrade fail", err)
		}

		select {
		case <-u.Stopping():
			u.Info("upgrade stopped due to stopping")
			close(u.upgradingBlockChan)
		default:
		}
	}()
}

func (u *Upd) upgrade(vi *VerInfo) error {
	if !u.markUpgrading() {
		return errs.New("already upgrading")
	}
	defer u.unmarkUpgrading()

	u.Info("start upgrade", "cur_ver", buildinfo.Version, "new_ver", vi.Ver)

	var err error
	downloadPath := filepath.Join(u.DownloadDir, vi.Filename)
	if paths.FileExist(downloadPath) {
		u.Info("download dst path exists, remove it", "path", downloadPath)
		err = os.RemoveAll(downloadPath)
		if err != nil {
			return errs.Wrapf(err, "remove exists download dst path fail: %s", downloadPath)
		}
	}

	err = downloadPackage(u.DownloadDir, vi.Filename, u.DownloadRateLimit, vi.StoreCfg)
	if err != nil {
		return errs.Wrap(err, "download package fail")
	}
	u.Info("download package done", "cfg", u.Cfg, "ver_info", vi)

	extractDir := strings.TrimSpace(u.Cfg.ExtractDir)
	if !strings.HasPrefix(extractDir, "/") {
		extractDir = filepath.Join(u.DownloadDir, extractDir)
	}
	stdout, stderr, err := extractPackage(downloadPath, extractDir)
	if err != nil {
		return errs.Wrapf(err, "extract package fail, stdout: %v, stderr: %v", stdout, stderr)
	}
	u.Info("extract package done", "extract_dir", extractDir, "stdout", stdout, "stderr", stderr)

	stopped := make(chan struct{})
	go func() {
		u.Info("close all svc")
		runner.Stop(u.Parent())
		if u.Parent().ChildrenErr() != nil {
			u.Error("close all svc error occurred", u.Parent().ChildrenErr())
		}
		close(stopped)
	}()

	select {
	case <-time.After(time.Minute):
		u.Error("close all svc timeout", u.Parent().ChildrenErr())
	case <-stopped:
		u.Info("all svc closed")
	}

	upgradeDeployScriptPath := strings.TrimSpace(u.Cfg.UpgradeDeployScriptPath)
	if !strings.HasPrefix(upgradeDeployScriptPath, "/") {
		upgradeDeployScriptPath = filepath.Join(extractDir, upgradeDeployScriptPath)
	}
	if !paths.FileExist(upgradeDeployScriptPath) {
		return errs.Errorf("upgrade deploy script not exists: %s", upgradeDeployScriptPath)
	}
	res, err := cmd.Run("bash", upgradeDeployScriptPath)
	u.Info("exec upgrade deploy script", "result", res, "err", err)
	if res.ExitCode != 0 || err != nil {
		os.Exit(1)
	}

	upgradeStartScriptPath := strings.TrimSpace(u.Cfg.UpgradeStartScriptPath)
	if upgradeStartScriptPath != "" {
		if !strings.HasPrefix(upgradeStartScriptPath, "/") {
			upgradeStartScriptPath = filepath.Join(extractDir, upgradeStartScriptPath)
		}
		if !paths.FileExist(upgradeStartScriptPath) {
			return errs.Errorf("upgrade start script not exists: %s", upgradeStartScriptPath)
		}

		res, err = cmd.Run("bash", upgradeStartScriptPath)
		u.Info("exec upgrade start script", "result", res, "err", err)
		if res.ExitCode != 0 || err != nil {
			os.Exit(1)
		}
	}

	os.Exit(0)
	return nil
}

func downloadPackage(downloadDir string, filename string, ratelimitN int, storeCfg *pipeline.RWCfg) error {
	cfg := pipeline.NewCfg().
		AddCfg(storeCfg).
		Add(
			pipeline.RWRoleStarter,
			pipeline.RWTypeCopy,
			&pipeline.CopyRWCfg{BufSize: 1024 * 1024},
			nil,
		).
		Add(
			pipeline.RWRoleWriter,
			pipeline.RWTypeFile,
			&pipeline.FileRWCfg{Path: filepath.Join(downloadDir, filename)},
			nil,
		)

	p := pipeline.New()
	p.Cfg = cfg
	p.Inherit(D())
	err := runner.Init(p)
	if err != nil {
		return errs.Wrap(err, "init download pipeline fail")
	}

	runner.Start(p)
	return p.Err()
}

func extractPackage(filepath string, dstDir string) ([]string, []string, error) {
	if !paths.DirExist(dstDir) {
		return nil, nil, errs.Errorf("extract dst dir not exists: %s", dstDir)
	}
	res, err := cmd.Run("tar", "xf", filepath, "-C", dstDir)
	if res != nil {
		return res.Stdout, res.Stderr, err
	}
	return nil, nil, err
}
