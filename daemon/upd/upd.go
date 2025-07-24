package upd

import (
	"os"
	"sync/atomic"
	"time"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/buildinfo"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/pipeline"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/paths"
	"github.com/donkeywon/golib/util/v"
)

const DaemonTypeUpd boot.DaemonType = "upd"

var D Upd = New()

type Upd interface {
	boot.Daemon
	Upgrade(vi *VerInfo)
}

// upd must be first daemon
type upd struct {
	runner.Runner
	*Cfg

	upgrading          atomic.Bool
	upgradingBlockChan chan struct{}
	allDoneExceptMe    chan struct{}
}

func New() Upd {
	return &upd{
		Runner:             runner.Create(string(DaemonTypeUpd)),
		upgradingBlockChan: make(chan struct{}),
		allDoneExceptMe:    make(chan struct{}),
	}
}

func (u *upd) Stop() error {
	u.Cancel()
	if u.isUpgrading() {
		// at this point, all daemon done except upd
		close(u.allDoneExceptMe)
		<-u.upgradingBlockChan
	}
	return u.Runner.Stop()
}

func (u *upd) markUpgrading() bool {
	return u.upgrading.CompareAndSwap(false, true)
}

func (u *upd) unmarkUpgrading() {
	u.upgrading.Store(false)
}

func (u *upd) isUpgrading() bool {
	return u.upgrading.Load()
}

func (u *upd) Upgrade(vi *VerInfo) {
	go func() {
		success := u.upgrade(vi)

		// at this point, success is always fail, current process will exit if upgrade success
		if !success {
		}

		select {
		case <-u.Stopping():
			u.Info("upgrade stopped due to stopping")
			close(u.upgradingBlockChan)
		default:
		}
	}()
}

func (u *upd) upgrade(vi *VerInfo) bool {
	if !u.markUpgrading() {
		u.Warn("already upgrading")
		return false
	}
	defer u.unmarkUpgrading()

	err := v.Struct(vi)
	if err != nil {
		u.Error("version info is invalid", err, "new_ver_info", vi)
		return false
	}

	deployCmd := vi.DeployCmd
	if len(deployCmd) == 0 {
		deployCmd = u.Cfg.DeployCmd
	}
	if len(deployCmd) == 0 {
		u.Error("deploy cmd is empty", nil)
		return false
	}

	startCmd := vi.StartCmd
	if len(startCmd) == 0 {
		startCmd = u.Cfg.StartCmd
	}
	if len(startCmd) == 0 {
		u.Error("start cmd is empty", nil)
		return false
	}

	u.Info("start upgrade", "cur_ver", buildinfo.Version, "new_ver_info", vi)

	if paths.DirExist(vi.DownloadDstPath) {
		u.Error("downloaded dst path is dir, use another path", nil)
		return false
	}

	if paths.FileExist(vi.DownloadDstPath) {
		u.Info("download dst path exists, remove it", "path", vi.DownloadDstPath)
		err = os.Remove(vi.DownloadDstPath)
		if err != nil {
			u.Error("remove exists download dst path failed", err)
			return false
		}
	}

	u.Info("start download new package")
	err = downloadPackage(vi.DownloadDstPath, vi.StoreCfg)
	if err != nil {
		u.Error("download new package failed", err)
		return false
	}
	u.Info("download new package done")

	go func() {
		u.Info("stopping all")
		runner.StopAndWait(u.Parent())
	}()

	select {
	case <-time.After(time.Minute):
		u.Error("stop all timeout", u.Parent().Err())
	case <-u.allDoneExceptMe:
		u.Info("all stopped")
	}

	u.Info("start deploy new package")
	var cmdResult *cmd.Result
	cmdResult = cmd.Run(deployCmd...)
	if cmdResult.Err() != nil {
		u.Error("deploy new package failed", err, "cmd_result", cmdResult)
		os.Exit(1)
	}
	u.Info("deploy new package done")

	u.Info("start new version")
	cmdResult = cmd.Run(startCmd...)
	if cmdResult.Err() != nil {
		u.Error("start new version failed", err, "cmd_result", cmdResult)
		os.Exit(1)
	}
	u.Info("start new version done, exit now")
	os.Exit(0)
	return true
}

func downloadPackage(downloadDstPath string, storeCfg *pipeline.ReaderCfg) error {
	cfg := pipeline.NewCfg()
	cfg.Add(pipeline.WorkerCopy, pipeline.NewCopyCfg(), &pipeline.CommonOption{}).
		ReadFromReader(storeCfg.CommonCfgWithOption).
		WriteTo(pipeline.WriterFile, &pipeline.FileCfg{Path: downloadDstPath}, &pipeline.CommonOption{})

	p := pipeline.New()
	p.SetCfg(cfg)
	p.Inherit(D)
	err := runner.Init(p)
	if err != nil {
		return errs.Wrap(err, "init download pipeline failed")
	}

	return runner.Run(p)
}
