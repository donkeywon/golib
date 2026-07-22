package taskd

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/logs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/task"
	"github.com/donkeywon/golib/task/step"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Cfg tests
// ---------------------------------------------------------------------------

func TestNewCfg(t *testing.T) {
	c := NewCfg()
	require.Len(t, c.Pools, 1)
	assert.Equal(t, DefaultPool, c.Pools[0].Name)
	assert.Equal(t, DefaultPoolSize, c.Pools[0].Size)
	assert.Equal(t, DefaultQueueSize, c.Pools[0].QueueSize)
}

func TestPoolCfg(t *testing.T) {
	pc := PoolCfg{Name: "test", Size: 10, QueueSize: 100}
	assert.Equal(t, "test", pc.Name)
	assert.Equal(t, 10, pc.Size)
	assert.Equal(t, 100, pc.QueueSize)
}

// ---------------------------------------------------------------------------
// Error sentinel tests
// ---------------------------------------------------------------------------

func TestSentinelErrors(t *testing.T) {
	assert.EqualError(t, ErrStopping, "stopping, reject")
	assert.EqualError(t, ErrTaskNotExists, "task not exists")
	assert.EqualError(t, ErrTaskAlreadyExists, "task already exists")
	assert.EqualError(t, ErrTaskAlreadyStopping, "task already stopping")
	assert.EqualError(t, ErrTaskAlreadyPausing, "task already pausing")
	assert.EqualError(t, ErrTaskNotStarted, "task not started")
	assert.EqualError(t, ErrTaskNotPaused, "task not paused")
	assert.EqualError(t, ErrPoolNotExists, "pool not exists")
}

// ---------------------------------------------------------------------------
// TaskState constants
// ---------------------------------------------------------------------------

func TestTaskStateValues(t *testing.T) {
	assert.Equal(t, TaskState(0), TaskStatePending)
	assert.Equal(t, TaskState(1), TaskStateRunning)
	assert.Equal(t, TaskState(2), TaskStateDone)
	assert.Equal(t, TaskState(3), TaskStatePausing)
	assert.Equal(t, TaskState(4), TaskStatePaused)
}

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	td := New()
	require.NotNil(t, td)
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func TestInit(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		pools:       make(map[string]pond.Pool),
	}
	td.cfg = &Cfg{Pools: []PoolCfg{{Name: "default", Size: 5, QueueSize: 10}}}
	err := td.Init(context.Background())
	require.NoError(t, err)
	require.NotNil(t, td.pools["default"])
}

func TestInit_MultiplePools(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		pools:       make(map[string]pond.Pool),
	}
	td.cfg = &Cfg{Pools: []PoolCfg{
		{Name: "default", Size: 5, QueueSize: 10},
		{Name: "io", Size: 10, QueueSize: 100},
	}}
	err := td.Init(context.Background())
	require.NoError(t, err)
	require.NotNil(t, td.pools["default"])
	require.NotNil(t, td.pools["io"])
}

func TestInit_NoPools(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		pools:       make(map[string]pond.Pool),
	}
	td.cfg = &Cfg{}
	err := td.Init(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pools")
}

// ---------------------------------------------------------------------------
// SetCfg
// ---------------------------------------------------------------------------

func TestSetCfg(t *testing.T) {
	td := &taskd{}
	cfg := &Cfg{Pools: []PoolCfg{{Name: "x", Size: 1, QueueSize: 1}}}
	td.SetCfg(cfg)
	assert.Equal(t, cfg, td.cfg)
}

// ---------------------------------------------------------------------------
// Stop
// ---------------------------------------------------------------------------

func TestStop_NoCancel(t *testing.T) {
	td := &taskd{}
	err := td.Stop(context.Background())
	require.NoError(t, err)
}

func TestStop_WithCancel(t *testing.T) {
	td := &taskd{}
	ctx, cancel := context.WithCancel(context.Background())
	td.ctx = ctx
	td.cancel = cancel
	err := td.Stop(context.Background())
	require.NoError(t, err)
	assert.Error(t, td.ctx.Err())
}

// ---------------------------------------------------------------------------
// addTaskInfo
// ---------------------------------------------------------------------------

func TestAddTaskInfo_Success(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	ok := td.addTaskInfo("task1", "default")
	assert.True(t, ok)
	assert.True(t, td.IsTaskExists("task1"))
}

func TestAddTaskInfo_Duplicate(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	ok := td.addTaskInfo("task1", "default")
	assert.True(t, ok)
	ok = td.addTaskInfo("task1", "other")
	assert.False(t, ok)
}

func TestAddTaskInfo_Multiple(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	assert.True(t, td.addTaskInfo("a", "pool1"))
	assert.True(t, td.addTaskInfo("b", "pool2"))
	assert.True(t, td.addTaskInfo("c", "pool3"))
	assert.Equal(t, 3, len(td.taskInfoMap))
}

// ---------------------------------------------------------------------------
// setTaskToTaskInfo
// ---------------------------------------------------------------------------

func makeCfg(id string) task.Cfg {
	c := task.NewCfg()
	c.SetID(id)
	return c
}

func TestSetTaskToTaskInfo_Exists(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	tk := task.New()
	tk.SetCfg(makeCfg("my-task"))
	td.addTaskInfo("my-task", "default")
	td.setTaskToTaskInfo(tk)
	assert.Equal(t, tk, td.taskInfoMap["my-task"].task)
}

func TestSetTaskToTaskInfo_NotExists(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	tk := task.New()
	tk.SetCfg(makeCfg("nonexistent"))
	// Should not panic
	td.setTaskToTaskInfo(tk)
}

// ---------------------------------------------------------------------------
// removeTaskInfo
// ---------------------------------------------------------------------------

func TestRemoveTaskInfo(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.addTaskInfo("task1", "default")
	td.removeTaskInfo("task1")
	assert.False(t, td.IsTaskExists("task1"))
}

