package taskd

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/alitto/pond/v2"
	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/logs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/task"
	"github.com/donkeywon/golib/util/reflects"
	"github.com/donkeywon/golib/util/v"
)

const DaemonTypeTaskd boot.DaemonType = "taskd"

var (
	ErrStopping            = errors.New("stopping, reject")
	ErrTaskNotExists       = errors.New("task not exists")
	ErrTaskAlreadyExists   = errors.New("task already exists")
	ErrTaskAlreadyStopping = errors.New("task already stopping")
	ErrTaskAlreadyPausing  = errors.New("task already pausing")
	ErrTaskNotStarted      = errors.New("task not started")
	ErrTaskNotPaused       = errors.New("task not paused")
	ErrPoolNotExists       = errors.New("pool not exists")
)

type TaskState uint32

const (
	TaskStatePending TaskState = 0
	TaskStateRunning TaskState = 1
	TaskStateDone    TaskState = 2
	TaskStatePausing TaskState = 3
	TaskStatePaused  TaskState = 4
)

type Hook func(*task.Task, error, *HookExtraData)

type HookExtraData struct {
	Wait bool
}

type taskInfo struct {
	task  *task.Task
	state atomic.Uint32
	pool  string
}

type Taskd interface {
	boot.Daemon
	SubmitTask(ctx context.Context, pool string, taskCfg task.Cfg) (*task.Task, error)
	SubmitTaskAndWait(ctx context.Context, pool string, taskCfg task.Cfg) (*task.Task, error)
	StopTask(ctx context.Context, taskID string) error
	PauseTask(ctx context.Context, taskID string) error
	ResumeTask(ctx context.Context, taskID string) (*task.Task, error)
	IsTaskExists(taskID string) bool
	ListTasks() []*task.Task
	ListTasksCfg() []task.Cfg
	ListTaskIDs() []string
	ListPendingTaskIDs() []string
	ListRunningTaskIDs() []string
	ListPausingTaskIDs() []string
	ListPausedTaskIDs() []string
	GetTaskState(taskID string) (TaskState, error)
	GetTaskCfg(taskID string) (task.Cfg, error)
	OnTaskCreate(hooks ...Hook)
	OnTaskInit(hooks ...Hook)
	OnTaskSubmit(hooks ...Hook)
	OnTaskStart(hooks ...Hook)
	OnTaskPausing(hooks ...Hook)
	OnTaskPaused(hooks ...Hook)
	OnTaskDone(hooks ...Hook)
}

var _ Taskd = (*taskd)(nil)

type taskd struct {
	runner.Base

	cfg    *Cfg
	l      *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc

	pools map[string]pond.Pool

	mu          sync.RWMutex
	taskInfoMap map[string]*taskInfo

	createHooks  []Hook
	initHooks    []Hook
	submitHooks  []Hook
	startHooks   []Hook
	pausingHooks []Hook
	pausedHooks  []Hook
	doneHooks    []Hook
}

func New() boot.Daemon {
	return &taskd{
		taskInfoMap: make(map[string]*taskInfo),
		pools:       make(map[string]pond.Pool),
	}
}

func (td *taskd) Init(ctx context.Context) error {
	if len(td.cfg.Pools) == 0 {
		return errs.New("no pools")
	}
	for _, poolCfg := range td.cfg.Pools {
		td.pools[poolCfg.Name] = pond.NewPool(poolCfg.Size, pond.WithQueueSize(poolCfg.QueueSize))
	}
	return nil
}

func (td *taskd) Start(ctx context.Context) error {
	td.l = logs.FromCtx(ctx)
	td.ctx, td.cancel = context.WithCancel(ctx)

	<-td.Stopping()
	td.waitAllTaskDone()
	for _, pool := range td.pools {
		pool.Stop()
	}
	return nil
}

func (td *taskd) Stop(ctx context.Context) error {
	if td.cancel != nil {
		td.cancel()
	}
	return nil
}

func (td *taskd) SetCfg(cfg any) {
	td.cfg = cfg.(*Cfg)
}

func (td *taskd) SubmitTask(ctx context.Context, pool string, taskCfg task.Cfg) (*task.Task, error) {
	return td.createInitSubmit(ctx, pool, taskCfg, false)
}

