package pipeline

import (
	"errors"
	"os/exec"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/cmd"
)

func init() {
	plugin.RegisterWithCfg(RWTypeCmd, func() interface{} { return NewCmdRW() }, func() interface{} { return NewCmdRWCfg() })
}

const RWTypeCmd RWType = "cmd"

func NewCmdRWCfg() *cmd.Cfg {
	return &cmd.Cfg{}
}

type CmdRW struct {
	RW
	*cmd.Cfg
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

	return c.RW.Init()
}

func (c *CmdRW) Start() error {
	result, err := cmd.RunRaw(c.Ctx(), c.Cfg, func(cmd *exec.Cmd) error {
		if c.Writer() != nil {
			cmd.Stdout = c
		}

		if c.Reader() != nil {
			cmd.Stdin = c
		}
		return nil
	})
	c.Info("cmd exit", "result", result)
	if errors.Is(err, ErrStoppedManually) {
		err = nil
	}

	if result != nil {
		c.Store(consts.FieldCmdStderr, result.Stderr)
		c.Store(consts.FieldCmdStdout, result.Stdout)
		c.Store(consts.FieldCmdExitCode, result.ExitCode)
		c.Store(consts.FieldStartTimeNano, result.StartTimeNano)
		c.Store(consts.FieldStopTimeNano, result.StopTimeNano)
		c.Store(consts.FieldCmdSignaled, result.Signaled)
	}

	closeErr := c.Close()
	if err == nil {
		return closeErr
	}

	return errors.Join(errs.Wrapf(err, "cmd exit, result: %v", result), closeErr)
}

func (c *CmdRW) Type() interface{} {
	return RWTypeCmd
}

func (c *CmdRW) GetCfg() interface{} {
	return c.Cfg
}
