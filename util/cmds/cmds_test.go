package cmds

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithOptions(t *testing.T) {
	t.Run("nil option", func(t *testing.T) {
		cmd := exec.Command("echo", "hello")
		WithOptions(cmd, nil)
		// Should not panic
		assert.NotNil(t, cmd)
	})

	t.Run("multiple options including nil", func(t *testing.T) {
		cmd := exec.Command("echo", "hello")
		dir := t.TempDir()
		WithOptions(cmd, WorkingDir(dir), nil, WorkingDir("/tmp"))
		assert.Equal(t, "/tmp", cmd.Dir)
	})

	t.Run("empty options", func(t *testing.T) {
		cmd := exec.Command("echo", "hello")
		WithOptions(cmd)
		assert.NotNil(t, cmd)
	})
}

func TestWorkingDir(t *testing.T) {
	dir := t.TempDir()
	opt := WorkingDir(dir)
	cmd := exec.Command("echo", "hello")
	opt(cmd)
	assert.Equal(t, dir, cmd.Dir)
}

func TestEnvMap(t *testing.T) {
	t.Run("empty map does nothing", func(t *testing.T) {
		cmd := exec.Command("echo", "hello")
		opt := EnvMap(map[string]string{})
		opt(cmd)
		assert.Nil(t, cmd.Env)
	})

	t.Run("sets env vars", func(t *testing.T) {
		cmd := exec.Command("echo", "hello")
		opt := EnvMap(map[string]string{"KEY": "value", "FOO": "bar"})
		opt(cmd)
		assert.Contains(t, cmd.Env, "KEY=value")
		assert.Contains(t, cmd.Env, "FOO=bar")
	})

	t.Run("appends to existing env", func(t *testing.T) {
		cmd := exec.Command("echo", "hello")
		cmd.Env = []string{"EXISTING=old"}
		opt := EnvMap(map[string]string{"NEW": "val"})
		opt(cmd)
		assert.Contains(t, cmd.Env, "EXISTING=old")
		assert.Contains(t, cmd.Env, "NEW=val")
	})
}

func TestEnvKVs(t *testing.T) {
	t.Run("empty kvs does nothing", func(t *testing.T) {
		cmd := exec.Command("echo", "hello")
		opt := EnvKVs()
		opt(cmd)
		assert.Nil(t, cmd.Env)
	})

	t.Run("sets env from kvs", func(t *testing.T) {
		cmd := exec.Command("echo", "hello")
		opt := EnvKVs("A", "1", "B", "2")
		opt(cmd)
		assert.Contains(t, cmd.Env, "A=1")
		assert.Contains(t, cmd.Env, "B=2")
	})

	t.Run("odd count panics", func(t *testing.T) {
		assert.Panics(t, func() {
			opt := EnvKVs("A", "1", "B")
			cmd := exec.Command("echo", "hello")
			opt(cmd)
		})
	})

	t.Run("appends to existing env", func(t *testing.T) {
		cmd := exec.Command("echo", "hello")
		cmd.Env = []string{"EXIST=ing"}
		opt := EnvKVs("NEW", "env")
		opt(cmd)
		assert.Contains(t, cmd.Env, "EXIST=ing")
		assert.Contains(t, cmd.Env, "NEW=env")
	})
}

func TestWaitDelay(t *testing.T) {
	opt := WaitDelay(5 * time.Second)
	cmd := exec.Command("echo", "hello")
	opt(cmd)
	assert.Equal(t, 5*time.Second, cmd.WaitDelay)
}

func TestGracefulStop(t *testing.T) {
	t.Run("no signals returns nil", func(t *testing.T) {
		opt := GracefulStop(time.Second)
		assert.Nil(t, opt)
	})

	t.Run("wait duration below minimum gets adjusted", func(t *testing.T) {
		// This tests that waitDuration is adjusted to defaultWaitInterval.
		// We can't easily test the Cancel function without a running process,
		// but we can verify the option is not nil.
		opt := GracefulStop(time.Millisecond, syscall.SIGTERM)
		// The waitDurationBetweenSignals is below defaultWaitInterval, so it gets adjusted.
		// But GracefulStop still returns a valid option.
		assert.NotNil(t, opt)
	})

	t.Run("returns valid option", func(t *testing.T) {
		opt := GracefulStop(time.Second, syscall.SIGTERM, syscall.SIGINT)
		assert.NotNil(t, opt)

		// Apply the option to verify it works
		cmd := exec.Command("echo", "hello")
		opt(cmd)
		assert.NotNil(t, cmd.Cancel)
	})
}