func TestRemoveTaskInfo_NotExists(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	// Should not panic
	td.removeTaskInfo("nonexistent")
}

// ---------------------------------------------------------------------------
// changeTaskState
// ---------------------------------------------------------------------------

func TestChangeTaskState_Success(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.addTaskInfo("task1", "default")
	ok := td.changeTaskState("task1", TaskStatePending, TaskStateRunning)
	assert.True(t, ok)

	state, err := td.GetTaskState("task1")
	require.NoError(t, err)
	assert.Equal(t, TaskStateRunning, state)
}

func TestChangeTaskState_WrongFromState(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.addTaskInfo("task1", "default")
	// Transition Pending -> Running
	td.changeTaskState("task1", TaskStatePending, TaskStateRunning)
	// Now try Pending -> Running again (state is Running, not Pending)
	ok := td.changeTaskState("task1", TaskStatePending, TaskStateRunning)
	assert.False(t, ok)

	state, _ := td.GetTaskState("task1")
	assert.Equal(t, TaskStateRunning, state)
}

func TestChangeTaskState_NotExists(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	ok := td.changeTaskState("nonexistent", TaskStatePending, TaskStateRunning)
	assert.False(t, ok)
}

func TestChangeTaskState_Chain(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.addTaskInfo("t", "p")

	assert.True(t, td.changeTaskState("t", TaskStatePending, TaskStateRunning))
	assert.True(t, td.changeTaskState("t", TaskStateRunning, TaskStatePausing))
	assert.True(t, td.changeTaskState("t", TaskStatePausing, TaskStatePaused))
	assert.True(t, td.changeTaskState("t", TaskStatePaused, TaskStateDone))

	state, err := td.GetTaskState("t")
	require.NoError(t, err)
	assert.Equal(t, TaskStateDone, state)
}

// ---------------------------------------------------------------------------
// getTask
// ---------------------------------------------------------------------------

func TestGetTask(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	assert.Nil(t, td.getTask("nonexistent"))

	tk := task.New()
	tk.SetCfg(makeCfg("real"))
	td.addTaskInfo("real", "default")
	td.setTaskToTaskInfo(tk)
	assert.Equal(t, tk, td.getTask("real"))
}

// ---------------------------------------------------------------------------
// IsTaskExists
// ---------------------------------------------------------------------------

func TestIsTaskExists(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	assert.False(t, td.IsTaskExists("x"))
	td.addTaskInfo("x", "default")
	assert.True(t, td.IsTaskExists("x"))
	td.removeTaskInfo("x")
	assert.False(t, td.IsTaskExists("x"))
}

// ---------------------------------------------------------------------------
// isTaskPausing
// ---------------------------------------------------------------------------

func TestIsTaskPausing(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	assert.False(t, td.isTaskPausing("x"))

	td.addTaskInfo("x", "default")
	assert.False(t, td.isTaskPausing("x"))

	td.changeTaskState("x", TaskStatePending, TaskStatePausing)
	assert.True(t, td.isTaskPausing("x"))
}

// ---------------------------------------------------------------------------
// GetTaskState
// ---------------------------------------------------------------------------

func TestGetTaskState(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	_, err := td.GetTaskState("x")
	assert.ErrorIs(t, err, ErrTaskNotExists)

	td.addTaskInfo("x", "default")
	state, err := td.GetTaskState("x")
	require.NoError(t, err)
	assert.Equal(t, TaskStatePending, state)

	td.changeTaskState("x", TaskStatePending, TaskStateRunning)
	state, err = td.GetTaskState("x")
	require.NoError(t, err)
	assert.Equal(t, TaskStateRunning, state)
}

// ---------------------------------------------------------------------------
// GetTaskCfg
// ---------------------------------------------------------------------------

func TestGetTaskCfg(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	_, err := td.GetTaskCfg("x")
	assert.ErrorIs(t, err, ErrTaskNotExists)

	tk := task.New()
	cfg := makeCfg("cfg-task")
	tk.SetCfg(cfg)
	td.addTaskInfo("cfg-task", "default")
	td.setTaskToTaskInfo(tk)

	got, err := td.GetTaskCfg("cfg-task")
	require.NoError(t, err)
	assert.Equal(t, "cfg-task", got.ID)
}

// ---------------------------------------------------------------------------
// ListTasks
// ---------------------------------------------------------------------------

func TestListTasks_Empty(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	assert.Empty(t, td.ListTasks())
}

func TestListTasks_WithPaused(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}

	tk1 := task.New()
	tk1.SetCfg(makeCfg("t1"))
	td.addTaskInfo("t1", "default")
	td.setTaskToTaskInfo(tk1)

	tk2 := task.New()
	tk2.SetCfg(makeCfg("t2"))
	td.addTaskInfo("t2", "default")
	td.setTaskToTaskInfo(tk2)

	// Set t2 as paused
	td.taskInfoMap["t2"].state.Store(uint32(TaskStatePaused))

	tasks := td.ListTasks()
	assert.Len(t, tasks, 1)
	assert.Equal(t, tk1, tasks[0])
}

func TestListTasks_AllRunning(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}

	for _, id := range []string{"a", "b", "c"} {
		tk := task.New()
		tk.SetCfg(makeCfg(id))
		td.addTaskInfo(id, "default")
		td.setTaskToTaskInfo(tk)
		td.taskInfoMap[id].state.Store(uint32(TaskStateRunning))
	}

	tasks := td.ListTasks()
	assert.Len(t, tasks, 3)
}

// ---------------------------------------------------------------------------
// ListTasksCfg
// ---------------------------------------------------------------------------

func TestListTasksCfg(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	assert.Empty(t, td.ListTasksCfg())

	tk := task.New()
	cfg := makeCfg("cfg-list")
	tk.SetCfg(cfg)
	td.addTaskInfo("cfg-list", "default")
	td.setTaskToTaskInfo(tk)

	cfgs := td.ListTasksCfg()
	assert.Len(t, cfgs, 1)
	assert.Equal(t, "cfg-list", cfgs[0].ID)
}

