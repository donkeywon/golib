package upd

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/buildinfo"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/pipeline"
	"github.com/donkeywon/golib/ratelimit"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util"
	"github.com/donkeywon/golib/util/cmd"
)

const SvcTypeUpd boot.SvcType = "upd"

var _u = &Upd{
	Runner: runner.Create(string(SvcTypeUpd)),
}

type Upd struct {
	runner.Runner
	*Cfg

	updating atomic.Bool
}

func New() *Upd {
	return _u
}

func (u *Upd) Type() interface{} {
	return SvcTypeUpd
}

func (u *Upd) GetCfg() interface{} {
	return u.Cfg
}

func (u *Upd) markUpdating() bool {
	return u.updating.CompareAndSwap(false, true)
}

func (u *Upd) unmarkUpdating() {
	u.updating.Store(false)
}

func (u *Upd) Upgrade(vi *VerInfo) {
	go func() {
		err := u.upgrade(vi)
		if err != nil {
			u.Error("upgrade fail", err)
		}
	}()
}

func (u *Upd) upgrade(vi *VerInfo) error {
	if !u.markUpdating() {
		return errs.New("already updating")
	}
	defer u.unmarkUpdating()

	u.Info("start upgrade", "cur_ver", buildinfo.Version, "new_ver", vi.Ver)

	var err error
	downloadPath := filepath.Join(u.DownloadDir, vi.Filename)
	if util.FileExist(downloadPath) {
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

	tmpExtractDir := filepath.Join(u.DownloadDir, "upgrade-svc-tmp")
	stdout, stderr, err := extractPackage(downloadPath, tmpExtractDir)
	if err != nil {
		return errs.Wrapf(err, "extract package fail, stdout: %v, stderr: %v", stdout, stderr)
	}
	u.Info("extract package done", "extract_dir", tmpExtractDir, "stdout", stdout, "stderr", stderr)

	stopped := make(chan struct{})
	go func() {
		u.Info("close all svc")
		runner.Stop(u.Parent())
		<-u.Done()
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

	res, err := cmd.Run("bash", fmt.Sprintf("%s/bin/upgrade.sh", tmpExtractDir))
	u.Info("exec upgrade.sh", "result", res, "err", err)
	if res.ExitCode != 0 || err != nil {
		os.Exit(1)
	}
	os.Exit(0)
	return nil
}

func downloadPackage(downloadDir string, filename string, ratelimitN int, storeCfg *pipeline.StoreCfg) error {
	cfg := pipeline.NewCfg().
		Add(
			pipeline.RWRoleReader,
			pipeline.RWTypeStore,
			storeCfg,
			&pipeline.RWCommonCfg{
				EnableRateLimit: true,
				RateLimiterCfg: &ratelimit.RateLimiterCfg{
					Type: ratelimit.RateLimiterTypeFixed,
					Cfg: &ratelimit.FixedRateLimiterCfg{
						N: ratelimitN,
					},
				},
			},
		).
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
	p.Inherit(_u)
	err := runner.Init(p)
	if err != nil {
		return errs.Wrap(err, "init download pipeline fail")
	}

	runner.Start(p)
	return p.Err()
}

func extractPackage(filepath string, dstDir string) ([]string, []string, error) {
	res, err := cmd.Run("tar", "xf", filepath, "-C", dstDir)
	if res != nil {
		return res.Stdout, res.Stderr, err
	}
	return nil, nil, err
}

func Upgrade(vi *VerInfo) {
	_u.Upgrade(vi)
}
