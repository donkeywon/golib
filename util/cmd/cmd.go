package cmd

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"

	"github.com/donkeywon/golib/util/bufferpool"
	"github.com/donkeywon/golib/util/proc"
)

type Cfg struct {
	Command    []string          `json:"command"    validate:"required,min=1" yaml:"command"`
	Env        map[string]string `json:"env"        yaml:"env"`
	RunAsUser  string            `json:"runAsUser"  yaml:"runAsUser"`
	WorkingDir string            `json:"workingDir" yaml:"workingDir"`
}

type Result struct {
	Stdout        []string `json:"stdout"`
	Stderr        []string `json:"stderr"`
	ExitCode      int      `json:"exitCode"`
	Pid           int      `json:"pid"`
	StartTimeNano int64    `json:"startTimeNano"`
	StopTimeNano  int64    `json:"stopTimeNano"`
	Signaled      bool     `json:"signaled"`

	stdoutBuf *bufferpool.Buffer
	stderrBuf *bufferpool.Buffer
}

func Run(command ...string) (*Result, error) {
	cfg := &Cfg{
		Command: command,
	}
	return RunRaw(context.Background(), cfg)
}

func RunRaw(ctx context.Context, cfg *Cfg, beforeRun ...func(cmd *exec.Cmd)) (*Result, error) {
	cmd, err := Create(cfg)
	if err != nil {
		return nil, err
	}
	return RunCmd(ctx, cmd, beforeRun...)
}

func RunCmd(ctx context.Context, cmd *exec.Cmd, beforeRun ...func(cmd *exec.Cmd)) (*Result, error) {
	r, err := StartCmd(cmd, beforeRun...)
	return WaitCmd(ctx, cmd, r, err)
}

// StartCmd start a command
// if error is nil, you can get pid from Result.Pid.
// Must call WaitCmd after StartCmd whether error is nil or not
func StartCmd(cmd *exec.Cmd, beforeRun ...func(cmd *exec.Cmd)) (*Result, error) {
	if len(beforeRun) > 0 {
		for _, f := range beforeRun {
			f(cmd)
		}
	}

	r := &Result{}

	if cmd.Stdout == nil {
		r.stdoutBuf = bufferpool.GetBuffer()
		cmd.Stdout = r.stdoutBuf
	}
	if cmd.Stderr == nil {
		r.stderrBuf = bufferpool.GetBuffer()
		cmd.Stderr = r.stderrBuf
	}
	r.StartTimeNano = time.Now().UnixNano()
	err := cmd.Start()
	if err == nil {
		r.Pid = cmd.Process.Pid
	}

	return r, err
}

// WaitCmd wait command exit
// Must called after StartCmd
func WaitCmd(ctx context.Context, cmd *exec.Cmd, startResult *Result, startErr error) (*Result, error) {
	if startResult.stdoutBuf != nil {
		defer func() {
			startResult.stdoutBuf.Free()
			startResult.stdoutBuf = nil
		}()
	}
	if startResult.stderrBuf != nil {
		defer func() {
			startResult.stderrBuf.Free()
			startResult.stderrBuf = nil
		}()
	}

	var waitErr error
	if startErr == nil {
		cmdDone := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_ = MustStop(context.Background(), cmd)
			case <-cmdDone:
				return
			}
		}()
		waitErr = cmd.Wait()
		startResult.StopTimeNano = time.Now().UnixNano()
		close(cmdDone)
	}

	startResult.Signaled = IsSignaled(waitErr)
	if cmd.ProcessState != nil {
		startResult.Pid = cmd.ProcessState.Pid()
		startResult.ExitCode = cmd.ProcessState.ExitCode()
	}

	if startResult.stdoutBuf != nil {
		startResult.Stdout = startResult.stdoutBuf.Lines()
	}
	if startResult.stderrBuf != nil {
		startResult.Stderr = startResult.stderrBuf.Lines()
	}
	return startResult, waitErr
}

// IsSignaled err is Cmd.Wait() or Cmd.Run() result error.
func IsSignaled(err error) bool {
	if err == nil {
		return false
	}

	exitErr := &exec.ExitError{}
	if errors.As(err, &exitErr) {
		if waitStatus, ok1 := exitErr.Sys().(syscall.WaitStatus); ok1 {
			if waitStatus.Signaled() {
				return true
			}
		}
	}
	return false
}

func Stop(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}

	return proc.Stop(cmd.Process.Pid)
}

func MustStop(ctx context.Context, cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}

	return proc.MustStop(ctx, cmd.Process.Pid)
}
