package step

import (
	"context"
	"os/exec"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/v"
)

func init() {
	plugin.RegWithCfg(TypeCmd, func() Step { return NewCmdStep() }, func() any { return NewCmdStepCfg() })
}

const TypeCmd Type = "cmd"

func NewCmdStepCfg() *cmd.Cfg {
	return &cmd.Cfg{}
}

type CmdStep struct {
	Step
	*cmd.Cfg

	cmd       *exec.Cmd
	beforeRun []func(cmd *exec.Cmd) error
}

func NewCmdStep() *CmdStep {
	return &CmdStep{
		Step: CreateBase(string(TypeCmd)),
		Cfg:  NewCmdStepCfg(),
	}
}

func (c *CmdStep) Init() error {
	err := v.Struct(c.Cfg)
	if err != nil {
		return err
	}

	c.WithLoggerFields("cmd", c.Command[0])

	return c.Step.Init()
}

func (c *CmdStep) Start() error {
	var err error

	result := cmd.RunCmd(c.Ctx(), c.cmd, c.Cfg, c.beforeRun...)
	err = result.Err()
	c.Info("cmd exit", "result", result)

	if result != nil {
		c.Store(consts.FieldCmdStdout, result.Stdout)
		c.Store(consts.FieldCmdStderr, result.Stderr)
		c.Store(consts.FieldCmdExitCode, result.ExitCode)
		c.Store(consts.FieldStartTimeNano, result.StartTimeNano)
		c.Store(consts.FieldStopTimeNano, result.StopTimeNano)
		c.Store(consts.FieldCmdSignaled, result.Signaled)
	}

	if result != nil && result.Signaled {
		select {
		case <-c.Stopping():
			c.Info("cmd exit signaled", "err", result.Err)
			err = nil
		default:
		}
	}

	if err != nil {
		return errs.Wrap(err, "exec cmd failed")
	}

	return nil
}

func (c *CmdStep) Stop() error {
	return cmd.MustStop(context.Background(), c.cmd)
}

func (c *CmdStep) SetCfg(cfg any) {
	c.Cfg = cfg.(*cmd.Cfg)
	c.cmd = exec.CommandContext(c.Ctx(), c.Command[0], c.Command[1:]...)
}

func (c *CmdStep) BeforeRun(f ...func(cmd *exec.Cmd) error) {
	c.beforeRun = append(c.beforeRun, f...)
}