// ---------------------------------------------------------------------------
// ListTaskIDs
// ---------------------------------------------------------------------------

func TestListTaskIDs(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	assert.Empty(t, td.ListTaskIDs())

	td.addTaskInfo("id1", "default")
	td.addTaskInfo("id2", "default")
	ids := td.ListTaskIDs()
	assert.Len(t, ids, 2)
	assert.Contains(t, ids, "id1")
	assert.Contains(t, ids, "id2")
}

// ---------------------------------------------------------------------------
// ListPendingTaskIDs / ListRunningTaskIDs / ListPausingTaskIDs / ListPausedTaskIDs
// ---------------------------------------------------------------------------

func TestListByState(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	assert.Empty(t, td.ListPendingTaskIDs())
	assert.Empty(t, td.ListRunningTaskIDs())
	assert.Empty(t, td.ListPausingTaskIDs())
	assert.Empty(t, td.ListPausedTaskIDs())

	td.addTaskInfo("pending", "default") // default state = 0 = Pending
	td.addTaskInfo("running", "default")
	td.addTaskInfo("pausing", "default")
	td.addTaskInfo("paused", "default")
	td.addTaskInfo("done", "default")

	td.taskInfoMap["running"].state.Store(uint32(TaskStateRunning))
	td.taskInfoMap["pausing"].state.Store(uint32(TaskStatePausing))
	td.taskInfoMap["paused"].state.Store(uint32(TaskStatePaused))
	td.taskInfoMap["done"].state.Store(uint32(TaskStateDone))

	assert.Equal(t, []string{"pending"}, td.ListPendingTaskIDs())
	assert.Equal(t, []string{"running"}, td.ListRunningTaskIDs())
	assert.Equal(t, []string{"pausing"}, td.ListPausingTaskIDs())
	assert.Equal(t, []string{"paused"}, td.ListPausedTaskIDs())
}

// ---------------------------------------------------------------------------
// removeTaskIfPaused
// ---------------------------------------------------------------------------

func TestRemoveTaskIfPaused_NotPaused(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.addTaskInfo("x", "default")
	tt, pool := td.removeTaskIfPaused("x")
	assert.Nil(t, tt)
	assert.Empty(t, pool)
	assert.True(t, td.IsTaskExists("x")) // still exists
}

func TestRemoveTaskIfPaused_IsPaused(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	tk := task.New()
	tk.SetCfg(makeCfg("paused-task"))
	td.addTaskInfo("paused-task", "io-pool")
	td.setTaskToTaskInfo(tk)
	td.taskInfoMap["paused-task"].state.Store(uint32(TaskStatePaused))

	tt, pool := td.removeTaskIfPaused("paused-task")
	assert.Equal(t, tk, tt)
	assert.Equal(t, "io-pool", pool)
	assert.False(t, td.IsTaskExists("paused-task"))
}

func TestRemoveTaskIfPaused_NotExists(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	tt, pool := td.removeTaskIfPaused("ghost")
	assert.Nil(t, tt)
	assert.Empty(t, pool)
}

// ---------------------------------------------------------------------------
// restorePaused
// ---------------------------------------------------------------------------

func TestRestorePaused(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	tk := task.New()
	tk.SetCfg(makeCfg("restored"))
	td.restorePaused(tk, "some-pool")

	assert.True(t, td.IsTaskExists("restored"))
	state, err := td.GetTaskState("restored")
	require.NoError(t, err)
	assert.Equal(t, TaskStatePaused, state)

	e := td.taskInfoMap["restored"]
	assert.Equal(t, tk, e.task)
	assert.Equal(t, "some-pool", e.pool)
}

// ---------------------------------------------------------------------------
// Hook registration
// ---------------------------------------------------------------------------

func TestOnTaskCreate(t *testing.T) {
	td := &taskd{}
	h := func(tk *task.Task, err error, extra *HookExtraData) {}
	td.OnTaskCreate(h)
	assert.Len(t, td.createHooks, 1)
	td.OnTaskCreate(h, h)
	assert.Len(t, td.createHooks, 3)
}

func TestOnTaskInit(t *testing.T) {
	td := &taskd{}
	h := func(tk *task.Task, err error, extra *HookExtraData) {}
	td.OnTaskInit(h)
	assert.Len(t, td.initHooks, 1)
}

func TestOnTaskSubmit(t *testing.T) {
	td := &taskd{}
	h := func(tk *task.Task, err error, extra *HookExtraData) {}
	td.OnTaskSubmit(h)
	assert.Len(t, td.submitHooks, 1)
}

func TestOnTaskStart(t *testing.T) {
	td := &taskd{}
	h := func(tk *task.Task, err error, extra *HookExtraData) {}
	td.OnTaskStart(h)
	assert.Len(t, td.startHooks, 1)
}

func TestOnTaskPausing(t *testing.T) {
	td := &taskd{}
	h := func(tk *task.Task, err error, extra *HookExtraData) {}
	td.OnTaskPausing(h)
	assert.Len(t, td.pausingHooks, 1)
}

func TestOnTaskPaused(t *testing.T) {
	td := &taskd{}
	h := func(tk *task.Task, err error, extra *HookExtraData) {}
	td.OnTaskPaused(h)
	assert.Len(t, td.pausedHooks, 1)
}

func TestOnTaskDone(t *testing.T) {
	td := &taskd{}
	h := func(tk *task.Task, err error, extra *HookExtraData) {}
	td.OnTaskDone(h)
	assert.Len(t, td.doneHooks, 1)
}

// ---------------------------------------------------------------------------
// hookTask / hook
// ---------------------------------------------------------------------------

func TestHookTask_NilTask(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.l = slog.New(slog.DiscardHandler)
	called := false
	hooks := []Hook{func(tt *task.Task, err error, extra *HookExtraData) {
		assert.Nil(t, tt)
		assert.Nil(t, err)
		called = true
	}}
	td.hookTask(nil, nil, hooks, "test", nil)
	assert.True(t, called)
}

