package task

import (
	"testing"
	"time"

	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/task/step"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

func TestTask(t *testing.T) {
	cfg := NewCfg().Add(step.TypeCmd, &cmd.Cfg{
		Command: []string{"sleep", "5"},
	}).Add(step.TypeCmd, &cmd.Cfg{
		Command: []string{"sleep", "10"},
	}).Defer(step.TypeCmd, &cmd.Cfg{
		Command: []string{"sleep", "12"},
	}).Defer(step.TypeCmd, &cmd.Cfg{
		Command: []string{"sleep", "13"},
	}).SetID("test-task").SetType(Type("test"))

	cfg.CurStepIdx = 0
	cfg.CurDeferStepIdx = 0

	task := New()
	task.Cfg = cfg
	tests.DebugInit(task)

	require.NoError(t, runner.Init(task))

	go func() {
		time.Sleep(1 * time.Second)
		runner.Stop(task)
	}()

	runner.Start(task)
	require.NoError(t, task.Err())

	task.Info("result", "result", task.Result())
}
