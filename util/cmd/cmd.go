package cmd

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/util/bufferpool"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/donkeywon/golib/util/proc"
)

type Cfg struct {
	Env        map[string]string `json:"env"        yaml:"env"`
	RunAsUser  string            `json:"runAsUser"  yaml:"runAsUser"`
	WorkingDir string            `json:"workingDir" yaml:"workingDir"`
	Command    []string          `json:"command"    validate:"required" yaml:"command"`
	SetPgid    bool              `json:"setPgid"    yaml:"setPgid"`
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

func (r *Result) markDone() {
	close(r.done)
}

func (r *Result) Done() <-chan struct{} {
	return r.done
}

func (r *Result) Err() error {
	return r.err
}

func (r *Result) String() string {
	buf := bufferpool.Get()
	defer buf.Free()

	buf.WriteString(`{"stdout":[`)
	for i := range r.Stdout {
		buf.WriteString(strconv.Quote(r.Stdout[i]))
		if i < len(r.Stdout)-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteString(`],"stderr":[`)
	for i := range r.Stderr {
		buf.WriteString(strconv.Quote(r.Stderr[i]))
		if i < len(r.Stderr)-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteString(`],"exitCode":`)
	buf.WriteString(strconv.Itoa(r.ExitCode))
	buf.WriteString(`,"pid":`)
	buf.WriteString(strconv.Itoa(r.Pid))
	buf.WriteString(`,"startTimeNano":`)
	buf.WriteString(strconv.FormatInt(r.StartTimeNano, 10))
	buf.WriteString(`,"stopTimeNano":`)
	buf.WriteString(strconv.FormatInt(r.StopTimeNano, 10))
	buf.WriteString(`,"signaled":`)
	buf.WriteString(strconv.FormatBool(r.Signaled))
	buf.WriteByte('}')
	return buf.String()
}

func Run(command ...string) *Result {
	return RunCtx(context.Background(), command...)
}

func RunCtx(ctx context.Context, command ...string) *Result {
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	r := Start(ctx, cmd, nil)
	<-r.Done()
	return r
}

// Start start a command
// you can get pid from Result.Pid, 0 means start fail.
func Start(ctx context.Context, cmd *exec.Cmd, cfg *Cfg, beforeStart ...func(cmd *exec.Cmd)) *Result {
	r := &Result{
		done: make(chan struct{}),
	}
	cfgBeforeStart, err := beforeStartFromCfg(cfg)
	if err != nil {
		r.err = err
		r.markDone()
		return r
	}

	beforeStart = append(cfgBeforeStart, beforeStart...)
	if len(beforeStart) > 0 {
		for _, f := range beforeStart {
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
	err = cmd.Start()
	r.err = err
	if err == nil {
		r.Pid = cmd.Process.Pid
	}

	go func() {
		defer r.markDone()
		err = wait(ctx, cmd, cfg, r)
		r.err = err
	}()

	return r
}

func wait(ctx context.Context, cmd *exec.Cmd, cfg *Cfg, startResult *Result) error {
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
				if cfg.SetPgid {
					_ = MustStopGroup(context.Background(), cmd, 5, proc.MustKillSignals...)
				} else {
					_ = MustStop(context.Background(), cmd, 5, proc.MustKillSignals...)
				}
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
	if waitErr != nil {
		if len(startResult.Stdout) > 0 || len(startResult.Stderr) > 0 {
			waitErr = errs.Wrapf(waitErr, "stdout: %s, stderr: %s", jsons.MustMarshalString(startResult.Stdout), jsons.MustMarshalString(startResult.Stderr))
		}
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

	return proc.Kill(cmd.Process.Pid, syscall.SIGTERM)
}

func StopGroup(cmd *exec.Cmd) error {
	if cmd == nil {
		return nil
	}
	if cmd.Process == nil {
		return nil
	}
	return proc.KillGroup(cmd.Process.Pid, syscall.SIGTERM)
}

func MustStop(ctx context.Context, cmd *exec.Cmd, singleSigWaitExitSec int, sig ...syscall.Signal) error {
	if cmd == nil {
		return nil
	}
	if cmd.Process == nil {
		return nil
	}

	return proc.MustKill(ctx, cmd.Process.Pid, singleSigWaitExitSec, sig...)
}

func MustStopGroup(ctx context.Context, cmd *exec.Cmd, singleSigWaitExitSec int, sig ...syscall.Signal) error {
	if cmd == nil {
		return nil
	}
	if cmd.Process == nil {
		return nil
	}
	return proc.MustKillGroup(ctx, cmd.Process.Pid, singleSigWaitExitSec, sig...)
}

func KillGroup(cmd *exec.Cmd) error {
	if cmd == nil {
		return nil
	}
	if cmd.Process == nil {
		return nil
	}
	return proc.KillGroup(cmd.Process.Pid, syscall.SIGKILL)
}