func (td *taskd) SubmitTaskAndWait(ctx context.Context, pool string, taskCfg task.Cfg) (*task.Task, error) {
	return td.createInitSubmit(ctx, pool, taskCfg, true)
}

func (td *taskd) StopTask(ctx context.Context, taskID string) error {
	select {
	case <-td.Stopping():
		return ErrStopping
	default:
	}

	if t, _ := td.removeTaskIfPaused(taskID); t != nil {
		return nil
	}

	t := td.getTask(taskID)
	if t == nil {
		return ErrTaskNotExists
	}

	select {
	case <-t.Stopping():
		return ErrTaskAlreadyStopping
	default:
	}

	return runner.Stop(ctx, t)
}

func (td *taskd) PauseTask(ctx context.Context, taskID string) error {
	select {
	case <-td.Stopping():
		return ErrStopping
	default:
	}

	t := td.getTask(taskID)
	if t == nil {
		return ErrTaskNotExists
	}

	select {
	case <-t.Started():
	default:
		return ErrTaskNotStarted
	}

	select {
	case <-t.Stopping():
		return ErrTaskAlreadyStopping
	default:
	}

	if !td.changeTaskState(taskID, TaskStateRunning, TaskStatePausing) {
		return ErrTaskAlreadyPausing
	}

	td.hookTask(t, nil, td.pausingHooks, "pausing", nil)
	return runner.Stop(ctx, t)
}

func (td *taskd) ResumeTask(ctx context.Context, taskID string) (*task.Task, error) {
	select {
	case <-td.Stopping():
		return nil, ErrStopping
	default:
	}

	t, pool := td.removeTaskIfPaused(taskID)
	if t == nil {
		return nil, ErrTaskNotPaused
	}

	oldCfg := t.Cfg()
	newT, err := td.createInitSubmit(ctx, pool, oldCfg, false, func(newT *task.Task, _ error, _ *HookExtraData) {
		t.Range(func(k string, val any) bool {
			newT.Store(k, val)
			return true
		})

		oldSteps := t.Steps()
		for i, newStep := range newT.Steps() {
			oldSteps[i].Range(func(k string, val any) bool {
				newStep.Store(k, val)
				return true
			})
		}

		oldDeferSteps := t.DeferSteps()
		for i, newStep := range newT.DeferSteps() {
			oldDeferSteps[i].Range(func(k string, val any) bool {
				newStep.Store(k, val)
				return true
			})
		}
	})
	if err != nil {
		td.restorePaused(t, pool)
		return newT, err
	}

	return newT, nil
}

func (td *taskd) IsTaskExists(taskID string) bool {
	td.mu.RLock()
	defer td.mu.RUnlock()
	_, exists := td.taskInfoMap[taskID]
	return exists
}

func (td *taskd) isTaskPausing(taskID string) bool {
	td.mu.RLock()
	defer td.mu.RUnlock()
	e, exists := td.taskInfoMap[taskID]
	return exists && TaskState(e.state.Load()) == TaskStatePausing
}

func (td *taskd) GetTaskState(taskID string) (TaskState, error) {
	td.mu.RLock()
	defer td.mu.RUnlock()
	e, exists := td.taskInfoMap[taskID]
	if !exists {
		return TaskStatePending, ErrTaskNotExists
	}
	return TaskState(e.state.Load()), nil
}

func (td *taskd) GetTaskCfg(taskID string) (task.Cfg, error) {
	td.mu.RLock()
	defer td.mu.RUnlock()
	e, exists := td.taskInfoMap[taskID]
	if !exists {
		return task.Cfg{}, ErrTaskNotExists
	}
	return e.task.Cfg(), nil
}

func (td *taskd) ListTasks() []*task.Task {
	td.mu.RLock()
	defer td.mu.RUnlock()

	tasks := make([]*task.Task, 0, len(td.taskInfoMap))
	for _, e := range td.taskInfoMap {
		if TaskState(e.state.Load()) != TaskStatePaused {
			tasks = append(tasks, e.task)
		}
	}
	return tasks
}