func TestHookTask_WithTask(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.l = slog.New(slog.DiscardHandler)
	tk := task.New()
	tk.SetCfg(makeCfg("hook-test"))
	called := false
	hooks := []Hook{func(tt *task.Task, err error, extra *HookExtraData) {
		assert.Equal(t, tk, tt)
		called = true
	}}
	td.hookTask(tk, nil, hooks, "test", nil)
	assert.True(t, called)
}

func TestHookTask_WithError(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.l = slog.New(slog.DiscardHandler)
	testErr := errors.New("test error")
	called := false
	hooks := []Hook{func(tt *task.Task, err error, extra *HookExtraData) {
		assert.Equal(t, testErr, err)
		called = true
	}}
	td.hookTask(nil, testErr, hooks, "test", nil)
	assert.True(t, called)
}

func TestHookTask_WithExtraData(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.l = slog.New(slog.DiscardHandler)
	extra := &HookExtraData{Wait: true}
	called := false
	hooks := []Hook{func(tt *task.Task, err error, extra *HookExtraData) {
		assert.True(t, extra.Wait)
		called = true
	}}
	td.hookTask(nil, nil, hooks, "test", extra)
	assert.True(t, called)
}

func TestHookTask_MultipleHooks(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.l = slog.New(slog.DiscardHandler)
	count := 0
	hooks := []Hook{
		func(tt *task.Task, err error, extra *HookExtraData) { count++ },
		func(tt *task.Task, err error, extra *HookExtraData) { count++ },
		func(tt *task.Task, err error, extra *HookExtraData) { count++ },
	}
	td.hookTask(nil, nil, hooks, "test", nil)
	assert.Equal(t, 3, count)
}

func TestHookTask_EmptyHooks(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.l = slog.New(slog.DiscardHandler)
	// Should not panic
	td.hookTask(nil, nil, nil, "test", nil)
	td.hookTask(nil, nil, []Hook{}, "test", nil)
}

func TestHook_PanicRecovery(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.l = slog.New(slog.DiscardHandler)
	called := false
	td.hook(nil, nil, func(tk *task.Task, err error, extra *HookExtraData) {
		called = true
		panic("boom")
	}, 0, "test", nil)
	assert.True(t, called)
	// Should not have panicked up the stack
}

func TestHook_PanicRecovery_WithTaskID(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.l = slog.New(slog.DiscardHandler)
	tk := task.New()
	// Use a cfg with an ID so hook's recover can log it
	cfg := makeCfg("panic-task")
	tk.SetCfg(cfg)
	called := false
	td.hook(tk, nil, func(tt *task.Task, err error, extra *HookExtraData) {
		called = true
		panic("kaboom")
	}, 1, "start", nil)
	assert.True(t, called)
}

func TestHook_PanicRecovery_NoLogger(t *testing.T) {
	// hook's recover handler calls td.l.Error which needs a logger.
	// Even with a discard handler, hooks should not propagate panics.
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.l = slog.New(slog.DiscardHandler)
	called := false
	td.hook(nil, nil, func(tk *task.Task, err error, extra *HookExtraData) {
		called = true
		panic("no-logger-panic")
	}, 0, "test", nil)
	assert.True(t, called)
}

// ---------------------------------------------------------------------------
// createTask
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// initTask
// ---------------------------------------------------------------------------

func TestInitTask_Success(t *testing.T) {
	td := &taskd{}
	// Register a step type so task init can create steps
	plugin.Reg(step.TypeCmd, func() step.Step { return step.NewCmdStep() }, func() any { return step.NewCmdStepCfg() })

	cfg := makeCfg("init-test")
	cfg.Add(step.TypeCmd, step.NewCmdStepCfg())
	tk := task.New()
	tk.SetCfg(cfg)

	err := td.initTask(context.Background(), tk)
	require.NoError(t, err)
}

