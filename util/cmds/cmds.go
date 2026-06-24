package cmds

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/donkeywon/golib/util/proc"
)

const (
	defaultWaitInterval = 10 * time.Millisecond
)

type Option func(cmd *exec.Cmd)

func Start(ctx context.Context, name string, args []string, opts ...Option) (*exec.Cmd, error) {
	return execCmd(ctx, name, args, func(c *exec.Cmd) error { return c.Start() }, opts...)
}

func Run(ctx context.Context, name string, args []string, opts ...Option) (*exec.Cmd, error) {
	return execCmd(ctx, name, args, func(c *exec.Cmd) error { return c.Run() }, opts...)
}

func execCmd(ctx context.Context, name string, args []string, f func(*exec.Cmd) error, opts ...Option) (*exec.Cmd, error) {
	if ctx == nil {
		panic("nil context")
	}

	c := exec.CommandContext(ctx, name, args...)
	for _, opt := range opts {
		opt(c)
	}

	return c, f(c)
}

func WorkingDir(dir string) Option {
	return func(cmd *exec.Cmd) {
		cmd.Dir = dir
	}
}

func EnvMap(m map[string]string) Option {
	return func(cmd *exec.Cmd) {
		if len(m) == 0 {
			return
		}
		if cmd.Env == nil {
			cmd.Env = make([]string, 0, len(m))
		}
		buf := bytes.NewBuffer(nil)
		for k, v := range m {
			buf.WriteString(k)
			buf.WriteByte('=')
			buf.WriteString(v)
			cmd.Env = append(cmd.Env, buf.String())
			buf.Reset()
		}
	}
}

func EnvKVs(kvs ...string) Option {
	return func(cmd *exec.Cmd) {
		if len(kvs) == 0 {
			return
		}
		if len(kvs)%2 != 0 {
			panic("even env kv count")
		}
		if cmd.Env == nil {
			cmd.Env = make([]string, 0, len(kvs)/2)
		}

		buf := bytes.NewBuffer(nil)
		for i := 0; i < len(kvs); i += 2 {
			buf.WriteString(kvs[i])
			buf.WriteByte('=')
			buf.WriteString(kvs[i+1])
			cmd.Env = append(cmd.Env, buf.String())
			buf.Reset()
		}
	}
}

func WaitDelay(d time.Duration) Option {
	return func(cmd *exec.Cmd) {
		cmd.WaitDelay = d
	}
}

func GracefulStop(waitDurationBetweenSignals time.Duration, signals ...os.Signal) Option {
	if waitDurationBetweenSignals < defaultWaitInterval {
		waitDurationBetweenSignals = defaultWaitInterval
	}
	if len(signals) == 0 {
		signals = []os.Signal{syscall.SIGKILL}
	}

	return func(cmd *exec.Cmd) {
		cmd.Cancel = func() error {
			for _, sig := range signals {
				err := cmd.Process.Signal(sig)
				if err != nil {
					return err
				}

				if proc.WaitProcExit(context.Background(), cmd.Process.Pid, defaultWaitInterval, int(waitDurationBetweenSignals/defaultWaitInterval)) {
					return nil
				}
			}

			return cmd.Process.Kill()
		}
	}
}

// IsSignaled check if cmd exit signaled, err is Cmd.Wait() or Cmd.Run() error.
func IsSignaled(err error) (isSignaled bool, isCoreDump bool, signal syscall.Signal) {
	if err == nil {
		return false, false, -1
	}

	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		return false, false, -1
	}

	waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return false, false, -1
	}

	return waitStatus.Signaled(), waitStatus.CoreDump(), waitStatus.Signal()
}
