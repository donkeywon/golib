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
}

func Run(command ...string) (*Result, error) {
	cfg := &Cfg{
		Command: command,
	}
	return RunRaw(context.Background(), nil, cfg)
}

func RunRaw(ctx context.Context, beforeRun func(cmd *exec.Cmd), cfg *Cfg) (*Result, error) {
	cmd, err := Create(cfg)
	if err != nil {
		return nil, err
	}
	return RunCmd(ctx, beforeRun, cmd)
}

func RunCmd(ctx context.Context, beforeRun func(cmd *exec.Cmd), cmd *exec.Cmd) (*Result, error) {
	stderrBuf := bufferpool.GetBuffer()
	defer stderrBuf.Free()
	stdoutBuf := bufferpool.GetBuffer()
	defer stdoutBuf.Free()

	cmd.Stderr = stderrBuf
	cmd.Stdout = stdoutBuf

	if beforeRun != nil {
		beforeRun(cmd)
	}

	startTSNano := time.Now().UnixNano()
	var stopTSNano int64
	err := cmd.Start()
	if err == nil {
		cmdDone := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_ = MustStop(context.Background(), cmd)
			case <-cmdDone:
				return
			}
		}()
		err = cmd.Wait()
		stopTSNano = time.Now().UnixNano()
		close(cmdDone)
	}

	pid := 0
	if cmd.ProcessState != nil {
		pid = cmd.ProcessState.Pid()
	}

	signaled := IsSignaled(err)
	exitCode := cmd.ProcessState.ExitCode()

	stderr := stderrBuf.Lines()
	stdout := stdoutBuf.Lines()

	r := &Result{
		Stdout:        stdout,
		Stderr:        stderr,
		ExitCode:      exitCode,
		Pid:           pid,
		StartTimeNano: startTSNano,
		StopTimeNano:  stopTSNano,
		Signaled:      signaled,
	}
	return r, err
}

// err is Cmd.Wait() or Cmd.Run() result error.
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