func TestInitTask_EmptySteps(t *testing.T) {
	td := &taskd{}
	cfg := makeCfg("init-empty")
	tk := task.New()
	tk.SetCfg(cfg)

	// Even with no steps, Init should work (v.Struct validates Task struct, not Cfg)
	err := td.initTask(context.Background(), tk)
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// createInit
// ---------------------------------------------------------------------------

func TestCreateInit_InvalidCfg(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
	}
	// Empty cfg won't pass validation (ID required, Steps required)
	_, err := td.createInit(context.Background(), task.Cfg{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid task cfg")
}





// ---------------------------------------------------------------------------
// createInitSubmit
// ---------------------------------------------------------------------------

func makeInitializedTaskd(t *testing.T) *taskd {
	t.Helper()
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		pools:       make(map[string]pond.Pool),
	}
	td.l = slog.New(slog.DiscardHandler)
	td.cfg = &Cfg{Pools: []PoolCfg{
		{Name: "default", Size: 5, QueueSize: 10},
		{Name: "io", Size: 3, QueueSize: 50},
	}}
	err := td.Init(context.Background())
	require.NoError(t, err)
	td.ctx, td.cancel = context.WithCancel(context.Background())
	return td
}

// startAndStopTaskd calls runner.Start on td (which blocks on Stopping),
// then runner.StopAndWait to close the Stopping channel.
// This is needed so ErrStopping paths can be tested.
func startAndStopTaskd(t *testing.T, td *taskd) {
	t.Helper()
	td.l = slog.New(slog.DiscardHandler)
	ctx := logs.CtxWith(context.Background(), td.l)

	// Initialize runner.Base channels
	td.Base.Init(ctx)

	// Start in goroutine (blocked on Stopping)
	started := make(chan struct{})
	go func() {
		close(started)
		_ = runner.Start(ctx, td)
	}()
	<-started

	// Give it time to call markStarted
	time.Sleep(10 * time.Millisecond)

	// Stop to close the Stopping channel
	err := runner.StopAndWait(ctx, td)
	require.NoError(t, err)
}

func validTaskCfg(t *testing.T) task.Cfg {
	t.Helper()
	plugin.Reg(step.TypeCmd, func() step.Step { return step.NewCmdStep() }, func() any { return step.NewCmdStepCfg() })
	c := makeCfg("submit-test-" + t.Name())
	c.Add(step.TypeCmd, step.NewCmdStepCfg())
	return c
}

func TestCreateInitSubmit_ErrStopping(t *testing.T) {
	td := makeInitializedTaskd(t)
	startAndStopTaskd(t, td) // closes Stopping channel

	cfg := validTaskCfg(t)
	tt, err := td.createInitSubmit(context.Background(), "default", cfg, false)
	assert.Nil(t, tt)
	assert.ErrorIs(t, err, ErrStopping)
}

func TestCreateInitSubmit_ErrPoolNotExists_Empty(t *testing.T) {
	td := makeInitializedTaskd(t)
	cfg := validTaskCfg(t)
	tt, err := td.createInitSubmit(context.Background(), "", cfg, false)
	assert.Nil(t, tt)
	assert.ErrorIs(t, err, ErrPoolNotExists)
}

func TestCreateInitSubmit_ErrPoolNotExists_Invalid(t *testing.T) {
	td := makeInitializedTaskd(t)
	cfg := validTaskCfg(t)
	tt, err := td.createInitSubmit(context.Background(), "nonexistent-pool", cfg, false)
	assert.Nil(t, tt)
	assert.ErrorIs(t, err, ErrPoolNotExists)
}

func TestCreateInitSubmit_ErrTaskAlreadyExists(t *testing.T) {
	td := makeInitializedTaskd(t)
	cfg := validTaskCfg(t)

	// Manually add task info to simulate duplicate
	td.addTaskInfo(cfg.ID, "default")

	tt, err := td.createInitSubmit(context.Background(), "default", cfg, false)
	assert.Nil(t, tt)
	assert.ErrorIs(t, err, ErrTaskAlreadyExists)
}

func TestCreateInitSubmit_InvalidCfg(t *testing.T) {
	td := makeInitializedTaskd(t)
	// Use empty cfg which fails validation
	_, err := td.createInitSubmit(context.Background(), "default", task.Cfg{}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create init task failed")
}

func TestCreateInitSubmit_StoppingAfterAddTaskInfo(t *testing.T) {
	td := makeInitializedTaskd(t)

	// Test that task info is cleaned up when createInit fails.
	// An invalid cfg causes createInit to fail after addTaskInfo.
	badCfg := task.Cfg{} // invalid cfg
	_, err := td.createInitSubmit(context.Background(), "default", badCfg, false)
	require.Error(t, err)
	// Task info should have been removed
	assert.False(t, td.IsTaskExists(badCfg.ID))
}

// ---------------------------------------------------------------------------
// SubmitTask / SubmitTaskAndWait
// ---------------------------------------------------------------------------

func TestSubmitTask_ErrStopping(t *testing.T) {
	td := makeInitializedTaskd(t)
	startAndStopTaskd(t, td)

	cfg := validTaskCfg(t)
	tt, err := td.SubmitTask(context.Background(), "default", cfg)
	assert.Nil(t, tt)
	assert.ErrorIs(t, err, ErrStopping)
}

func TestSubmitTaskAndWait_ErrStopping(t *testing.T) {
	td := makeInitializedTaskd(t)
	startAndStopTaskd(t, td)

	cfg := validTaskCfg(t)
	tt, err := td.SubmitTaskAndWait(context.Background(), "default", cfg)
	assert.Nil(t, tt)
	assert.ErrorIs(t, err, ErrStopping)
}

// ---------------------------------------------------------------------------
// StopTask
// ---------------------------------------------------------------------------

func TestStopTask_ErrStopping(t *testing.T) {
	td := makeInitializedTaskd(t)
	startAndStopTaskd(t, td)

	err := td.StopTask(context.Background(), "any")
	assert.ErrorIs(t, err, ErrStopping)
}

func TestStopTask_RemovePaused(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		l:           slog.New(slog.DiscardHandler),
	}
	tk := task.New()
	tk.SetCfg(makeCfg("paused-stop"))
	td.addTaskInfo("paused-stop", "default")
	td.setTaskToTaskInfo(tk)
	td.taskInfoMap["paused-stop"].state.Store(uint32(TaskStatePaused))

	err := td.StopTask(context.Background(), "paused-stop")
	require.NoError(t, err)
	assert.False(t, td.IsTaskExists("paused-stop"))
}

func TestStopTask_ErrTaskNotExists(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		l:           slog.New(slog.DiscardHandler),
	}
	err := td.StopTask(context.Background(), "ghost")
	assert.ErrorIs(t, err, ErrTaskNotExists)
}

// TestStopTask_ErrTaskAlreadyStopping is hard to test without integration:
// the task needs its Stopping channel closed but Started not closed.
// This requires runner.Start+runner.Stop on the task, which needs
// logs.FromCtx in the context.
// TestPauseTask_ErrTaskAlreadyStopping: requires runner.Start+Stop on task.

// ---------------------------------------------------------------------------
// StopTask - success path (not stopping, task exists, task not stopping)
// ---------------------------------------------------------------------------

func TestStopTask_Success(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		l:           slog.New(slog.DiscardHandler),
	}

	tk := task.New()
	tk.SetCfg(makeCfg("stop-me"))

	// Start the task so Started channel is closed, but don't stop it
	ctx := logs.CtxWith(context.Background(), slog.New(slog.DiscardHandler))
	tk.Base.Init(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = runner.Start(ctx, tk)
	}()

	// Wait for markStarted
	time.Sleep(10 * time.Millisecond)
	<-tk.Started()

	// Add to taskd
	td.addTaskInfo("stop-me", "default")
	td.setTaskToTaskInfo(tk)

	// Stop the task - this calls runner.Stop which closes Stopping channel
	err := td.StopTask(context.Background(), "stop-me")
	require.NoError(t, err)

	// Wait for the Start goroutine to finish
	<-done
}

