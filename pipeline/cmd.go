package pipeline

import (
	"context"
	"errors"
	"os/exec"
	"slices"

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

	forceClosed := false
	defer func() {
		if forceClosed {
			return
		}
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
	stderrBuf := newKeepLastWriter(8 * 1024)
	cmds.WithOptions(cmd, cmds.DumpStderr(stderrBuf))
	err = cmd.Run()
	stderr := stderrBuf.Bytes()
	if err == nil {
		l.Info().Bytes("stderr", stderr).Msg("cmd done")
		return nil
	}

	isSignaled, isCoreDump, sig := cmds.IsSignaled(err)
	exitCode := cmd.ProcessState.ExitCode()
	if errors.Is(err, context.Canceled) {
		l.Info().AnErr("err", err).Int("exit_code", exitCode).Bool("is_signaled", isSignaled).Bool("is_coredump", isCoreDump).Str("signal", sig.String()).Bytes("stderr", stderr).Msg("cmd canceled")
	} else if isSignaled {
		l.Warn().AnErr("err", err).Int("exit_code", exitCode).Bool("is_signaled", isSignaled).Bool("is_coredump", isCoreDump).Str("signal", sig.String()).Bytes("stderr", stderr).Msg("cmd signaled")
	} else {
		l.Error().AnErr("err", err).Int("exit_code", exitCode).Bool("is_signaled", isSignaled).Bool("is_coredump", isCoreDump).Str("signal", sig.String()).Bytes("stderr", stderr).Msg("cmd failed")
	}

	if len(stderr) > 0 {
		if stderrBuf.Truncated() {
			err = errs.Wrapf(err, "stderr(truncated): %s", stderr)
		} else {
			err = errs.Wrapf(err, "stderr: %s", stderr)
		}
	}
	forceClosed = true
	closeErr := c.Close(true)
	if closeErr != nil {
		err = errors.Join(err, errs.Wrap(closeErr, "close failed"))
	}

	return errs.Wrap(err, "exec cmd failed")
}

func (c *cmdWorker) SupportZeroCopy() bool {
	return true
}

type keepLastWriter struct {
	offset    int64
	b         []byte
	written   int64
	overwrite bool
}

func newKeepLastWriter(n int64) *keepLastWriter {
	if n == 0 {
		panic("keepLastWriter: zero length")
	}
	return &keepLastWriter{b: make([]byte, n)}
}

func (l *keepLastWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}

	for len(p) > 0 {
		nc := copy(l.b[l.offset:], p)
		n += nc
		l.offset += int64(nc)
		l.written += int64(nc)
		if l.offset == int64(len(l.b)) {
			l.overwrite = true
			l.offset = 0
		}

		p = p[nc:]
	}
	return
}

func (l *keepLastWriter) Bytes() []byte {
	if !l.overwrite {
		return l.b[:l.offset]
	}

	if l.offset == 0 {
		return l.b
	}

	return slices.Concat(l.b[l.offset:], l.b[:l.offset])
}

func (l *keepLastWriter) Truncated() bool { return l.written > int64(len(l.b)) }
