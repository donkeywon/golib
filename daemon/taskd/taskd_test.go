package taskd

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/task"
	"github.com/donkeywon/golib/task/step"
	"github.com/donkeywon/golib/util/rands"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

var (
	tdtest = New().(*taskd)
)

func TestMain(m *testing.M) {
	plugin.Reg(stepTypeTick, newTickStep, func() any { return &tickStepCfg{Interval: 1} })
	cfg := NewCfg()
	cfg.Pools[0].Size = 2
	cfg.Pools[0].QueueSize = 5

	tdtest.cfg = cfg
	tests.Init(tdtest)

	runner.Init(tdtest)

	os.Exit(m.Run())
}

func TestTaskd(t *testing.T) {
	runner.Start(tdtest)

	maxNum := 10
	for i := range maxNum {
		go func(idx int) {
			r := rands.RandInt(1, maxNum)
			taskID := fmt.Sprintf("test-abc-%d", r)
			_, err := tdtest.SubmitTask(createTaskCfg(taskID, r))
			if err != nil {
				tdtest.Error("submit task failed", err, "task_id", taskID)
			}
		}(i)
	}

	time.Sleep(time.Second * 1000)
}

func TestPause(t *testing.T) {
	cfg := task.NewCfg().SetID("test-pause").SetType("abc").Add(stepTypeTick, &tickStepCfg{
		Count: 3,
	}).Add(stepTypeTick, &tickStepCfg{
		Count: 20,
	}).Add(stepTypeTick, &tickStepCfg{
		Count: 5,
	})

	tsk, err := tdtest.SubmitTask(cfg)
	require.NoError(t, err)

	time.Sleep(time.Second * 5)
	err = tdtest.PauseTask(tsk.Cfg.ID)
	require.NoError(t, err)

	tdtest.Info("task result", "result", tsk.Result())

	time.Sleep(time.Second)
	tsk, err = tdtest.ResumeTask(tsk.Cfg.ID)
	require.NoError(t, err)

	<-tsk.Done()
	tdtest.Info("task result", "result", tsk.Result())
}

const stepTypeTick step.Type = "tick"

type tickStepCfg struct {
	Interval int
	Count    int
}

type tickStep struct {
	step.Step
	Cfg *tickStepCfg
}

func newTickStep() *tickStep {
	return &tickStep{
		Step: step.CreateBase(string(stepTypeTick)),
	}
}

func (t *tickStep) Start() error {
	ticker := time.NewTicker(time.Second * time.Duration(t.Cfg.Interval))
	for i := range t.Cfg.Count {
		select {
		case <-t.Stopping():
			return nil
		case <-ticker.C:
			t.Info("tick", "i", i)
		}
	}
	t.Store("field_test", fmt.Sprintf("%d-%d", t.Cfg.Interval, t.Cfg.Count))
	return nil
}

func createTaskCfg(id string, tick int) *task.Cfg {
	return task.NewCfg().SetID(id).SetType("abc").Add(stepTypeTick, &tickStepCfg{
		Count: tick,
	})
}