// ---------------------------------------------------------------------------
// PauseTask
// ---------------------------------------------------------------------------

func TestPauseTask_ErrStopping(t *testing.T) {
	td := makeInitializedTaskd(t)
	startAndStopTaskd(t, td)

	err := td.PauseTask(context.Background(), "any")
	assert.ErrorIs(t, err, ErrStopping)
}

func TestPauseTask_ErrTaskNotExists(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		l:           slog.New(slog.DiscardHandler),
	}
	err := td.PauseTask(context.Background(), "ghost")
	assert.ErrorIs(t, err, ErrTaskNotExists)
}

func TestPauseTask_ErrTaskNotStarted(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		l:           slog.New(slog.DiscardHandler),
	}
	tk := task.New()
	tk.SetCfg(makeCfg("not-started"))
	td.addTaskInfo("not-started", "default")
	td.setTaskToTaskInfo(tk)
	// Task is in map but not started (Started channel not closed)

	err := td.PauseTask(context.Background(), "not-started")
	assert.ErrorIs(t, err, ErrTaskNotStarted)
}

// TestPauseTask_ErrTaskAlreadyStopping: requires runner.Start+Stop on task.

// ---------------------------------------------------------------------------
// ResumeTask
// ---------------------------------------------------------------------------

func TestResumeTask_ErrStopping(t *testing.T) {
	td := makeInitializedTaskd(t)
	startAndStopTaskd(t, td)

	tt, err := td.ResumeTask(context.Background(), "any")
	assert.Nil(t, tt)
	assert.ErrorIs(t, err, ErrStopping)
}

func TestResumeTask_ErrTaskNotPaused(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		l:           slog.New(slog.DiscardHandler),
	}
	// No task info at all
	tt, err := td.ResumeTask(context.Background(), "ghost")
	assert.Nil(t, tt)
	assert.ErrorIs(t, err, ErrTaskNotPaused)

	// Task exists but not in Paused state
	td.addTaskInfo("running-task", "default")
	tt, err = td.ResumeTask(context.Background(), "running-task")
	assert.Nil(t, tt)
	assert.ErrorIs(t, err, ErrTaskNotPaused)
}

// ---------------------------------------------------------------------------
// waitAllTaskDone
// ---------------------------------------------------------------------------

func TestWaitAllTaskDone_Empty(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	// No tasks - should return immediately
	td.waitAllTaskDone()
}

func TestWaitAllTaskDone_WithCompletedTasks(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}

	// Create tasks, start and stop them properly so Done channels are closed
	tk1 := task.New()
	tk1.SetCfg(makeCfg("done1"))

	tk2 := task.New()
	tk2.SetCfg(makeCfg("done2"))

	td.addTaskInfo("done1", "default")
	td.addTaskInfo("done2", "default")
	td.setTaskToTaskInfo(tk1)
	td.setTaskToTaskInfo(tk2)

	// Set state to something other than paused so ListTasks includes them
	td.taskInfoMap["done1"].state.Store(uint32(TaskStateRunning))
	td.taskInfoMap["done2"].state.Store(uint32(TaskStateRunning))

	// Use runner.Start + runner.Stop to properly close Done channels
	ctx := logs.CtxWith(context.Background(), slog.New(slog.DiscardHandler))

	for _, tk := range []*task.Task{tk1, tk2} {
		tk.Base.Init(ctx)
		go func(t *task.Task) {
			_ = runner.Start(ctx, t)
		}(tk)
		time.Sleep(10 * time.Millisecond) // ensure markStarted is called
		_ = runner.StopAndWait(ctx, tk)
	}

	// Now waitAllTaskDone should return immediately
	td.waitAllTaskDone()
}

// ---------------------------------------------------------------------------
// DaemonType constant
// ---------------------------------------------------------------------------

func TestDaemonTypeTaskd(t *testing.T) {
	assert.Equal(t, boot.DaemonType("taskd"), DaemonTypeTaskd)
}

// ---------------------------------------------------------------------------
// Taskd interface compliance
// ---------------------------------------------------------------------------

func TestTaskdImplementsInterface(t *testing.T) {
	var td Taskd = (*taskd)(nil)
	assert.Nil(t, td)
}

// ---------------------------------------------------------------------------
// HookExtraData
// ---------------------------------------------------------------------------

func TestHookExtraData(t *testing.T) {
	extra := &HookExtraData{Wait: true}
	assert.True(t, extra.Wait)

	extra2 := &HookExtraData{Wait: false}
	assert.False(t, extra2.Wait)
}

// ---------------------------------------------------------------------------
// listIDsByState with empty map
// ---------------------------------------------------------------------------

func TestListIDsByState_Empty(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	assert.Empty(t, td.listIDsByState(TaskStatePending))
	assert.Empty(t, td.listIDsByState(TaskStateRunning))
	assert.Empty(t, td.listIDsByState(TaskStatePausing))
	assert.Empty(t, td.listIDsByState(TaskStatePaused))
	assert.Empty(t, td.listIDsByState(TaskStateDone))
}

// ---------------------------------------------------------------------------
// listIDsByState with multiple entries
// ---------------------------------------------------------------------------

func TestListIDsByState_Mixed(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.addTaskInfo("a", "p")
	td.addTaskInfo("b", "p")
	td.addTaskInfo("c", "p")
	td.addTaskInfo("d", "p")

	td.taskInfoMap["a"].state.Store(uint32(TaskStatePending))
	td.taskInfoMap["b"].state.Store(uint32(TaskStateRunning))
	td.taskInfoMap["c"].state.Store(uint32(TaskStatePaused))
	td.taskInfoMap["d"].state.Store(uint32(TaskStatePaused))

	pendingIDs := td.listIDsByState(TaskStatePending)
	assert.ElementsMatch(t, []string{"a"}, pendingIDs)

	runningIDs := td.listIDsByState(TaskStateRunning)
	assert.ElementsMatch(t, []string{"b"}, runningIDs)

	pausedIDs := td.listIDsByState(TaskStatePaused)
	assert.ElementsMatch(t, []string{"c", "d"}, pausedIDs)
}

