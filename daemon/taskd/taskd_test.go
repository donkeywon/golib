package taskd

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/donkeywon/golib/kvs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/task"
	"github.com/donkeywon/golib/task/step"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// ---- mock steps ----

const (
	quickStepType step.Type = "quickstep"
	blockStepType step.Type = "blockstep"
)

type quickStepCfg struct{}

type quickStep struct {
	kvs.Map[string, any]
}

func (q *quickStep) SetCfg(cfg any) {}

func (q *quickStep) Run(ctx context.Context) error {
	return nil
}

type blockStepCfg struct {
	Block chan struct{}
}

type blockStep struct {
	kvs.Map[string, any]
	cfg blockStepCfg
}

func (b *blockStep) SetCfg(cfg any) {
	b.cfg = cfg.(blockStepCfg)
}

func (b *blockStep) Run(ctx context.Context) error {
	if b.cfg.Block != nil {
		select {
		case <-b.cfg.Block:
		case <-ctx.Done():
		}
	}
	return nil
}

func TestMain(m *testing.M) {
	plugin.Reg(quickStepType, func() step.Step { return &quickStep{} }, func() any { return quickStepCfg{} })
	plugin.Reg(blockStepType, func() step.Step { return &blockStep{} }, func() any { return blockStepCfg{} })
	os.Exit(m.Run())
}

// ---- helpers ----

func testCfg() Cfg {
	return Cfg{
		Size:            2,
		QueueSize:       8,
		ShutdownTimeout: time.Second,
	}
}

// newTestTaskd 构造已就绪的 taskd(ready=true, ctx 未取消, pool 可用)
func newTestTaskd(t *testing.T) *taskd {
	t.Helper()
	l := zerolog.Nop()
	td := &taskd{
		cfg: testCfg(),
		l:   &l,
		ctx: context.Background(),
	}
	td.ready.Store(true)
	td.pool = pond.NewPool(td.cfg.Size, pond.WithQueueSize(td.cfg.QueueSize))
	return td
}

func newQuickTask(t *testing.T, id string) *task.Task {
	t.Helper()
	cfg := task.NewCfg().SetID(id).Add(quickStepType, quickStepCfg{})
	return plugin.CreateWithCfg[*task.Task](task.PluginTypeTask, cfg)
}

func newBlockTask(t *testing.T, id string, block chan struct{}) *task.Task {
	t.Helper()
	cfg := task.NewCfg().SetID(id).Add(blockStepType, blockStepCfg{Block: block})
	return plugin.CreateWithCfg[*task.Task](task.PluginTypeTask, cfg)
}

func newTaskInfo(t *testing.T, tk *task.Task) *taskInfo {
	t.Helper()
	return &taskInfo{
		task:   tk,
		ctx:    context.Background(),
		cancel: func() {},
		done:   make(chan struct{}),
	}
}

func waitTaskRemoved(t *testing.T, td *taskd, id string) {
	t.Helper()
	require.Eventually(t, func() bool { return !td.IsTaskExists(id) }, 2*time.Second, 5*time.Millisecond)
}

// ---- Cfg ----

func TestNewCfg(t *testing.T) {
	c := NewCfg()
	require.Equal(t, DefaultPoolSize, c.Size)
	require.Equal(t, DefaultQueueSize, c.QueueSize)
	require.Equal(t, DefaultShutdownTimeout, c.ShutdownTimeout)
}

func TestNewAndSetCfg(t *testing.T) {
	td := New().(*taskd)
	require.NotNil(t, td)

	td.SetCfg(testCfg())
	require.Equal(t, testCfg(), td.cfg)
}

func TestTaskStateString(t *testing.T) {
	require.Equal(t, "pending", TaskStatePending.String())
	require.Equal(t, "running", TaskStateRunning.String())
	require.Equal(t, "stopping", TaskStateStopping.String())
	require.Equal(t, "unknown", TaskState(99).String())
}

// ---- ready / stopping 守卫 ----

func TestSubmitTask_NotReady(t *testing.T) {
	td := &taskd{}
	_, err := td.SubmitTask(context.Background(), newQuickTask(t, "t1").Cfg())
	require.ErrorIs(t, err, ErrNotReady)
}

func TestStopTask_NotReady(t *testing.T) {
	td := &taskd{}
	err := td.StopTask(context.Background(), "t1")
	require.ErrorIs(t, err, ErrNotReady)
}

func TestSubmitTask_Stopping(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	td := newTestTaskd(t)
	td.ctx = ctx

	_, err := td.SubmitTask(context.Background(), newQuickTask(t, "t1").Cfg())
	require.ErrorIs(t, err, ErrStopping)
}

