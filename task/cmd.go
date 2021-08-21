package task

import (
	"github.com/donkeywon/golib/consts"
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
}

func NewCmdStep() *Cmd {
	return &Cmd{
		Step: newBase(string(StepTypeCmd)),
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

	result, err := cmd.RunRaw(c.Ctx(), c.Cfg)
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

func (c *Cmd) Stop() error {
	c.Cancel()
	return nil
}

func (c *Cmd) Type() interface{} {
	return StepTypeCmd
}

func (c *Cmd) GetCfg() interface{} {
	return c.Cfg
}