// ---------------------------------------------------------------------------
// setTaskToTaskInfo edge cases
// ---------------------------------------------------------------------------

func TestSetTaskToTaskInfo_IdNotInMap(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	tk := task.New()
	tk.SetCfg(makeCfg("orphan"))

	// addTaskInfo for a different ID
	td.addTaskInfo("other", "default")
	// setTaskToTaskInfo for "orphan" - not in map, should be no-op
	td.setTaskToTaskInfo(tk)
	assert.Nil(t, td.taskInfoMap["orphan"])
}

// ---------------------------------------------------------------------------
// removeTaskIfPaused - competing state
// ---------------------------------------------------------------------------

func TestRemoveTaskIfPaused_RunningState(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.addTaskInfo("running", "default")
	td.taskInfoMap["running"].state.Store(uint32(TaskStateRunning))
	tt, pool := td.removeTaskIfPaused("running")
	assert.Nil(t, tt)
	assert.Empty(t, pool)
	assert.True(t, td.IsTaskExists("running"))
}

func TestRemoveTaskIfPaused_PendingState(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.addTaskInfo("pending", "default")
	// Default state is 0 = Pending
	tt, pool := td.removeTaskIfPaused("pending")
	assert.Nil(t, tt)
	assert.Empty(t, pool)
	assert.True(t, td.IsTaskExists("pending"))
}

// ---------------------------------------------------------------------------
// changeTaskState - consecutive state transitions
// ---------------------------------------------------------------------------

func TestChangeTaskState_ToPausing(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.addTaskInfo("t", "p")
	td.taskInfoMap["t"].state.Store(uint32(TaskStateRunning))
	ok := td.changeTaskState("t", TaskStateRunning, TaskStatePausing)
	assert.True(t, ok)

	state, _ := td.GetTaskState("t")
	assert.Equal(t, TaskStatePausing, state)
}

func TestChangeTaskState_ToPaused(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}
	td.addTaskInfo("t", "p")
	td.taskInfoMap["t"].state.Store(uint32(TaskStatePausing))
	ok := td.changeTaskState("t", TaskStatePausing, TaskStatePaused)
	assert.True(t, ok)

	state, _ := td.GetTaskState("t")
	assert.Equal(t, TaskStatePaused, state)
}

// ---------------------------------------------------------------------------
// GetTaskState - various states
// ---------------------------------------------------------------------------

func TestGetTaskState_AllStates(t *testing.T) {
	td := &taskd{taskInfoMap: make(map[string]*taskInfo)}

	states := []TaskState{TaskStatePending, TaskStateRunning, TaskStateDone, TaskStatePausing, TaskStatePaused}
	for i, s := range states {
		id := string(rune('a' + i))
		td.addTaskInfo(id, "default")
		td.taskInfoMap[id].state.Store(uint32(s))
		got, err := td.GetTaskState(id)
		require.NoError(t, err)
		assert.Equal(t, s, got)
	}
}

// ---------------------------------------------------------------------------
// GetTaskCfg with nil task (would panic if reachable, so no direct test)
// ---------------------------------------------------------------------------

func TestGetTaskCfg_ExistsButNoTaskSet(t *testing.T) {
	// This test documents that GetTaskCfg panics if task field is nil.
	// Since we can't test nil dereference without causing panic, we skip.
}

// ---------------------------------------------------------------------------
// initTask panic recovery
// ---------------------------------------------------------------------------

func TestInitTask_Panic(t *testing.T) {
	td := &taskd{}
	// Passing nil *task.Task does not trigger runner.Init's nil check
	// because a typed nil pointer satisfies the Runner interface.
	// The panic recovery in initTask is a safety net that's hard to trigger
	// through the normal initTask API (typed parameter).
	// runner.Init would only panic with an untyped nil interface, which
	// initTask's call sites never produce.
	err := td.initTask(context.Background(), nil)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// StopTask - ErrTaskAlreadyStopping
// ---------------------------------------------------------------------------

func TestStopTask_ErrTaskAlreadyStopping(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		l:           slog.New(slog.DiscardHandler),
	}

	// Create a task with no steps so Start returns immediately
	tk := task.New()
	tk.SetCfg(makeCfg("stopping-test"))

	// Start and stop the task so its Stopping channel is closed
	ctx := logs.CtxWith(context.Background(), slog.New(slog.DiscardHandler))
	tk.Base.Init(ctx)
	go func() { _ = runner.Start(ctx, tk) }()
	time.Sleep(10 * time.Millisecond)
	_ = runner.Stop(ctx, tk)

	// Add to taskd
	td.addTaskInfo("stopping-test", "default")
	td.setTaskToTaskInfo(tk)

	err := td.StopTask(context.Background(), "stopping-test")
	assert.ErrorIs(t, err, ErrTaskAlreadyStopping)
}

// ---------------------------------------------------------------------------
// PauseTask - ErrTaskAlreadyStopping
// ---------------------------------------------------------------------------

func TestPauseTask_ErrTaskAlreadyStopping(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		l:           slog.New(slog.DiscardHandler),
	}

	tk := task.New()
	tk.SetCfg(makeCfg("pausing-stop-test"))

	ctx := logs.CtxWith(context.Background(), slog.New(slog.DiscardHandler))
	tk.Base.Init(ctx)
	go func() { _ = runner.Start(ctx, tk) }()
	time.Sleep(10 * time.Millisecond)
	_ = runner.Stop(ctx, tk)

	td.addTaskInfo("pausing-stop-test", "default")
	td.setTaskToTaskInfo(tk)

	err := td.PauseTask(context.Background(), "pausing-stop-test")
	assert.ErrorIs(t, err, ErrTaskAlreadyStopping)
}