func TestStopTask_Stopping(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	td := newTestTaskd(t)
	td.ctx = ctx

	err := td.StopTask(context.Background(), "t1")
	require.ErrorIs(t, err, ErrStopping)
}

// ---- createAndSubmit ----

func TestSubmitTask_InvalidCfg(t *testing.T) {
	td := newTestTaskd(t)
	_, err := td.SubmitTask(context.Background(), task.NewCfg())
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid task cfg")
}

func TestSubmitTask_Duplicate(t *testing.T) {
	td := newTestTaskd(t)
	block := make(chan struct{})
	defer close(block)

	_, err := td.SubmitTask(context.Background(), newBlockTask(t, "dup", block).Cfg())
	require.NoError(t, err)

	_, err = td.SubmitTask(context.Background(), newBlockTask(t, "dup", nil).Cfg())
	require.ErrorIs(t, err, ErrTaskAlreadyExists)
}

func TestSubmitTask_CtxCanceledAfterCreate(t *testing.T) {
	td := newTestTaskd(t)
	ctx, cancel := context.WithCancel(context.Background())
	td.ctx = ctx
	td.OnTaskCreate(func(ctx context.Context, t *task.Task, err error, extra *HookExtraData) {
		cancel()
	})

	_, err := td.SubmitTask(context.Background(), newQuickTask(t, "c1").Cfg())
	require.ErrorIs(t, err, ErrStopping)
	require.False(t, td.IsTaskExists("c1"), "中途取消应清理 taskInfo")
}

func TestSubmitTask_PoolStopped(t *testing.T) {
	td := newTestTaskd(t)
	td.pool.Stop().Wait() // Stop 是异步的,等 closed 标记生效

	_, err := td.SubmitTask(context.Background(), newQuickTask(t, "t1").Cfg())
	require.ErrorIs(t, err, ErrStopping)
	require.ErrorContains(t, err, "submit failed")
	require.False(t, td.IsTaskExists("t1"), "提交失败应清理 taskInfo")
}

func TestSubmitTask_Success(t *testing.T) {
	td := newTestTaskd(t)
	var runHook, doneHook, submitHook bool
	td.OnTaskRun(func(ctx context.Context, t *task.Task, err error, extra *HookExtraData) { runHook = true })
	td.OnTaskDone(func(ctx context.Context, t *task.Task, err error, extra *HookExtraData) { doneHook = true })
	td.OnTaskSubmit(func(ctx context.Context, t *task.Task, err error, extra *HookExtraData) { submitHook = true })

	_, err := td.SubmitTask(context.Background(), newQuickTask(t, "ok1").Cfg())
	require.NoError(t, err)
	waitTaskRemoved(t, td, "ok1")
	require.True(t, runHook)
	require.True(t, doneHook)
	require.True(t, submitHook)
}

func TestSubmitTaskAndWait_Success(t *testing.T) {
	td := newTestTaskd(t)

	_, err := td.SubmitTaskAndWait(context.Background(), newQuickTask(t, "w1").Cfg())
	require.NoError(t, err)
	require.False(t, td.IsTaskExists("w1"), "同步提交完成后 taskInfo 应已清理")
}

