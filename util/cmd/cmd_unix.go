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

func beforeStartFromCfg(cfg *Cfg) ([]func(cmd *exec.Cmd), error) {
	if cfg == nil {
		return nil, nil
	}
	var beforeRun []func(cmd *exec.Cmd)
	if len(cfg.Env) > 0 {
		beforeRun = append(beforeRun, func(cmd *exec.Cmd) {
			for k, v := range cfg.Env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
		})
	}
	if cfg.WorkingDir != "" {
		if !paths.DirExist(cfg.WorkingDir) {
			return nil, errs.Errorf("working dir not exists: %s", cfg.WorkingDir)
		}
		beforeRun = append(beforeRun, func(cmd *exec.Cmd) {
			cmd.Dir = cfg.WorkingDir
		})
	}
	if cfg.RunAsUser != "" {
		u, er := user.Lookup(cfg.RunAsUser)
		if er != nil {
			return nil, errs.Wrapf(er, "system user not found: %s", cfg.RunAsUser)
		}

		beforeRun = append(beforeRun, func(cmd *exec.Cmd) {
			if cmd.SysProcAttr == nil {
				cmd.SysProcAttr = &syscall.SysProcAttr{}
			}

			uid, _ := strconv.Atoi(u.Uid)
			gid, _ := strconv.Atoi(u.Gid)

			cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		})
	}
	if cfg.SetPgid {
		beforeRun = append(beforeRun, func(cmd *exec.Cmd) {
			if cmd.SysProcAttr == nil {
				cmd.SysProcAttr = &syscall.SysProcAttr{}
			}
			cmd.SysProcAttr.Setpgid = true
		})
	}
	return beforeRun, nil
}