func (td *taskd) ListTasksCfg() []task.Cfg {
	td.mu.RLock()
	defer td.mu.RUnlock()

	cfgs := make([]task.Cfg, 0, len(td.taskInfoMap))
	for _, e := range td.taskInfoMap {
		cfgs = append(cfgs, e.task.Cfg())
	}
	return cfgs
}

func (td *taskd) ListTaskIDs() []string {
	td.mu.RLock()
	defer td.mu.RUnlock()

	ids := make([]string, 0, len(td.taskInfoMap))
	for id := range td.taskInfoMap {
		ids = append(ids, id)
	}
	return ids
}

func (td *taskd) ListPendingTaskIDs() []string {
	return td.listIDsByState(TaskStatePending)
}

func (td *taskd) ListRunningTaskIDs() []string {
	return td.listIDsByState(TaskStateRunning)
}

func (td *taskd) ListPausingTaskIDs() []string {
	return td.listIDsByState(TaskStatePausing)
}

func (td *taskd) ListPausedTaskIDs() []string {
	return td.listIDsByState(TaskStatePaused)
}

func (td *taskd) OnTaskCreate(hooks ...Hook) {
	td.createHooks = append(td.createHooks, hooks...)
}

func (td *taskd) OnTaskInit(hooks ...Hook) {
	td.initHooks = append(td.initHooks, hooks...)
}

func (td *taskd) OnTaskSubmit(hooks ...Hook) {
	td.submitHooks = append(td.submitHooks, hooks...)
}

func (td *taskd) OnTaskStart(hooks ...Hook) {
	td.startHooks = append(td.startHooks, hooks...)
}

func (td *taskd) OnTaskPausing(hooks ...Hook) {
	td.pausingHooks = append(td.pausingHooks, hooks...)
}

func (td *taskd) OnTaskPaused(hooks ...Hook) {
	td.pausedHooks = append(td.pausedHooks, hooks...)
}

func (td *taskd) OnTaskDone(hooks ...Hook) {
	td.doneHooks = append(td.doneHooks, hooks...)
}

func (td *taskd) waitAllTaskDone() {
	for _, t := range td.ListTasks() {
		<-t.Done()
	}
}

func (td *taskd) createInit(ctx context.Context, taskCfg task.Cfg, extra *HookExtraData, beforeInit ...Hook) (*task.Task, error) {
	if err := v.Struct(taskCfg); err != nil {
		return nil, errs.Wrap(err, "invalid task cfg")
	}

	t, err := td.createTask(taskCfg)
	if err != nil {
		td.hookTask(t, err, td.createHooks, "create", extra)
		return t, errs.Wrapf(err, "create task failed")
	}

	td.hookTask(t, nil, td.createHooks, "create", extra)

	for _, h := range beforeInit {
		h(t, nil, extra)
	}

	err = td.initTask(ctx, t)
	td.hookTask(t, err, td.initHooks, "init", extra)
	if err != nil {
		return t, errs.Wrap(err, "init task failed")
	}

	return t, nil
}

func (td *taskd) submit(t *task.Task, pool string, wait bool) {
	taskID := t.Cfg().ID

	f := func() {
		td.changeTaskState(taskID, TaskStatePending, TaskStateRunning)

		td.hookTask(t, nil, td.startHooks, "start", &HookExtraData{Wait: wait})
		err := runner.Start(td.ctx, t)

		// TODO 通过err判断是否pausing
		if td.isTaskPausing(taskID) {
			td.changeTaskState(taskID, TaskStatePausing, TaskStatePaused)
			td.hookTask(t, nil, td.pausedHooks, "paused", nil)
		} else {
			td.removeTaskInfo(taskID)
		}

		td.hookTask(t, err, td.doneHooks, "done", &HookExtraData{Wait: wait})
	}

	pt := td.pools[pool].Submit(f)
	if wait {
		pt.Wait()
	}

	td.hookTask(t, nil, td.submitHooks, "submit", &HookExtraData{Wait: wait})
}

