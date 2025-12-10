package upd

import (
	"context"
	"errors"
	"os"
	"os/exec"
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

var (
	ErrAlreadyUpgrading = errors.New("already upgrading")
)

type Upd interface {
	boot.Daemon
	Upgrade(vi *VerInfo) error
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

func (u *upd) Upgrade(vi *VerInfo) error {
	if !u.markUpgrading() {
		return ErrAlreadyUpgrading
	}

	err := u.prepareUpgrade(vi)
	if err != nil {
		u.unmarkUpgrading()
		return errs.Wrap(err, "prepare upgrade failed")
	}

	go func() {
		defer func() {
			u.unmarkUpgrading()

			err := recover()
			if err != nil {
				u.Error("panic on upgrade", errs.PanicToErr(err))
			}
		}()

		downloadSuccessfully := u.upgrade(vi)

		if !downloadSuccessfully {
			// download failed, just return for next upgrade
			return
		}

		select {
		case <-u.Stopping():
			u.Info("upgrade stopped due to stopping")
			close(u.upgradingBlockChan)
		default:
		}
	}()
	return nil
}

func (u *upd) prepareUpgrade(vi *VerInfo) error {
	err := v.Struct(vi)
	if err != nil {
		return errs.Wrap(err, "version info is invalid")
	}

	upgradeCmd := vi.UpgradeCmd
	if len(upgradeCmd) == 0 {
		upgradeCmd = u.Cfg.UpgradeCmd
	}
	if len(upgradeCmd) == 0 {
		return errs.New("upgrade cmd is empty")
	}

	upgradeOutputPath := vi.UpgradeOutputPath
	if upgradeOutputPath == "" {
		upgradeOutputPath = u.Cfg.UpgradeOutputPath
	}
	if upgradeOutputPath == "" {
		return errs.New("upgrade output path is empty")
	}
	upgradeOutputFile, err := os.OpenFile(upgradeOutputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return errs.Wrapf(err, "create upgrade output file failed: %s", upgradeOutputPath)
	}
	err = upgradeOutputFile.Close()
	if err != nil {
		return errs.Wrapf(err, "close upgrade output file failed: %s", upgradeOutputPath)
	}

	if paths.DirExist(vi.DownloadDstPath) {
		return errs.Errorf("downloaded dst path is dir, use another path: %s", vi.DownloadDstPath)
	}

	if paths.FileExist(vi.DownloadDstPath) {
		u.Info("download dst path exists, remove it", "path", vi.DownloadDstPath)
		err = os.Remove(vi.DownloadDstPath)
		if err != nil {
			return errs.Wrapf(err, "remove exists download dst path failed")
		}
	}

	return nil
}

func (u *upd) upgrade(vi *VerInfo) bool {
	u.Info("start download new package", "ver", vi.Ver)
	err := downloadPackage(vi.DownloadDstPath, vi.StoreCfg)
	if err != nil {
		u.Error("download new package failed", err)
		return false
	}
	u.Info("download new package done, start upgrade", "cur_ver", buildinfo.Version, "new_ver", vi.Ver)

	// 没有退路可言，no going back
	go func() {
		u.Info("stopping all")
		runner.StopAndWait(u.Parent())
	}()

	select {
	case <-time.After(time.Minute):
		u.Error("stop all daemon timeout", u.Parent().Err())
	case <-u.allDoneExceptMe:
		u.Info("all daemon stopped except me")
	}

	upgradeCmd := vi.UpgradeCmd
	if len(upgradeCmd) == 0 {
		upgradeCmd = u.Cfg.UpgradeCmd
	}
	upgradeOutputPath := vi.UpgradeOutputPath
	if upgradeOutputPath == "" {
		upgradeOutputPath = u.Cfg.UpgradeOutputPath
	}
	upgradeOutputFile, err := os.OpenFile(upgradeOutputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		u.Error("open upgrade output file failed", err, "path", upgradeOutputPath)
	}

	u.Info("start exec upgrade cmd")
	cmdCfg := cmd.NewCfg()
	cmdCfg.Command = upgradeCmd
	cmdCfg.SetPgid = true
	cmdResult := cmd.Start(context.Background(), cmdCfg, func(cmd *exec.Cmd) {
		cmd.Stdout = upgradeOutputFile
		cmd.Stderr = upgradeOutputFile
	})
	if cmdResult.Pid <= 0 {
		u.Error("exec upgrade cmd failed", cmdResult.Err(), "result", cmdResult.String())
		os.Exit(1)
	}

	u.Info("upgrade cmd started, exit now")
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
