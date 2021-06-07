package cmd

import (
	"os/exec"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util"
)

func Create(cfg *Cfg) (*exec.Cmd, error) {
	err := util.V.Struct(cfg)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(cfg.Command[0], cfg.Command[1:]...)
	if len(cfg.Env) > 0 {
		for k, v := range cfg.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	if cfg.WorkingDir != "" {
		if !util.DirExist(cfg.WorkingDir) {
			return nil, errs.Errorf("working dir not exists: %s", cfg.WorkingDir)
		}
		cmd.Dir = cfg.WorkingDir
	}
	if cfg.RunAsUser != "" {
		// not support
	}
	return cmd, nil
}
