package task

import (
	"context"
	"os/exec"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/vtil"
)

func init() {
	plugin.RegisterWithCfg(StepTypeCmd, func() interface{} { return NewCmdStep() }, func() interface{} { return NewCmdStepCfg() })
}

const StepTypeCmd StepType = "cmd"

func NewCmdStepCfg() *cmd.Cfg {
	return &cmd.Cfg{}
}

type CmdStep struct {
	Step
	*cmd.Cfg

	cmd *exec.Cmd
}

func NewCmdStep() *CmdStep {
	return &CmdStep{
		Step: newBase(string(StepTypeCmd)),
		Cfg:  NewCmdStepCfg(),
	}
}

func (c *CmdStep) Init() error {
	err := vtil.Struct(c.Cfg)
	if err != nil {
		return err
	}

	c.WithLoggerFields("cmd", c.Command[0])

	c.cmd = exec.CommandContext(c.Ctx(), c.Command[0], c.Command[1:]...)

	return c.Step.Init()
}

func (c *CmdStep) Start() error {
	var err error

	result, err := cmd.RunCmd(c.Ctx(), c.cmd, c.Cfg)
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
			c.Info("cmd exit signaled", "err", err)
			err = nil
		default:
		}
	}

	if err != nil {
		return errs.Wrap(err, "exec cmd fail")
	}

	return nil
}

func (c *CmdStep) Stop() error {
	return cmd.MustStop(context.Background(), c.cmd)
}

func (c *CmdStep) Type() interface{} {
	return StepTypeCmd
}

func (c *CmdStep) GetCfg() interface{} {
	return c.Cfg
}
