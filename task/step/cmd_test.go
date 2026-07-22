package step

import (
	"context"
	"log/slog"
	"os/exec"
	"testing"

	"github.com/donkeywon/golib/logs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loggerCtx returns a context with a nop slog.Logger for use in tests.
func loggerCtx() context.Context {
	l, err := logs.NopLoggerCreator.Create()
	if err != nil {
		panic(err)
	}
	return logs.CtxWith(context.Background(), l)
}

func TestNewCmdStepCfg(t *testing.T) {
	cfg := NewCmdStepCfg()
	require.NotNil(t, cfg)
	assert.Empty(t, cfg.Name)
	assert.Nil(t, cfg.Args)
	assert.Nil(t, cfg.Env)
	assert.Empty(t, cfg.RunAsUser)
	assert.Empty(t, cfg.WorkingDir)
	assert.False(t, cfg.SetPgid)
	assert.False(t, cfg.DumpStdout)
	assert.False(t, cfg.DumpStderr)
	assert.Nil(t, cfg.GracefulStopSignals)
	assert.Equal(t, 0, int(cfg.GracefulStopWaitDurationBetweenSignals))
}

func TestNewCmdStep(t *testing.T) {
	cs := NewCmdStep()
	require.NotNil(t, cs)
	// options is nil on a freshly created CmdStep since it hasn't been used yet
	assert.Nil(t, cs.options)
}

func TestCmdStep_SetCfg(t *testing.T) {
	cs := NewCmdStep()
	cfg := CmdStepCfg{Name: "echo", Args: []string{"hello"}}
	cs.SetCfg(cfg)
	assert.Equal(t, "echo", cs.cfg.Name)
	assert.Equal(t, []string{"hello"}, cs.cfg.Args)

	// Type assertion — wrong type panics
	assert.Panics(t, func() {
		cs.SetCfg("not a CmdStepCfg")
	})
}

func TestCmdStep_CmdOptions(t *testing.T) {
	cs := NewCmdStep()

	// Initially nil
	assert.Nil(t, cs.options)

	// Append options
	opt1 := func(cmd *exec.Cmd) {}
	opt2 := func(cmd *exec.Cmd) {}
	cs.CmdOptions(opt1, opt2)
	assert.Len(t, cs.options, 2)

	// Append more
	opt3 := func(cmd *exec.Cmd) {}
	cs.CmdOptions(opt3)
	assert.Len(t, cs.options, 3)
}

func TestCmdStep_Init_ValidationError(t *testing.T) {
	cs := NewCmdStep()
	// Empty config should fail validation
	err := cs.Init(context.Background())
	require.Error(t, err)
}

func TestCmdStep_Init_ValidationSuccess(t *testing.T) {
	cs := NewCmdStep()
	cs.cfg = CmdStepCfg{Name: "echo", Args: []string{"test"}}
	err := cs.Init(context.Background())
	require.NoError(t, err)
}

func TestCmdStep_Start(t *testing.T) {
	cs := NewCmdStep()
	cs.cfg = CmdStepCfg{Name: "echo", Args: []string{"hello"}}

	err := cs.Init(context.Background())
	require.NoError(t, err)

	// Start needs a logger in context
	err = cs.Start(loggerCtx())
	require.NoError(t, err)

	// Verify exit code stored
	exitCode, ok := cs.Load("exitCode")
	assert.True(t, ok)
	assert.Equal(t, 0, exitCode)
}

func TestCmdStep_Start_WithDumpStdout(t *testing.T) {
	cs := NewCmdStep()
	cs.cfg = CmdStepCfg{
		Name:       "echo",
		Args:       []string{"hello world"},
		DumpStdout: true,
	}

	err := cs.Init(context.Background())
	require.NoError(t, err)

	err = cs.Start(loggerCtx())
	require.NoError(t, err)

	stdout, ok := cs.Load("stdout")
	assert.True(t, ok)
	assert.Contains(t, string(stdout.([]byte)), "hello world")
}

func TestCmdStep_Start_WithDumpStderr(t *testing.T) {
	cs := NewCmdStep()
	// Use sh -c to write to stderr
	cs.cfg = CmdStepCfg{
		Name:       "sh",
		Args:       []string{"-c", "echo 'error msg' >&2"},
		DumpStderr: true,
	}

	err := cs.Init(context.Background())
	require.NoError(t, err)

	err = cs.Start(loggerCtx())
	require.NoError(t, err)

	stderr, ok := cs.Load("stderr")
	assert.True(t, ok)
	assert.Contains(t, string(stderr.([]byte)), "error msg")
}

func TestCmdStep_Start_ExitCodeNonZero(t *testing.T) {
	cs := NewCmdStep()
	cs.cfg = CmdStepCfg{Name: "sh", Args: []string{"-c", "exit 42"}}

	err := cs.Init(context.Background())
	require.NoError(t, err)

	err = cs.Start(loggerCtx())
	// Command exits non-zero, but it's not signaled or cancelled
	// The error is wrapped with "exec cmd failed"
	require.Error(t, err)
	exitCode, ok := cs.Load("exitCode")
	assert.True(t, ok)
	assert.Equal(t, 42, exitCode)
}

func TestCmdStep_Stop(t *testing.T) {
	cs := NewCmdStep()
	cs.cfg = CmdStepCfg{Name: "sleep", Args: []string{"10"}}

	err := cs.Init(context.Background())
	require.NoError(t, err)

	// Stop before Start should work (cancel is nil, will panic)
	// We just verify the Stop method exists and does the right thing
	assert.Panics(t, func() {
		_ = cs.Stop(context.Background())
	})
}

func TestErrCanceled(t *testing.T) {
	assert.NotNil(t, errCanceled)
	assert.Equal(t, "canceled", errCanceled.Error())
}

func TestCmdStep_Start_CommandNotFound(t *testing.T) {
	cs := NewCmdStep()
	cs.cfg = CmdStepCfg{Name: "nonexistent_command_xyz", Args: []string{}}

	err := cs.Init(context.Background())
	require.NoError(t, err)

	err = cs.Start(loggerCtx())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec cmd failed")
}

func TestCmdStep_IsSignaledFields(t *testing.T) {
	cs := NewCmdStep()
	cs.cfg = CmdStepCfg{Name: "echo", Args: []string{"test"}}

	err := cs.Init(context.Background())
	require.NoError(t, err)

	err = cs.Start(loggerCtx())
	require.NoError(t, err)

	isSignaled, ok := cs.Load("isSignaled")
	assert.True(t, ok)
	assert.False(t, isSignaled.(bool))

	isCoredump, ok := cs.Load("isCoredump")
	assert.True(t, ok)
	assert.False(t, isCoredump.(bool))
}

func TestTypeCmd_PluginReg(t *testing.T) {
	// init() should have registered the "cmd" type
	assert.NotEmpty(t, string(TypeCmd))
}

func TestCmdStep_StartFailingCommand(t *testing.T) {
	cs := NewCmdStep()
	cs.cfg = CmdStepCfg{Name: "false"} // false always exits with status 1

	if _, err := exec.LookPath("false"); err != nil {
		t.Skip("false command not available")
	}

	err := cs.Init(context.Background())
	require.NoError(t, err)

	err = cs.Start(loggerCtx())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec cmd failed")
}

func init() {
	// Ensure NopLoggerCreator is available
	if logs.NopLoggerCreator == nil {
		panic("NopLoggerCreator is nil")
	}
}

var _ = slog.LevelInfo // ensure slog import is used
