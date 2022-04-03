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
	Env        map[string]string `json:"env"        yaml:"env"`
	RunAsUser  string            `json:"runAsUser"  yaml:"runAsUser"`
	WorkingDir string            `json:"workingDir" yaml:"workingDir"`
	Command    []string          `json:"command"    validate:"required" yaml:"command"`
}

func NewCfg() *Cfg {
	return &Cfg{}
}

type Result struct {
	err           error
	stdoutBuf     *bufferpool.Buffer
	stderrBuf     *bufferpool.Buffer
	done          chan struct{}
	Stdout        []string `json:"stdout"`
	Stderr        []string `json:"stderr"`
	ExitCode      int      `json:"exitCode"`
	Pid           int      `json:"pid"`
	StartTimeNano int64    `json:"startTimeNano"`
	StopTimeNano  int64    `json:"stopTimeNano"`
	Signaled      bool     `json:"signaled"`
}

func (r *Result) Done() <-chan struct{} {
	return r.done
}

func (r *Result) Err() error {
	return r.err
}

func Run(command ...string) *Result {
	return RunCtx(context.Background(), command...)
}

func RunCtx(ctx context.Context, command ...string) *Result {
	return RunRaw(ctx, &Cfg{Command: command})
}

func RunRaw(ctx context.Context, cfg *Cfg, beforeStart ...func(cmd *exec.Cmd)) *Result {
	cmd := exec.CommandContext(ctx, cfg.Command[0], cfg.Command[1:]...)
	return RunCmd(ctx, cmd, cfg, beforeStart...)
}

func RunCmd(ctx context.Context, cmd *exec.Cmd, cfg *Cfg, beforeStart ...func(cmd *exec.Cmd)) *Result {
	cfgBeforeStart, err := beforeStartFromCfg(cfg)
	if err != nil {
		return &Result{err: err}
	}
	r := Start(cmd, append(cfgBeforeStart, beforeStart...)...)
	Wait(ctx, cmd, r)
	<-r.Done()
	return r
}

// Start start a command
// you can get pid from Result.Pid, 0 means start fail.
// Must call Wait after Start whether cmd fail or not.
func Start(cmd *exec.Cmd, beforeRun ...func(cmd *exec.Cmd)) *Result {
	r := &Result{
		done: make(chan struct{}),
	}
	if len(beforeRun) > 0 {
		for _, f := range beforeRun {
			f(cmd)
		}
	}

	if cmd.Stdout == nil {
		r.stdoutBuf = bufferpool.Get()
		cmd.Stdout = r.stdoutBuf
	}
	if cmd.Stderr == nil {
		r.stderrBuf = bufferpool.Get()
		cmd.Stderr = r.stderrBuf
	}
	r.StartTimeNano = time.Now().UnixNano()
	err := cmd.Start()
	r.err = err
	if err == nil {
		r.Pid = cmd.Process.Pid
	}

	return r
}

// Wait wait command exit
// Must called after Start.
func Wait(ctx context.Context, cmd *exec.Cmd, startResult *Result) {
	go func() {
		defer close(startResult.done)
		err := wait(ctx, cmd, startResult)
		startResult.err = err
	}()
}

func wait(ctx context.Context, cmd *exec.Cmd, startResult *Result) error {
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
	if startResult.err == nil {
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
	} else {
		waitErr = startResult.err
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
	return waitErr
}

// IsSignaled check if cmd exit signaled, err is Cmd.Wait() or Cmd.Run() result error.
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
	if cmd == nil {
		return nil
	}
	if cmd.Process == nil {
		return nil
	}

	return proc.Stop(cmd.Process.Pid)
}

func MustStop(ctx context.Context, cmd *exec.Cmd) error {
	if cmd == nil {
		return nil
	}
	if cmd.Process == nil {
		return nil
	}

	return proc.MustStop(ctx, cmd.Process.Pid)
}
