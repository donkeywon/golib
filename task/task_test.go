package task

import (
	"testing"

	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/task/step"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

func TestTask(t *testing.T) {
	cfg := NewCfg().Add(step.TypeCmd, &cmd.Cfg{
		Command: []string{"echo", "1"},
	}).Add(step.TypeCmd, &cmd.Cfg{
		Command: []string{"echo", "2"},
	}).Defer(step.TypeCmd, &cmd.Cfg{
		Command: []string{"echo", "3"},
	}).Defer(step.TypeCmd, &cmd.Cfg{
		Command: []string{"echo", "4"},
	}).SetID("test-task").SetType(Type("test"))

	cfg.CurStepIdx = 0
	cfg.CurDeferStepIdx = 0

	task := New()
	task.Cfg = cfg
	tests.DebugInit(task)

	require.NoError(t, runner.Init(task))

	runner.Start(task)
	require.NoError(t, task.Err())

	task.Info("result", "result", task.Result())
}