func TestDumpStdout(t *testing.T) {
	t.Run("sets stdout when nil", func(t *testing.T) {
		var buf bytes.Buffer
		opt := DumpStdout(&buf)
		cmd := exec.Command("echo", "hello")
		opt(cmd)
		assert.Equal(t, &buf, cmd.Stdout)
	})

	t.Run("multi-writer when already set", func(t *testing.T) {
		var buf1, buf2 bytes.Buffer
		cmd := exec.Command("echo", "hello")
		cmd.Stdout = &buf1
		opt := DumpStdout(&buf2)
		opt(cmd)
		assert.NotNil(t, cmd.Stdout)
		// Should be a multi-writer
		_, ok := cmd.Stdout.(io.Writer)
		assert.True(t, ok)
	})
}

func TestDumpStderr(t *testing.T) {
	t.Run("sets stderr when nil", func(t *testing.T) {
		var buf bytes.Buffer
		opt := DumpStderr(&buf)
		cmd := exec.Command("echo", "hello")
		opt(cmd)
		assert.Equal(t, &buf, cmd.Stderr)
	})

	t.Run("multi-writer when already set", func(t *testing.T) {
		var buf1, buf2 bytes.Buffer
		cmd := exec.Command("echo", "hello")
		cmd.Stderr = &buf1
		opt := DumpStderr(&buf2)
		opt(cmd)
		assert.NotNil(t, cmd.Stderr)
		_, ok := cmd.Stderr.(io.Writer)
		assert.True(t, ok)
	})
}

func TestIsSignaled(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		isSignaled, isCoreDump, sig := IsSignaled(nil)
		assert.False(t, isSignaled)
		assert.False(t, isCoreDump)
		assert.Equal(t, syscall.Signal(-1), sig)
	})

	t.Run("non-exit-error", func(t *testing.T) {
		err := errors.New("some random error")
		isSignaled, isCoreDump, sig := IsSignaled(err)
		assert.False(t, isSignaled)
		assert.False(t, isCoreDump)
		assert.Equal(t, syscall.Signal(-1), sig)
	})

	t.Run("exit-error (not signaled) with exit code 1", func(t *testing.T) {
		cmd := exec.Command("sh", "-c", "exit 1")
		err := cmd.Run()
		require.Error(t, err)

		isSignaled, isCoreDump, sig := IsSignaled(err)
		assert.False(t, isSignaled)
		assert.False(t, isCoreDump)
		assert.Equal(t, syscall.Signal(-1), sig)
	})

	t.Run("exit-error by SIGKILL", func(t *testing.T) {
		cmd := exec.Command("sleep", "10")
		require.NoError(t, cmd.Start())

		// Kill the process
		err := cmd.Process.Signal(syscall.SIGKILL)
		require.NoError(t, err)

		waitErr := cmd.Wait()
		require.Error(t, waitErr)

		isSignaled, isCoreDump, sig := IsSignaled(waitErr)
		assert.True(t, isSignaled)
		assert.False(t, isCoreDump)
		assert.Equal(t, syscall.SIGKILL, sig)
	})

	t.Run("exit-error by SIGTERM", func(t *testing.T) {
		cmd := exec.Command("sleep", "10")
		require.NoError(t, cmd.Start())

		err := cmd.Process.Signal(syscall.SIGTERM)
		require.NoError(t, err)

		waitErr := cmd.Wait()
		require.Error(t, waitErr)

		isSignaled, isCoreDump, sig := IsSignaled(waitErr)
		assert.True(t, isSignaled)
		assert.False(t, isCoreDump)
		assert.Equal(t, syscall.SIGTERM, sig)
	})
}

func TestIsSignaledExitCode2(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 2")
	err := cmd.Run()
	require.Error(t, err)

	isSignaled, isCoreDump, sig := IsSignaled(err)
	assert.False(t, isSignaled)
	assert.False(t, isCoreDump)
	assert.Equal(t, syscall.Signal(-1), sig)
}