func (td *taskd) createInitSubmit(ctx context.Context, pool string, taskCfg task.Cfg, wait bool, beforeInit ...Hook) (*task.Task, error) {
	select {
	case <-td.Stopping():
		return nil, ErrStopping
	default:
	}

	if pool == "" || td.pools[pool] == nil {
		return nil, ErrPoolNotExists
	}
	hookExtra := &HookExtraData{Wait: wait}

	if !td.addTaskInfo(taskCfg.ID, pool) {
		return nil, ErrTaskAlreadyExists
	}

	t, err := td.createInit(ctx, taskCfg, hookExtra, beforeInit...)
	if err != nil {
		td.removeTaskInfo(taskCfg.ID)
		return nil, errs.Wrap(err, "create init task failed")
	}

	select {
	case <-td.Stopping():
		td.removeTaskInfo(taskCfg.ID)
		return nil, ErrStopping
	default:
	}

	td.setTaskToTaskInfo(t)
	td.submit(t, pool, wait)
	return t, nil
}

func (td *taskd) createTask(cfg task.Cfg) (t *task.Task, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = errs.PanicToErrWithMsg(p, "panic on create task")
		}
	}()
	return plugin.CreateWithCfg[*task.Task](task.PluginTypeTask, &cfg), nil
}

func (td *taskd) initTask(ctx context.Context, t *task.Task) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = errs.PanicToErrWithMsg(p, "panic on init task")
		}
	}()
	return runner.Init(ctx, t)
}

func (td *taskd) addTaskInfo(taskID, pool string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()

	if _, exists := td.taskInfoMap[taskID]; exists {
		return false
	}
	td.taskInfoMap[taskID] = &taskInfo{pool: pool}
	return true
}

func (td *taskd) setTaskToTaskInfo(t *task.Task) {
	td.mu.RLock()
	defer td.mu.RUnlock()

	if e, exists := td.taskInfoMap[t.Cfg().ID]; exists {
		e.task = t
	}
}

func (td *taskd) removeTaskInfo(taskID string) {
	td.mu.Lock()
	defer td.mu.Unlock()

	delete(td.taskInfoMap, taskID)
}

func (td *taskd) changeTaskState(taskID string, from, to TaskState) bool {
	td.mu.RLock()
	defer td.mu.RUnlock()

	e, exists := td.taskInfoMap[taskID]
	if !exists {
		return false
	}
	return e.state.CompareAndSwap(uint32(from), uint32(to))
}

func (td *taskd) removeTaskIfPaused(taskID string) (*task.Task, string) {
	td.mu.Lock()
	defer td.mu.Unlock()

	e, exists := td.taskInfoMap[taskID]
	if !exists || TaskState(e.state.Load()) != TaskStatePaused {
		return nil, ""
	}
	t := e.task
	pool := e.pool
	delete(td.taskInfoMap, taskID)
	return t, pool
}

func (td *taskd) restorePaused(t *task.Task, pool string) {
	td.mu.Lock()
	defer td.mu.Unlock()

	info := &taskInfo{
		task: t,
		pool: pool,
	}
	info.state.Store(uint32(TaskStatePaused))

	td.taskInfoMap[t.Cfg().ID] = info
}

func (td *taskd) getTask(taskID string) *task.Task {
	td.mu.RLock()
	defer td.mu.RUnlock()

	e, exists := td.taskInfoMap[taskID]
	if !exists {
		return nil
	}
	return e.task
}

func (td *taskd) listIDsByState(state TaskState) []string {
	td.mu.RLock()
	defer td.mu.RUnlock()

	ids := make([]string, 0, len(td.taskInfoMap))
	for id, e := range td.taskInfoMap {
		if TaskState(e.state.Load()) == state {
			ids = append(ids, id)
		}
	}
	return ids
}

func (td *taskd) hookTask(t *task.Task, err error, hooks []Hook, hookType string, extra *HookExtraData) {
	for i, h := range hooks {
		td.hook(t, err, h, i, hookType, extra)
	}
}

func (td *taskd) hook(t *task.Task, err error, h Hook, hookIdx int, hookType string, extra *HookExtraData) {
	defer func() {
		p := recover()
		if p == nil {
			return
		}

		var taskID string
		if t != nil {
			taskID = t.Cfg().ID
		}
		td.l.Error("panic on hook task",
			"task_id", taskID,
			"err", errs.PanicToErr(p),
			"idx", hookIdx,
			"hook", reflects.GetFuncName(h),
			"hook_type", hookType)
	}()
	h(t, err, extra)
}
