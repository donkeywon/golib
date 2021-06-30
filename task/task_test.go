package task

import (
	"testing"

	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/test"
	"github.com/stretchr/testify/require"
)

func TestTask(t *testing.T) {
	cfg := NewCfg().Add(StepTypeCmd, &cmd.Cfg{
		Command: []string{"echo", "1"},
	}).Add(StepTypeCmd, &cmd.Cfg{
		Command: []string{"echo", "2"},
	}).Defer(StepTypeCmd, &cmd.Cfg{
		Command: []string{"echo", "3"},
	}).Defer(StepTypeCmd, &cmd.Cfg{
		Command: []string{"echo", "4"},
	}).SetID("test-task")

	cfg.CurStepIdx = 0
	cfg.CurDeferStepIdx = 3

	task := New()
	task.Cfg = cfg
	test.DebugInherit(task)

	require.NoError(t, runner.Init(task))

	runner.Start(task)
	require.NoError(t, task.Err())

	task.Info("result", "result", task.Result())
}
