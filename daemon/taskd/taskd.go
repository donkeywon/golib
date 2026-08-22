package taskd

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/task"
	"github.com/donkeywon/golib/util/reflects"
	"github.com/donkeywon/golib/util/v"
	"github.com/rs/zerolog"
)

const DaemonTypeTaskd boot.DaemonType = "taskd"

var (
	ErrNotReady            = errors.New("not ready")
	ErrStopping            = errors.New("stopping, reject")
	ErrTaskNotExists       = errors.New("task not exists")
	ErrTaskAlreadyExists   = errors.New("task already exists")
	ErrTaskAlreadyStopping = errors.New("task already stopping")
)

type TaskState uint32

func (ts TaskState) String() string {
	var s string
	switch ts {
	case TaskStatePending:
		s = "pending"
	case TaskStateRunning:
		s = "running"
	case TaskStateStopping:
		s = "stopping"
	default:
		s = "unknown"
	}
	return s
}

const (
	TaskStateUnknown  TaskState = 0
	TaskStatePending  TaskState = 1
	TaskStateRunning  TaskState = 2
	TaskStateStopping TaskState = 3
)

type Hook func(context.Context, *task.Task, error, *HookExtraData)

type HookExtraData struct {
	Wait bool
}

type taskInfo struct {
	task   *task.Task
	mu     sync.RWMutex
	state  TaskState
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func (ti *taskInfo) changeState(from, to TaskState) (TaskState, bool) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	if ti.state != from {
		return ti.state, false
	}

	ti.state = to
	return from, true
}

func (ti *taskInfo) getState() TaskState {
	ti.mu.RLock()
	defer ti.mu.RUnlock()

	return ti.state
}

type Taskd interface {
	boot.Daemon
	SubmitTask(ctx context.Context, taskCfg task.Cfg) (*task.Task, error)
	SubmitTaskAndWait(ctx context.Context, taskCfg task.Cfg) (*task.Task, error)
	StopTask(ctx context.Context, taskID string) error

	IsTaskExists(taskID string) bool
	ListTaskIDs() []string
	ListPendingTaskIDs() []string
	ListRunningTaskIDs() []string
	GetTaskState(taskID string) (TaskState, error)
	GetTaskCfg(taskID string) (task.Cfg, error)
	OnTaskCreate(hooks ...Hook)
	OnTaskSubmit(hooks ...Hook)
	OnTaskRun(hooks ...Hook)
	OnTaskDone(hooks ...Hook)
}

var _ Taskd = (*taskd)(nil)

type taskd struct {
	cfg Cfg
	l   *zerolog.Logger
	ctx context.Context

	ready atomic.Bool

	pool pond.Pool

	taskInfoMap sync.Map

	createHooks []Hook
	submitHooks []Hook
	runHooks    []Hook
	doneHooks   []Hook
}

func New() boot.Daemon {
	return &taskd{}
}

func (td *taskd) Run(ctx context.Context) error {
	td.pool = pond.NewPool(td.cfg.Size, pond.WithQueueSize(td.cfg.QueueSize), pond.WithContext(ctx))

	td.l = zerolog.Ctx(ctx)
	td.ctx = ctx
	td.ready.Store(true)

	<-ctx.Done()
	select {
	case <-td.pool.Stop().Done():
	case <-time.After(td.cfg.ShutdownTimeout):
	}
	return ctx.Err()
}

func (td *taskd) SetCfg(cfg Cfg) {
	td.cfg = cfg
}

func (td *taskd) SubmitTask(ctx context.Context, taskCfg task.Cfg) (*task.Task, error) {
	return td.createAndSubmit(ctx, taskCfg, false)
}

func (td *taskd) SubmitTaskAndWait(ctx context.Context, taskCfg task.Cfg) (*task.Task, error) {
	return td.createAndSubmit(ctx, taskCfg, true)
}

func (td *taskd) StopTask(ctx context.Context, taskID string) error {
	if !td.ready.Load() {
		return ErrNotReady
	}

	select {
	case <-td.ctx.Done():
		return ErrStopping
	default:
	}

	ti := td.getTaskInfo(taskID)
	if ti == nil {
		return ErrTaskNotExists
	}

	_, success := ti.changeState(TaskStateUnknown, TaskStateStopping)
	if success {
		// stop task before submit, just delete
		td.removeTaskInfo(taskID)
		return nil
	}

	_, success = ti.changeState(TaskStatePending, TaskStateStopping)
	if success {
		// stop task before run, just delete
		td.removeTaskInfo(taskID)
		return nil
	}

	old, success := ti.changeState(TaskStateRunning, TaskStateStopping)
	if !success {
		if old == TaskStateStopping {
			return ErrTaskAlreadyStopping
		}
		return errs.Errorf("task not running: %s", old.String())
	}

	ti.cancel()
	return nil
}

func (td *taskd) IsTaskExists(taskID string) bool {
	ti := td.getTaskInfo(taskID)
	return ti != nil
}

func (td *taskd) GetTaskState(taskID string) (TaskState, error) {
	ti := td.getTaskInfo(taskID)
	if ti == nil {
		return TaskStatePending, ErrTaskNotExists
	}
	return ti.getState(), nil
}

func (td *taskd) GetTaskCfg(taskID string) (task.Cfg, error) {
	ti := td.getTaskInfo(taskID)
	if ti == nil {
		return task.Cfg{}, ErrTaskNotExists
	}
	return ti.task.Cfg(), nil
}

func (td *taskd) ListTaskIDs() []string {
	ids := make([]string, 0, 16)
	td.taskInfoMap.Range(func(id, _ any) bool {
		ids = append(ids, id.(string))
		return true
	})
	return ids
}

