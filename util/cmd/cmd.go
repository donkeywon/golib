package cmd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/donkeywon/golib/util/proc"
)

const (
	defaultWaitInterval = time.Second
	defaultWaitCount    = 5
)

type Cfg struct {
	Command      []string          `json:"command"       yaml:"command"  validate:"required"`
	Env          map[string]string `json:"env"           yaml:"env"`
	RunAsUser    string            `json:"run_as_user"   yaml:"runAsUser"`
	WorkingDir   string            `json:"working_dir"   yaml:"workingDir"`
	SetPgid      bool              `json:"set_pgid"      yaml:"setPgid"`
	Signals      []int             `json:"signals"       yaml:"signals"`
	WaitInterval time.Duration     `json:"wait_interval" yaml:"waitInterval"`
	WaitCount    int               `json:"wait_count"    yaml:"waitCount"`
}

type Result struct {
	cmd       *exec.Cmd
	stdoutBuf *bytes.Buffer
	stderrBuf *bytes.Buffer
	err       error
	done      chan struct{}

	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
	ExitCode      int    `json:"exit_code"`
	Pid           int    `json:"pid"`
	StartTimeNano int64  `json:"start_time_nano"`
	StopTimeNano  int64  `json:"stop_time_nano"`
	Signaled      bool   `json:"signaled"`
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

func (r *Result) Cmd() *exec.Cmd {
	return r.cmd
}

func (r *Result) StdoutLines() []string {
	if r.stdoutBuf == nil {
		return nil
	}
	return scanLines(r.stdoutBuf)
}

func (r *Result) StderrLines() []string {
	if r.stderrBuf == nil {
		return nil
	}
	return scanLines(r.stderrBuf)
}

func scanLines(r io.Reader) []string {
	lines := make([]string, 0, 32)
	s := bufio.NewScanner(r)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	return lines
}

func (r *Result) String() string {
	buf := make([]byte, 0, 150+(len(r.Stdout)+len(r.Stderr))*5/4)

	buf = append(buf, `{"stdout":`...)
	buf = strconv.AppendQuote(buf, r.Stdout)

	buf = append(buf, `,"stderr":`...)
	buf = strconv.AppendQuote(buf, r.Stderr)

	buf = append(buf, `,"exit_code":`...)
	buf = strconv.AppendInt(buf, int64(r.ExitCode), 10)

	buf = append(buf, `,"pid":`...)
	buf = strconv.AppendInt(buf, int64(r.Pid), 10)

	buf = append(buf, `,"start_time_nano":`...)
	buf = strconv.AppendInt(buf, r.StartTimeNano, 10)

	buf = append(buf, `,"stop_time_nano":`...)
	buf = strconv.AppendInt(buf, r.StopTimeNano, 10)

	buf = append(buf, `,"signaled":`...)
	buf = strconv.AppendBool(buf, r.Signaled)
	buf = append(buf, '}')

	return string(buf)
}

func Exec(ctx context.Context, command ...string) *Result {
	if len(command) == 0 {
		panic("empty command")
	}
	return Run(ctx, &Cfg{Command: command})
}

func Run(ctx context.Context, cfg *Cfg, beforeStart ...func(cmd *exec.Cmd)) *Result {
	if len(cfg.Command) == 0 {
		panic("empty command")
	}
	r := Start(ctx, cfg, beforeStart...)
	<-r.Done()
	return r
}

// Start command
// you can get pid from Result.Pid before <-Result.Done(), 0 means start fail.
func Start(ctx context.Context, cfg *Cfg, beforeStart ...func(cmd *exec.Cmd)) *Result {
	if ctx == nil {
		panic("nil context")
	}
	if len(cfg.Command) == 0 {
		panic("empty command")
	}

	r := &Result{
		cmd:  exec.Command(cfg.Command[0], cfg.Command[1:]...),
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
			f(r.cmd)
		}
	}

	if r.cmd.Stdout == nil {
		r.stdoutBuf = bytes.NewBuffer(nil)
		r.cmd.Stdout = r.stdoutBuf
	}
	if r.cmd.Stderr == nil {
		r.stderrBuf = bytes.NewBuffer(nil)
		r.cmd.Stderr = r.stderrBuf
	}
	r.StartTimeNano = time.Now().UnixNano()
	err = r.cmd.Start()
	r.err = err
	if err == nil {
		r.Pid = r.cmd.Process.Pid
	}

	go func() {
		defer r.markDone()
		r.err = wait(ctx, r.cmd, cfg, r)
	}()

	return r
}

func wait(ctx context.Context, cmd *exec.Cmd, cfg *Cfg, r *Result) error {
	var waitErr error
	if r.err == nil {
		cmdDone := make(chan struct{})
		go func() {
			waitInterval := cfg.WaitInterval
			if waitInterval <= 0 {
				waitInterval = defaultWaitInterval
			}
			waitCount := cfg.WaitCount
			if waitCount <= 0 {
				waitCount = defaultWaitCount
			}
			select {
			case <-ctx.Done():
				if cfg.SetPgid {
					_ = MustStopGroup(context.Background(), cmd, waitInterval, waitCount, proc.MustKillSignals...)
				} else {
					_ = MustStop(context.Background(), cmd, waitInterval, waitCount, proc.MustKillSignals...)
				}
			case <-cmdDone:
				return
			}
		}()
		waitErr = cmd.Wait()
		r.StopTimeNano = time.Now().UnixNano()
		close(cmdDone)
	} else {
		waitErr = r.err
	}

	r.Signaled = IsSignaled(waitErr)
	if cmd.ProcessState != nil {
		r.Pid = cmd.ProcessState.Pid()
		r.ExitCode = cmd.ProcessState.ExitCode()
	}

	if r.stdoutBuf != nil {
		r.Stdout = r.stdoutBuf.String()
	}
	if r.stderrBuf != nil {
		r.Stderr = r.stderrBuf.String()
	}
	return waitErr
}

// IsSignaled check if cmd exit signaled, err is Cmd.Wait() or Cmd.Run() error.
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

func MustStop(ctx context.Context, cmd *exec.Cmd, interval time.Duration, count int, sig ...syscall.Signal) error {
	if cmd == nil {
		return nil
	}
	if cmd.Process == nil {
		return nil
	}

	return proc.MustKill(ctx, cmd.Process.Pid, interval, count, sig...)
}

func MustStopGroup(ctx context.Context, cmd *exec.Cmd, interval time.Duration, count int, sig ...syscall.Signal) error {
	if cmd == nil {
		return nil
	}
	if cmd.Process == nil {
		return nil
	}
	return proc.MustKillGroup(ctx, cmd.Process.Pid, interval, count, sig...)
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