func TestSubmitTaskAndWait_CallerCtxCanceled(t *testing.T) {
	td := newTestTaskd(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := td.SubmitTaskAndWait(ctx, newQuickTask(t, "w2").Cfg())
	require.NoError(t, err)
	require.False(t, td.IsTaskExists("w2"))
}

// ---- submit(white-box) ----

func TestSubmit_ChangeStateFailed(t *testing.T) {
	td := newTestTaskd(t)
	ti := newTaskInfo(t, newQuickTask(t, "t1"))
	ti.state = TaskStateStopping

	err := td.submit(ti, false)
	require.ErrorContains(t, err, "task state changed before submit")
}

func TestSubmit_PoolStopped(t *testing.T) {
	td := newTestTaskd(t)
	td.pool.Stop().Wait() // Stop 是异步的,等 closed 标记生效

	ti := newTaskInfo(t, newQuickTask(t, "t1"))
	err := td.submit(ti, false)
	require.ErrorIs(t, err, ErrStopping)
}

func TestSubmit_CtxDone(t *testing.T) {
	td := newTestTaskd(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	td.ctx = ctx

	// 占住 worker,让 f 排队,pt 不完成
	blocker := make(chan struct{})
	td.pool.Submit(func() { <-blocker })

	ti := newTaskInfo(t, newQuickTask(t, "t1"))
	err := td.submit(ti, false)
	require.ErrorIs(t, err, ErrStopping)
	close(blocker)
}

func TestSubmit_TaskRunFailedBeforeStart(t *testing.T) {
	// f 执行时状态已非 Pending(StopTask 抢占)→ changeState 失败分支
	td := newTestTaskd(t)
	td.pool = pond.NewPool(1, pond.WithQueueSize(td.cfg.QueueSize))

	blocker := make(chan struct{})
	td.pool.Submit(func() { <-blocker }) // 占住唯一 worker

	tk := newQuickTask(t, "t1")
	ti := newTaskInfo(t, tk)
	td.addTaskInfo("t1", tk)
	require.NoError(t, td.submit(ti, false)) // Unknown→Pending,f 排队

	require.NoError(t, td.StopTask(context.Background(), "t1")) // Pending→Stopping

	close(blocker) // f 执行 → changeState(Pending→Running) 失败 → 日志返回
	select {
	case <-ti.done:
	case <-time.After(2 * time.Second):
		t.Fatal("f not finished")
	}
	require.False(t, td.IsTaskExists("t1"))
}

func TestSubmitTask_CreateTaskPanic(t *testing.T) {
	plugin.Reg(task.PluginTypeTask, func() *task.Task { panic("creator boom") }, func() any { return task.NewCfg() })
	defer plugin.Reg(task.PluginTypeTask, task.New, func() any { return task.NewCfg() })

	td := newTestTaskd(t)
	// 不能调 newQuickTask(内部用 plugin 创建,会直接 panic),直接构造 cfg
	cfg := task.NewCfg().SetID("t1").Add(quickStepType, quickStepCfg{})
	_, err := td.SubmitTask(context.Background(), cfg)
	require.Error(t, err)
	require.ErrorContains(t, err, "create task failed")
}

// ---- StopTask 状态机 ----

func TestStopTask_NotExists(t *testing.T) {
	td := newTestTaskd(t)
	err := td.StopTask(context.Background(), "nope")
	require.ErrorIs(t, err, ErrTaskNotExists)
}

func TestStopTask_BeforeSubmit(t *testing.T) {
	td := newTestTaskd(t)
	td.addTaskInfo("pre1", newQuickTask(t, "pre1")) // state=Unknown,未提交

	require.NoError(t, td.StopTask(context.Background(), "pre1"))
	require.False(t, td.IsTaskExists("pre1"))
}

func TestStopTask_Pending(t *testing.T) {
	td := newTestTaskd(t)
	td.pool = pond.NewPool(1, pond.WithQueueSize(td.cfg.QueueSize))

	blocker := make(chan struct{})
	td.pool.Submit(func() { <-blocker }) // 占住 worker

	block := make(chan struct{})
	defer close(block)
	_, err := td.SubmitTask(context.Background(), newBlockTask(t, "pend1", block).Cfg())
	require.NoError(t, err)
	require.Equal(t, TaskStatePending, td.getTaskInfo("pend1").getState())

	require.NoError(t, td.StopTask(context.Background(), "pend1"))
	require.False(t, td.IsTaskExists("pend1"))

	close(blocker) // f 之后执行 → changeState 失败 → 日志
}

func TestStopTask_Running(t *testing.T) {
	td := newTestTaskd(t)
	block := make(chan struct{})
	defer close(block)

	_, err := td.SubmitTask(context.Background(), newBlockTask(t, "run1", block).Cfg())
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return td.getTaskInfo("run1") != nil && td.getTaskInfo("run1").getState() == TaskStateRunning
	}, 2*time.Second, 5*time.Millisecond)

	require.NoError(t, td.StopTask(context.Background(), "run1"))
	waitTaskRemoved(t, td, "run1")
}

func TestStopTask_AlreadyStopping(t *testing.T) {
	td := newTestTaskd(t)
	td.addTaskInfo("s1", newQuickTask(t, "s1"))
	td.getTaskInfo("s1").state = TaskStateStopping

	err := td.StopTask(context.Background(), "s1")
	require.ErrorIs(t, err, ErrTaskAlreadyStopping)
}

func TestStopTask_InvalidState(t *testing.T) {
	td := newTestTaskd(t)
	td.addTaskInfo("bad1", newQuickTask(t, "bad1"))
	td.getTaskInfo("bad1").state = TaskState(99)

	err := td.StopTask(context.Background(), "bad1")
	require.ErrorContains(t, err, "task not running")
}

