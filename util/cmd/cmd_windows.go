package cmd

import (
	"os/exec"

	"github.com/donkeywon/golib/util/paths"
)

func beforeStartFromCfg(cfg *Cfg) ([]func(cmd *exec.Cmd), error) {
	if cfg == nil {
		return nil
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
	return beforeRun, nil
}
