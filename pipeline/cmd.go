package pipeline

import (
	"context"
	"errors"
	"os/exec"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/cmds"
	"github.com/rs/zerolog"
)

type cmdWorker struct {
	BaseWorker

	name string
	args []string

	opts []cmds.Option
}

func NewCmdWorker(name string, args ...string) Worker {
	return &cmdWorker{
		name: name,
		args: args,
	}
}

func (c *cmdWorker) WithOptions(opts ...cmds.Option) {
	c.opts = append(c.opts, opts...)
}

func (c *cmdWorker) Run(ctx context.Context) (err error) {
	l := zerolog.Ctx(ctx).With().Str("cmd", c.name).Logger()

	defer func() {
		closeErr := c.Close(false)
		if closeErr != nil {
			err = errors.Join(err, errs.Wrap(closeErr, "close failed"))
		}
	}()

	cmd := exec.CommandContext(ctx, c.name, c.args...)
	cmds.WithOptions(cmd, c.opts...)
	cmds.WithOptions(cmd, func(cmd *exec.Cmd) {
		if c.Reader() != nil {
			cmd.Stdin = c.Reader()
		}
		if c.Writer() != nil {
			cmd.Stdout = c.Writer()
		}
	})
	err = cmd.Run()
	if err == nil {
		l.Info().Msg("cmd done")
	} else {
		isSignaled, isCoreDump, sig := cmds.IsSignaled(err)
		exitCode := cmd.ProcessState.ExitCode()
		if errors.Is(err, context.Canceled) {
			l.Info().Int("exit_code", exitCode).Bool("is_signaled", isSignaled).Bool("is_coredump", isCoreDump).Str("signal", sig.String()).Msg("cmd canceled")
		} else if isSignaled {
			l.Warn().Int("exit_code", exitCode).Bool("is_signaled", isSignaled).Bool("is_coredump", isCoreDump).Str("signal", sig.String()).Msg("cmd signaled")
		} else {
			l.Error().Int("exit_code", exitCode).Bool("is_signaled", isSignaled).Bool("is_coredump", isCoreDump).Str("signal", sig.String()).Msg("cmd failed")
		}

		closeErr := c.Close(true)
		if closeErr != nil {
			err = errors.Join(err, errs.Wrap(closeErr, "close failed"))
		}
	}
	if err != nil {
		return errs.Wrap(err, "exec cmd failed")
	}

	return nil
}

func (c *cmdWorker) SupportZeroCopy() bool {
	return true
}
