package rw

import (
	"context"
	"os/exec"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/cmd"
)

func init() {
	plugin.RegWithCfg(TypeCmd, NewCmd, func() any { return NewCmdCfg() })
}

const TypeCmd Type = "cmd"

func NewCmdCfg() *cmd.Cfg {
	return &cmd.Cfg{}
}

type Cmd struct {
	RW
	*cmd.Cfg

	cmd *exec.Cmd
}

func NewCmd() *Cmd {
	return &Cmd{
		RW: CreateBase(string(TypeCmd)),
	}
}

func (c *Cmd) Init() error {
	if !c.IsStarter() {
		return errs.New("cmd rw must be Starter")
	}

	c.cmd = exec.CommandContext(c.Ctx(), c.Cfg.Command[0], c.Cfg.Command[1:]...)

	return c.RW.Init()
}

func (c *Cmd) Start() error {
	result, err := cmd.RunCmd(context.Background(), c.cmd, c.Cfg, func(cmd *exec.Cmd) error {
		if c.Writer() != nil {
			cmd.Stdout = c.Writer()
		}

		if c.Reader() != nil {
			cmd.Stdin = c.Reader()
		}
		return nil
	})

	if result != nil {
		c.Store(consts.FieldCmdStderr, result.Stderr)
		c.Store(consts.FieldCmdStdout, result.Stdout)
		c.Store(consts.FieldCmdExitCode, result.ExitCode)
		c.Store(consts.FieldStartTimeNano, result.StartTimeNano)
		c.Store(consts.FieldStopTimeNano, result.StopTimeNano)
		c.Store(consts.FieldCmdSignaled, result.Signaled)
	}

	if result != nil && result.Signaled {
		select {
		case <-c.Stopping():
			c.Info("exit signaled", "err", err)
			return nil
		default:
		}
	}

	if err != nil {
		return errs.Wrapf(err, "cmd exit, result: %v", result)
	}

	return nil
}

func (c *Cmd) Stop() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	return c.cmd.Process.Kill()
}

func (c *Cmd) Type() Type {
	return TypeCmd
}
