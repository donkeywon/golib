package pipeline

import (
	"os/exec"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/cmd"
)

func init() {
	plugin.RegWithCfg(WorkerCmd, func() Worker { return NewCmd() }, func() any { return cmd.NewCfg() })
}

const WorkerCmd Type = "cmd"

type Cmd struct {
	Worker
	*cmd.Cfg
}

func NewCmd() *Cmd {
	return &Cmd{
		Worker: CreateWorker(string(WorkerCmd)),
		Cfg:    &cmd.Cfg{},
	}
}

func (c *Cmd) Start() error {
	defer c.Close()

	c.Cfg.SetPgid = true

	c.WithLoggerFields("cmd", c.Cfg.Command[0])

	c.Debug("starting pipeline cmd", "commands", c.Cfg.Command)

	result := cmd.Run(c.Ctx(), c.Cfg, func(cmd *exec.Cmd) {
		if c.Writer() != nil {
			switch w := c.Writer().(type) {
			case Writer:
				cmd.Stdout = w.DirectWriter()
			default:
				cmd.Stdout = c.Writer()
			}
		}

		if c.Reader() != nil {
			switch r := c.Reader().(type) {
			case Reader:
				cmd.Stdin = r.DirectReader()
			default:
				cmd.Stdin = c.Reader()
			}
		}
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
			c.Info("exit signaled", "err", result.Err())
			return nil
		default:
		}
	}

	if result.Err() != nil {
		return errs.Wrap(result.Err(), "pipeline cmd failed")
	}

	return nil
}

func (c *Cmd) Stop() error {
	c.Cancel()
	return nil
}

func (c *Cmd) SetCfg(cfg any) {
	c.Cfg = cfg.(*cmd.Cfg)
}
