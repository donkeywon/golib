package task

import (
	"context"
	"os/exec"

	"github.com/donkeywon/golib/common"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util"
	"github.com/donkeywon/golib/util/cmd"
)

func init() {
	plugin.Register(StepTypeCmd, func() interface{} { return NewCmdStep() })
	plugin.RegisterCfg(StepTypeCmd, func() interface{} { return NewCmdCfg() })
}

const StepTypeCmd StepType = "cmd"

func NewCmdCfg() *cmd.Cfg {
	return &cmd.Cfg{}
}

type Cmd struct {
	Step
	*cmd.Cfg

	cmd *exec.Cmd
}

func NewCmdStep() *Cmd {
	return &Cmd{
		Step: NewBase("cmd"),
		Cfg:  NewCmdCfg(),
	}
}

func (c *Cmd) Init() error {
	err := util.V.Struct(c.Cfg)
	if err != nil {
		return err
	}

	c.WithLoggerFields("cmd", c.Command[0])

	return c.Step.Init()
}

func (c *Cmd) Start() error {
	var err error

	c.cmd, err = cmd.Create(c.Cfg)
	if err != nil {
		return errs.Wrap(err, "create cmd fail")
	}

	result, err := cmd.RunCmd(c.Ctx(), nil, c.cmd)
	c.Info("cmd exit", "result", result)

	if result != nil {
		c.Store(common.FieldCmdStdout, result.Stdout)
		c.Store(common.FieldCmdStderr, result.Stderr)
		c.Store(common.FieldCmdExitCode, result.ExitCode)
		c.Store(common.FieldStartTimeNano, result.StartTimeNano)
		c.Store(common.FieldStopTimeNano, result.StopTimeNano)
		c.Store(common.FieldCmdSignaled, result.Signaled)
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

func (c *Cmd) Stop() error {
	return cmd.MustStop(context.Background(), c.cmd)
}

func (c *Cmd) Type() interface{} {
	return StepTypeCmd
}

func (c *Cmd) GetCfg() interface{} {
	return c.Cfg
}
