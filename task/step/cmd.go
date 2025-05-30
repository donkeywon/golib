package step

import (
	"context"
	"os/exec"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/proc"
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

	cmd         *exec.Cmd
	beforeStart []func(cmd *exec.Cmd)
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

	c.cmd = exec.CommandContext(c.Ctx(), c.Command[0], c.Command[1:]...)
	c.WithLoggerFields("cmd", c.Command[0])

	return c.Step.Init()
}

func (c *CmdStep) Start() error {
	var err error

	c.Cfg.SetPgid = true

	result := cmd.Start(c.Ctx(), c.cmd, c.Cfg, c.beforeStart...)
	<-result.Done()
	err = result.Err()
	c.Info("cmd exit", "result", result.String())

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
			c.Info("cmd exit signaled", "err", err)
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
	return cmd.MustStopGroup(context.Background(), c.cmd, 5, proc.MustKillSignals...)
}

func (c *CmdStep) SetCfg(cfg any) {
	c.Cfg = cfg.(*cmd.Cfg)
}

func (c *CmdStep) BeforeStart(f ...func(cmd *exec.Cmd)) {
	c.beforeStart = append(c.beforeStart, f...)
}