// ---------------------------------------------------------------------------
// PauseTask - success path (pausing hooks + changeTaskState + runner.Stop)
// ---------------------------------------------------------------------------

func TestPauseTask_Success(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		l:           slog.New(slog.DiscardHandler),
	}

	tk := task.New()
	tk.SetCfg(makeCfg("pause-me"))

	// Start the task so Started channel is closed, but don't stop it
	ctx := logs.CtxWith(context.Background(), slog.New(slog.DiscardHandler))
	tk.Base.Init(ctx)

	var startErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		startErr = runner.Start(ctx, tk)
	}()

	// Wait for markStarted
	time.Sleep(10 * time.Millisecond)
	<-tk.Started() // confirm started

	// Add to taskd and set state to Running
	td.addTaskInfo("pause-me", "default")
	td.setTaskToTaskInfo(tk)
	td.taskInfoMap["pause-me"].state.Store(uint32(TaskStateRunning))

	// Register a pausing hook
	pausingCalled := false
	td.OnTaskPausing(func(tt *task.Task, err error, extra *HookExtraData) {
		assert.Nil(t, err)
		pausingCalled = true
	})

	// Pause the task
	err := td.PauseTask(context.Background(), "pause-me")
	require.NoError(t, err)
	assert.True(t, pausingCalled)

	// State should be Pausing (runner.Stop called but submit goroutine hasn't
	// transitioned to Paused yet - that happens in the submit function)
	state, _ := td.GetTaskState("pause-me")
	assert.Equal(t, TaskStatePausing, state)

	// Wait for the Start goroutine to finish
	<-done
	_ = startErr
}

func TestPauseTask_AlreadyPausing(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		l:           slog.New(slog.DiscardHandler),
	}

	tk := task.New()
	tk.SetCfg(makeCfg("was-pausing"))

	// Start the task
	ctx := logs.CtxWith(context.Background(), slog.New(slog.DiscardHandler))
	tk.Base.Init(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = runner.Start(ctx, tk)
	}()

	time.Sleep(10 * time.Millisecond)
	<-tk.Started()

	// Add to taskd with state already Pausing (not Running)
	td.addTaskInfo("was-pausing", "default")
	td.setTaskToTaskInfo(tk)
	td.taskInfoMap["was-pausing"].state.Store(uint32(TaskStatePausing))

	// PauseTask should fail because state is not Running
	err := td.PauseTask(context.Background(), "was-pausing")
	assert.ErrorIs(t, err, ErrTaskAlreadyPausing)

	// Wait for goroutine
	<-done
}

// ---------------------------------------------------------------------------
// ResumeTask - resume fails, restorePaused called
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// createTask — success path (bug fixed: &cfg → cfg)
// ---------------------------------------------------------------------------

func TestCreateTask_Success(t *testing.T) {
	td := &taskd{}
	cfg := makeCfg("create-success")
	cfg.Add(step.TypeCmd, step.NewCmdStepCfg())
	tt, err := td.createTask(cfg)
	require.NoError(t, err)
	require.NotNil(t, tt)
	assert.Equal(t, "create-success", tt.Cfg().ID)
}

// ---------------------------------------------------------------------------
// createInit — success path
// ---------------------------------------------------------------------------

func TestCreateInit_Success(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
	}
	plugin.Reg(step.TypeCmd, func() step.Step { return step.NewCmdStep() }, func() any { return step.NewCmdStepCfg() })

	cfg := makeCfg("ci-success")
	cfg.Add(step.TypeCmd, step.NewCmdStepCfg())
	tt, err := td.createInit(context.Background(), cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, tt)
}

func TestCreateInit_InitTaskFails(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		initHooks:   []Hook{},
	}
	// No steps registered → init fails on validation
	cfg := makeCfg("ci-init-fail")
	// Empty steps — validation requires Steps to have at least 1
	_, err := td.createInit(context.Background(), cfg, nil)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// createInitSubmit — success path
// ---------------------------------------------------------------------------

func TestCreateInitSubmit_Success(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		pools:       make(map[string]pond.Pool),
		l:           slog.New(slog.DiscardHandler),
	}
	td.cfg = &Cfg{Pools: []PoolCfg{{Name: "default", Size: 5, QueueSize: 10}}}
	td.Init(context.Background())
	td.ctx, td.cancel = context.WithCancel(context.Background())

	cfg := makeCfg("cis-success")
	cfg.Add(step.TypeCmd, step.NewCmdStepCfg())

	tt, err := td.createInitSubmit(context.Background(), "default", cfg, false)
	require.NoError(t, err)
	require.NotNil(t, tt)
}

// ---------------------------------------------------------------------------
// ResumeTask — success path
// ---------------------------------------------------------------------------

func TestResumeTask_Success(t *testing.T) {
	td := &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		pools:       make(map[string]pond.Pool),
		l:           slog.New(slog.DiscardHandler),
	}
	td.cfg = &Cfg{Pools: []PoolCfg{{Name: "default", Size: 5, QueueSize: 10}}}
	td.Init(context.Background())
	td.ctx, td.cancel = context.WithCancel(context.Background())

	tk := task.New()
	oldCfg := makeCfg("resume-success")
	oldCfg.Add(step.TypeCmd, step.NewCmdStepCfg())
	tk.SetCfg(oldCfg)
	tk.Store("test-key", "test-val")

	// Add as paused
	td.restorePaused(tk, "default")
	assert.True(t, td.IsTaskExists("resume-success"))

	// Resume should succeed
	newT, err := td.ResumeTask(context.Background(), "resume-success")
	require.NoError(t, err)
	require.NotNil(t, newT)

	// Old task data should be copied
	val, ok := newT.Load("test-key")
	assert.True(t, ok)
	assert.Equal(t, "test-val", val)
}
