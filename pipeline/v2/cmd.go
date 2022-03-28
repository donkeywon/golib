package v2

import (
	"context"
	"os/exec"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/cmd"
)

func init() {
	plugin.RegWithCfg(WorkerCmd, NewCmd, cmd.NewCfg)
}

const WorkerCmd Type = "cmd"

type Cmd struct {
	Worker
	*cmd.Cfg

	c *exec.Cmd
}

func NewCmd() *Cmd {
	return &Cmd{
		Worker: CreateWorker(string(WorkerCmd)),
		Cfg:    &cmd.Cfg{},
	}
}

func (c *Cmd) Init() error {
	c.c = exec.CommandContext(c.Ctx(), c.Cfg.Command[0], c.Cfg.Command[1:]...)

	return c.Worker.Init()
}

func (c *Cmd) Start() error {
	defer c.Close()

	result, err := cmd.RunCmd(context.Background(), c.c, c.Cfg, func(cmd *exec.Cmd) error {
		if c.Writer() != nil {
			cmd.Stdout = c.Writer()
		}

		if c.Reader() != nil {
			cmd.Stdin = c.Reader()
		}
		return nil
	})

	c.Info("cmd exit", "result", result)
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
		return errs.Wrapf(err, "cmd failed")
	}

	return nil
}

func (c *Cmd) Stop() error {
	if c.c == nil || c.c.Process == nil {
		return nil
	}
	return c.c.Process.Kill()
}

func (c *Cmd) Type() Type {
	return WorkerCmd
}

func (c *Cmd) GetCfg() *cmd.Cfg {
	return c.Cfg
}
