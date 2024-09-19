package taskd

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/task"
	"github.com/donkeywon/golib/util/rands"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

var (
	td = New()
)

func TestMain(m *testing.M) {
	plugin.RegisterWithCfg(stepTypeTick, func() interface{} { return newTickStep() }, func() interface{} { return &tickStepCfg{Interval: 1} })
	cfg := NewCfg()
	cfg.PoolSize = 2
	cfg.QueueSize = 5

	td.Cfg = cfg
	tests.Init(td)

	runner.Init(td)

	os.Exit(m.Run())
}

func TestTaskd(t *testing.T) {
	runner.StartBG(td)

	maxNum := 10
	for i := range maxNum {
		go func(idx int) {
			r := rands.RandInt(1, maxNum)
			taskID := fmt.Sprintf("test-abc-%d", r)
			_, err := td.Submit(createTaskCfg(taskID, r))
			if err != nil {
				td.Error("submit task fail", err, "task_id", taskID)
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

	tsk, err := td.Submit(cfg)
	require.NoError(t, err)

	time.Sleep(time.Second * 5)
	err = td.Pause(tsk.Cfg.ID)
	require.NoError(t, err)

	td.Info("task result", "result", tsk.Result())

	time.Sleep(time.Second)
	tsk, err = td.Resume(tsk.Cfg.ID)
	require.NoError(t, err)

	<-tsk.Done()
	td.Info("task result", "result", tsk.Result())
}

const stepTypeTick task.StepType = "tick"

type tickStepCfg struct {
	Interval int
	Count    int
}

type tickStep struct {
	task.Step
	Cfg *tickStepCfg
}

func newTickStep() *tickStep {
	return &tickStep{
		Step: task.CreateBaseStep(string(stepTypeTick)),
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

func (t *tickStep) GetCfg() interface{} {
	return t.Cfg
}

func (t *tickStep) Type() interface{} {
	return stepTypeTick
}

func createTaskCfg(id string, tick int) *task.Cfg {
	return task.NewCfg().SetID(id).SetType("abc").Add(stepTypeTick, &tickStepCfg{
		Count: tick,
	})
}