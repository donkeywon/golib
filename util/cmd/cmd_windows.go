package cmd

import (
	"os/exec"
	"syscall"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/paths"
	"golang.org/x/sys/windows"
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
		// not support
	}
	if cfg.SetPgid {
		beforeRun = append(beforeRun, func(cmd *exec.Cmd) {
			if cmd.SysProcAttr == nil {
				cmd.SysProcAttr = &syscall.SysProcAttr{}
			}
			cmd.SysProcAttr.CreationFlags |= windows.CREATE_NEW_PROCESS_GROUP
		})
	}

	return beforeRun, nil
}
