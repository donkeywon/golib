package pipeline

import (
	"context"
	"os/exec"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/cmd"
)

func init() {
	plugin.RegWithCfg(RWTypeCmd, func() any { return NewCmdRW() }, func() any { return NewCmdRWCfg() })
}

const RWTypeCmd RWType = "cmd"

func NewCmdRWCfg() *cmd.Cfg {
	return &cmd.Cfg{}
}

type CmdRW struct {
	RW
	*cmd.Cfg

	cmd *exec.Cmd
}

func NewCmdRW() *CmdRW {
	return &CmdRW{
		RW: CreateBaseRW(string(RWTypeCmd)),
	}
}

func (c *CmdRW) Init() error {
	if !c.IsStarter() {
		return errs.New("cmd rw must be Starter")
	}

	c.cmd = exec.CommandContext(c.Ctx(), c.Cfg.Command[0], c.Cfg.Command[1:]...)

	return c.RW.Init()
}

func (c *CmdRW) Start() error {
	result, err := cmd.RunCmd(context.Background(), c.cmd, c.Cfg, func(cmd *exec.Cmd) error {
		if c.Writer() != nil {
			cmd.Stdout = c
		}

		if c.Reader() != nil {
			cmd.Stdin = c
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
			c.Info("exit signaled", "error", err)
			return nil
		default:
		}
	}

	if err == nil {
		return nil
	}

	return errs.Wrapf(err, "cmd exit, result: %v", result)
}

func (c *CmdRW) Stop() error {
	return cmd.MustStop(context.Background(), c.cmd)
}

func (c *CmdRW) Type() any {
	return RWTypeCmd
}

func (c *CmdRW) GetCfg() any {
	return c.Cfg
}
