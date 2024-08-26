//go:build linux || darwin || freebsd || solaris

package cmd

import (
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/paths"
)

func beforeRunFromCfg(cfg *Cfg) []func(cmd *exec.Cmd) error {
	if cfg == nil {
		return nil
	}
	var beforeRun []func(cmd *exec.Cmd) error
	if len(cfg.Env) > 0 {
		beforeRun = append(beforeRun, func(cmd *exec.Cmd) error {
			for k, v := range cfg.Env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
			return nil
		})
	}
	if cfg.WorkingDir != "" {
		beforeRun = append(beforeRun, func(cmd *exec.Cmd) error {
			if !paths.DirExist(cfg.WorkingDir) {
				return errs.Errorf("working dir not exists: %s", cfg.WorkingDir)
			}
			cmd.Dir = cfg.WorkingDir
			return nil
		})
	}
	if cfg.RunAsUser != "" {
		beforeRun = append(beforeRun, func(cmd *exec.Cmd) error {
			u, er := user.Lookup(cfg.RunAsUser)
			if er != nil {
				return errs.Wrapf(er, "system user not found: %s", cfg.RunAsUser)
			}
			uid, _ := strconv.Atoi(u.Uid)
			gid, _ := strconv.Atoi(u.Gid)

			cmd.SysProcAttr = &syscall.SysProcAttr{}
			cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
			return nil
		})
	}
	return beforeRun
}