func (td *taskd) ListPendingTaskIDs() []string {
	return td.listIDsByState(TaskStatePending)
}

func (td *taskd) ListRunningTaskIDs() []string {
	return td.listIDsByState(TaskStateRunning)
}

func (td *taskd) OnTaskCreate(hooks ...Hook) {
	td.createHooks = append(td.createHooks, hooks...)
}

func (td *taskd) OnTaskSubmit(hooks ...Hook) {
	td.submitHooks = append(td.submitHooks, hooks...)
}

func (td *taskd) OnTaskRun(hooks ...Hook) {
	td.runHooks = append(td.runHooks, hooks...)
}

func (td *taskd) OnTaskDone(hooks ...Hook) {
	td.doneHooks = append(td.doneHooks, hooks...)
}

func (td *taskd) submit(ti *taskInfo, wait bool) error {
	f := func() {
		defer close(ti.done)
		defer ti.cancel()

		oldState, success := ti.changeState(TaskStatePending, TaskStateRunning)
		if !success {
			td.l.Info().Str("task_id", ti.task.Cfg().ID).Str("task_state", oldState.String()).Msg("task state changed before running")
			return
		}

		td.hookTask(ti.ctx, ti.task, nil, td.runHooks, "run", &HookExtraData{Wait: wait})
		err := ti.task.Run(ti.ctx)
		td.hookTask(td.ctx, ti.task, err, td.doneHooks, "done", &HookExtraData{Wait: wait})

		td.removeTaskInfo(ti.task.Cfg().ID)
	}

	old, success := ti.changeState(TaskStateUnknown, TaskStatePending)
	if !success {
		return errs.Errorf("task state changed before submit: %s", old.String())
	}

	pt := td.pool.Submit(f)
	select {
	case <-pt.Done():
		// pool 已停止(pond 返回已完成的 future)或任务未执行成功,统一按未提交处理
		if err := pt.Wait(); err != nil {
			td.removeTaskInfo(ti.task.Cfg().ID)
			return ErrStopping
		}
	case <-td.ctx.Done():
		td.removeTaskInfo(ti.task.Cfg().ID)
		return ErrStopping
	default:
	}

	td.hookTask(ti.ctx, ti.task, nil, td.submitHooks, "submit", &HookExtraData{Wait: wait})
	if wait {
		pt.Wait()
	}
	return nil
}

func (td *taskd) createAndSubmit(ctx context.Context, taskCfg task.Cfg, wait bool) (*task.Task, error) {
	if !td.ready.Load() {
		return nil, ErrNotReady
	}

	select {
	case <-td.ctx.Done():
		return nil, ErrStopping
	default:
	}

	err := v.Struct(&taskCfg)
	if err != nil {
		return nil, errs.Wrap(err, "invalid task cfg")
	}

	hookExtra := &HookExtraData{Wait: wait}

	t, err := td.createTask(taskCfg)
	if err != nil {
		td.hookTask(ctx, t, err, td.createHooks, "create", hookExtra)
		return t, errs.Wrapf(err, "create task failed")
	}
	td.hookTask(ctx, t, nil, td.createHooks, "create", hookExtra)

	var (
		ti     *taskInfo
		exists bool
	)
	if ti, exists = td.addTaskInfo(taskCfg.ID, t); exists {
		return nil, ErrTaskAlreadyExists
	}

	ti.done = make(chan struct{})
	if wait {
		ti.ctx, ti.cancel = context.WithCancel(ctx)
	} else {
		ti.ctx, ti.cancel = context.WithCancel(td.ctx)
	}

	err = td.submit(ti, wait)
	if err != nil {
		return nil, errs.Wrap(err, "submit failed")
	}
	return t, nil
}

func (td *taskd) createTask(cfg task.Cfg) (t *task.Task, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = errs.PanicToErrWithMsg(p, "panic on create task")
		}
	}()
	return plugin.CreateWithCfg[*task.Task](task.PluginTypeTask, cfg), nil
}

func (td *taskd) addTaskInfo(taskID string, t *task.Task) (*taskInfo, bool) {
	tia, loaded := td.taskInfoMap.LoadOrStore(taskID, &taskInfo{task: t})
	return tia.(*taskInfo), loaded
}

func (td *taskd) removeTaskInfo(taskID string) {
	td.taskInfoMap.Delete(taskID)
}

func (td *taskd) getTaskInfo(taskID string) *taskInfo {
	tia, exists := td.taskInfoMap.Load(taskID)
	if !exists {
		return nil
	}
	return tia.(*taskInfo)
}

func (td *taskd) listIDsByState(state TaskState) []string {
	taskIDs := make([]string, 0, 16)
	td.taskInfoMap.Range(func(id, tia any) bool {
		ti := tia.(*taskInfo)
		if ti.getState() == state {
			taskIDs = append(taskIDs, ti.task.Cfg().ID)
		}
		return true
	})
	return taskIDs
}

func (td *taskd) hookTask(ctx context.Context, t *task.Task, err error, hooks []Hook, hookType string, extra *HookExtraData) {
	for i, h := range hooks {
		td.hook(ctx, t, err, h, i, hookType, extra)
	}
}

func (td *taskd) hook(ctx context.Context, t *task.Task, err error, h Hook, hookIdx int, hookType string, extra *HookExtraData) {
	defer func() {
		p := recover()
		if p == nil {
			return
		}

		var taskID string
		if t != nil {
			taskID = t.Cfg().ID
		}
		td.l.Error().Err(errs.PanicToErr(p)).Str("task_id", taskID).Int("idx", hookIdx).Str("hook", reflects.GetFuncName(h)).Str("hook_type", hookType).Msg("panic on hook task")
	}()
	h(ctx, t, err, extra)
}
