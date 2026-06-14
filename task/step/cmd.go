package step

import (
	"context"
	"os/exec"

	"github.com/donkeywon/golib/cmd"
	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/logs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/v"
)

func init() {
	plugin.Reg(TypeCmd, func() Step { return NewCmdStep() }, func() any { return NewCmdStepCfg() })
}

const TypeCmd Type = "cmd"

func NewCmdStepCfg() *cmd.Cfg {
	return &cmd.Cfg{}
}

type CmdStep struct {
	Base

	cfg         cmd.Cfg
	cancel      context.CancelFunc
	beforeStart []func(cmd *exec.Cmd)
}

func NewCmdStep() *CmdStep {
	return &CmdStep{}
}

func (c *CmdStep) Init(ctx context.Context) error {
	return v.StructCtx(ctx, c.cfg)
}

func (c *CmdStep) Start(ctx context.Context) error {
	l := logs.FromCtx(ctx)

	ctx, c.cancel = context.WithCancel(ctx)
	defer c.cancel()

	var err error

	c.cfg.SetPgid = true

	result := cmd.Run(ctx, c.cfg, c.beforeStart...)
	err = result.Err()
	if err == nil {
		l.Info("cmd done", "result", result)
	} else {
		l.Error("cmd failed", "result", result)
	}

	c.Store(consts.FieldCmdStdout, result.Stdout)
	c.Store(consts.FieldCmdStderr, result.Stderr)
	c.Store(consts.FieldCmdExitCode, result.ExitCode)
	c.Store(consts.FieldStartTimeNano, result.StartTimeNano)
	c.Store(consts.FieldStopTimeNano, result.StopTimeNano)
	c.Store(consts.FieldCmdSignaled, result.Signaled)

	if result != nil && result.Signaled {
		select {
		case <-c.Stopping():
			l.Info("cmd exit signaled", "err", err)
			err = nil
		default:
		}
	}

	if err != nil {
		return errs.Wrap(err, "exec cmd failed")
	}

	return nil
}

func (c *CmdStep) Stop(ctx context.Context) error {
	c.cancel()
	return nil
}

func (c *CmdStep) SetCfg(cfg any) {
	c.cfg = cfg.(cmd.Cfg)
}

func (c *CmdStep) BeforeStart(f ...func(cmd *exec.Cmd)) {
	c.beforeStart = append(c.beforeStart, f...)
}
