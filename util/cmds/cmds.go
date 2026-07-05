package cmds

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/donkeywon/golib/util/proc"
)

const (
	defaultWaitInterval = 10 * time.Millisecond
)

type Option func(cmd *exec.Cmd)

func WithOptions(c *exec.Cmd, opts ...Option) {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(c)
	}
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
		var sb strings.Builder
		for k, v := range m {
			sb.Grow(len(k) + 1 + len(v))
			sb.WriteString(k)
			sb.WriteByte('=')
			sb.WriteString(v)
			cmd.Env = append(cmd.Env, sb.String())
			sb.Reset()
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

		var sb strings.Builder
		for i := 0; i < len(kvs); i += 2 {
			sb.Grow(len(kvs[i]) + 1 + len(kvs[i+1]))
			sb.WriteString(kvs[i])
			sb.WriteByte('=')
			sb.WriteString(kvs[i+1])
			cmd.Env = append(cmd.Env, sb.String())
			sb.Reset()
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
		return nil
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

func DumpStdout(w io.Writer) Option {
	return func(cmd *exec.Cmd) {
		if cmd.Stdout == nil {
			cmd.Stdout = w
			return
		}

		cmd.Stdout = io.MultiWriter(cmd.Stdout, w)
	}
}

func DumpStderr(w io.Writer) Option {
	return func(cmd *exec.Cmd) {
		if cmd.Stderr == nil {
			cmd.Stderr = w
			return
		}

		cmd.Stderr = io.MultiWriter(cmd.Stderr, w)
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