// ---- 查询 ----

func TestIsTaskExists(t *testing.T) {
	td := newTestTaskd(t)
	require.False(t, td.IsTaskExists("nope"))
	td.addTaskInfo("t1", newQuickTask(t, "t1"))
	require.True(t, td.IsTaskExists("t1"))
}

func TestGetTaskState(t *testing.T) {
	td := newTestTaskd(t)

	state, err := td.GetTaskState("nope")
	require.ErrorIs(t, err, ErrTaskNotExists)
	require.Equal(t, TaskStatePending, state)

	td.addTaskInfo("t1", newQuickTask(t, "t1"))
	state, err = td.GetTaskState("t1")
	require.NoError(t, err)
	require.Equal(t, TaskStateUnknown, state)
}

func TestGetTaskCfg(t *testing.T) {
	td := newTestTaskd(t)

	_, err := td.GetTaskCfg("nope")
	require.ErrorIs(t, err, ErrTaskNotExists)

	td.addTaskInfo("t1", newQuickTask(t, "t1"))
	c, err := td.GetTaskCfg("t1")
	require.NoError(t, err)
	require.Equal(t, "t1", c.ID)
}

func TestListTasks(t *testing.T) {
	td := newTestTaskd(t)
	td.addTaskInfo("p1", newQuickTask(t, "p1"))
	td.addTaskInfo("r1", newQuickTask(t, "r1"))
	td.addTaskInfo("u1", newQuickTask(t, "u1"))
	td.getTaskInfo("p1").state = TaskStatePending
	td.getTaskInfo("r1").state = TaskStateRunning

	require.ElementsMatch(t, []string{"p1", "r1", "u1"}, td.ListTaskIDs())
	require.Equal(t, []string{"p1"}, td.ListPendingTaskIDs())
	require.Equal(t, []string{"r1"}, td.ListRunningTaskIDs())

	empty := newTestTaskd(t)
	require.Empty(t, empty.ListTaskIDs())
	require.Empty(t, empty.ListPendingTaskIDs())
	require.Empty(t, empty.ListRunningTaskIDs())
}

// ---- hooks ----

func TestOnTaskHooks(t *testing.T) {
	td := newTestTaskd(t)
	h := func(context.Context, *task.Task, error, *HookExtraData) {}

	td.OnTaskCreate(h)
	td.OnTaskSubmit(h)
	td.OnTaskRun(h)
	td.OnTaskDone(h)

	require.Len(t, td.createHooks, 1)
	require.Len(t, td.submitHooks, 1)
	require.Len(t, td.runHooks, 1)
	require.Len(t, td.doneHooks, 1)

	td.OnTaskRun(h, h)
	require.Len(t, td.runHooks, 3)
}

func TestHookTask_Panic(t *testing.T) {
	td := newTestTaskd(t)
	td.OnTaskDone(func(ctx context.Context, t *task.Task, err error, extra *HookExtraData) {
		panic("hook boom")
	})

	// t 非 nil:覆盖 recover 里取 taskID 的分支
	require.NotPanics(t, func() {
		td.hookTask(context.Background(), newQuickTask(t, "t1"), errors.New("task err"), td.doneHooks, "done", &HookExtraData{})
	})
}

// ---- createTask ----

func TestCreateTask_Panic(t *testing.T) {
	plugin.Reg(task.PluginTypeTask, func() *task.Task { panic("creator boom") }, func() any { return task.NewCfg() })
	defer plugin.Reg(task.PluginTypeTask, task.New, func() any { return task.NewCfg() })

	td := newTestTaskd(t)
	_, err := td.createTask(task.NewCfg().SetID("x"))
	require.Error(t, err)
	require.ErrorContains(t, err, "panic on create task")
}

// ---- Run ----

func TestRun_Canceled(t *testing.T) {
	td := &taskd{cfg: testCfg()}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- td.Run(ctx) }()

	require.Eventually(t, td.ready.Load, 2*time.Second, 10*time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestRun_ShutdownTimeout(t *testing.T) {
	td := &taskd{cfg: Cfg{Size: 1, QueueSize: 8, ShutdownTimeout: 100 * time.Millisecond}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- td.Run(ctx) }()

	require.Eventually(t, td.ready.Load, 2*time.Second, 10*time.Millisecond)

	// 占住 Run 创建的 pool,使 Stop 无法完成 → 触发超时
	blocker := make(chan struct{})
	td.pool.Submit(func() { <-blocker })
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after shutdown timeout")
	}
	close(blocker)
}
