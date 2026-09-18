package step

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"

	"codeberg.org/donkeywon/golib/consts"
	"codeberg.org/donkeywon/golib/errs"
	"codeberg.org/donkeywon/golib/kvs"
	"codeberg.org/donkeywon/golib/plugin"
	"codeberg.org/donkeywon/golib/util/cmds"
	"codeberg.org/donkeywon/golib/util/v"
	"github.com/rs/zerolog"
)

func init() {
	plugin.Reg(TypeCmd, func() Step { return NewCmdStep() }, NewCmdStepCfg)
}

const TypeCmd Type = "cmd"

type CmdStepCfg struct {
	Name               string            `json:"name"                 yaml:"name"               validate:"required"`
	Args               []string          `json:"args"                 yaml:"args"`
	Env                map[string]string `json:"env"                  yaml:"env"`
	RunAsUser          string            `json:"run_as_user"          yaml:"runAsUser"`
	WorkingDir         string            `json:"working_dir"          yaml:"workingDir"`
	SetPgid            bool              `json:"set_pgid"             yaml:"setPgid"`
	DumpStdout         bool              `json:"dump_stdout"          yaml:"dumpStdout"`
	DumpStderr         bool              `json:"dump_stderr"          yaml:"dumpStderr"`
	GracefulStopSignal int               `json:"graceful_stop_signal" yaml:"gracefulStopSignal"`
	WaitDelay          time.Duration     `json:"wait_delay"           yaml:"waitDelay"`
}

func NewCmdStepCfg() CmdStepCfg {
	return CmdStepCfg{}
}

type CmdStep struct {
	kvs.Map[string, any]

	cfg     CmdStepCfg
	options []cmds.Option
}

func NewCmdStep() *CmdStep {
	return &CmdStep{}
}

func (c *CmdStep) Init(ctx context.Context) error {
	return v.StructCtx(ctx, c.cfg)
}

func (c *CmdStep) Run(ctx context.Context) error {
	l := zerolog.Ctx(ctx).With().Str("cmd", c.cfg.Name).Logger()

	var (
		err  error
		opts []cmds.Option
	)

	if len(c.cfg.Env) > 0 {
		opts = append(opts, cmds.EnvMap(c.cfg.Env))
	}
	if len(c.cfg.RunAsUser) > 0 {
		opts = append(opts, cmds.RunAsUser(c.cfg.RunAsUser))
	}
	if len(c.cfg.WorkingDir) > 0 {
		opts = append(opts, cmds.WorkingDir(c.cfg.WorkingDir))
	}
	if c.cfg.SetPgid {
		opts = append(opts, cmds.SetPgid())
	}
	if c.cfg.WaitDelay > 0 {
		opts = append(opts, cmds.WaitDelay(c.cfg.WaitDelay))
	}
	if c.cfg.GracefulStopSignal > 0 {
		opts = append(opts, cmds.GracefulStop(c.cfg.WaitDelay, syscall.Signal(c.cfg.GracefulStopSignal)))
	}

	var (
		stdoutBuf *bytes.Buffer
		stderrBuf *bytes.Buffer
	)
	if c.cfg.DumpStdout {
		stdoutBuf = bytes.NewBuffer(nil)
		opts = append(opts, cmds.DumpStdout(stdoutBuf))
	}
	if c.cfg.DumpStderr {
		stderrBuf = bytes.NewBuffer(nil)
		opts = append(opts, cmds.DumpStderr(stderrBuf))
	}

	opts = append(opts, c.options...)
	cmd := exec.CommandContext(ctx, c.cfg.Name, c.cfg.Args...)
	cmds.WithOptions(cmd, opts...)
	err = cmd.Run()
	if err == nil {
		l.Info().Msg("cmd done")
	} else {
		isSignaled, isCoreDump, sig := cmds.IsSignaled(err)
		exitCode := cmd.ProcessState.ExitCode()
		c.Store(consts.FieldCmdExitCode, exitCode)
		c.Store(consts.FieldCmdIsSignaled, isSignaled)
		c.Store(consts.FieldCmdSignal, int(sig))
		c.Store(consts.FieldCmdIsCoredump, isCoreDump)

		if errors.Is(err, context.Canceled) {
			l.Warn().Int("exit_code", exitCode).Bool("is_signaled", isSignaled).Bool("is_coredump", isCoreDump).Str("signal", sig.String()).Msg("cmd canceled")
		} else if isSignaled {
			l.Warn().Int("exit_code", exitCode).Bool("is_signaled", isSignaled).Bool("is_coredump", isCoreDump).Str("signal", sig.String()).Msg("cmd signaled")
		} else {
			l.Error().Int("exit_code", exitCode).Bool("is_signaled", isSignaled).Bool("is_coredump", isCoreDump).Str("signal", sig.String()).Msg("cmd failed")
		}
	}

	if c.cfg.DumpStdout {
		c.Store(consts.FieldCmdStdout, stdoutBuf.Bytes())
	}
	if c.cfg.DumpStderr {
		c.Store(consts.FieldCmdStderr, stderrBuf.Bytes())
	}

	if err != nil {
		return errs.Wrap(err, "exec cmd failed")
	}

	return nil
}

func (c *CmdStep) SetCfg(cfg any) {
	c.cfg = cfg.(CmdStepCfg)
}

func (c *CmdStep) CmdOptions(opts ...cmds.Option) {
	c.options = append(c.options, opts...)
}
