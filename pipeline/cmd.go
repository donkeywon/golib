package pipeline

import (
	"context"
	"errors"
	"os/exec"

	"github.com/donkeywon/golib/common"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/cmd"
)

func init() {
	plugin.Register(RWTypeCmd, func() interface{} { return NewCmdRW() })
	plugin.RegisterCfg(RWTypeCmd, func() interface{} { return NewCmdRWCfg() })
}

const RWTypeCmd RWType = "cmd"

func NewCmdRWCfg() *cmd.Cfg {
	return &cmd.Cfg{}
}

type CmdRW struct {
	RW
	*cmd.Cfg

	cmdCtx context.Context
	cancel context.CancelFunc
}

func NewCmdRW() *CmdRW {
	return &CmdRW{
		RW: NewBaseRW(string(RWTypeCmd)),
	}
}

func (c *CmdRW) Init() error {
	if !c.IsStarter() {
		return errs.New("cmdRW must be Starter")
	}

	return c.RW.Init()
}

func (c *CmdRW) Start() error {
	c.cmdCtx, c.cancel = context.WithCancel(c.Ctx())

	result, err := cmd.RunRaw(c.cmdCtx, func(cmd *exec.Cmd) {
		if c.Writer() != nil {
			cmd.Stdout = c.Writer()
		}

		if c.Reader() != nil {
			cmd.Stdin = c.Reader()
		}
	}, c.Cfg)
	c.Info("cmd exit", "result", result)

	if result != nil {
		c.Store(common.FieldCmdStderr, result.Stderr)
		c.Store(common.FieldCmdStdout, result.Stdout)
		c.Store(common.FieldCmdExitCode, result.ExitCode)
		c.Store(common.FieldStartTimeNano, result.StartTimeNano)
		c.Store(common.FieldStopTimeNano, result.StopTimeNano)
		c.Store(common.FieldCmdSignaled, result.Signaled)
	}

	closeErr := c.Close()
	if result != nil && result.Signaled {
		select {
		case <-c.Stopping():
			c.Info("exit signaled", "error", err)
			return closeErr
		default:
		}
	}

	return errors.Join(errs.Wrapf(err, "cmd exit, result: %v", result), closeErr)
}

func (c *CmdRW) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func (c *CmdRW) Type() interface{} {
	return RWTypeCmd
}

func (c *CmdRW) GetCfg() interface{} {
	return c.Cfg
}
