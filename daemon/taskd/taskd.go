package taskd

import (
	"context"
	"errors"
	"sync"

	"github.com/alitto/pond/v2"
	"github.com/donkeywon/golib/boot"
	"github.com/donkeywon/golib/errs"
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
	ErrTaskNotStarted      = errors.New("task not started")
	ErrTaskNotPaused       = errors.New("task not paused")
)

var D Taskd = New()

type Taskd interface {
	boot.Daemon
	SubmitTask(taskCfg *task.Cfg) (*task.Task, error)
	SubmitTaskAndWait(context.Context, *task.Cfg) (*task.Task, error)
	StopTask(taskID string) error
	PauseTask(taskID string) error
	ResumeTask(taskID string) (*task.Task, error)
	IsTaskExists(taskID string) bool
	IsTaskPending(taskID string) bool
	IsTaskRunning(taskID string) bool
	IsTaskPaused(taskID string) bool
	ListTasks() []*task.Task
	ListTasksCfg() []*task.Cfg
	ListTaskIDs() []string
	ListPendingTaskIDs() []string
	ListRunningTaskIDs() []string
	ListPausedTaskIDs() []string
	GetTaskCfg(taskID string) (*task.Cfg, error)
	OnTaskCreate(hooks ...task.Hook)
	OnTaskInit(hooks ...task.Hook)
	OnTaskSubmit(hooks ...task.Hook)
	OnTaskStart(hooks ...task.Hook)
	OnTaskPausing(hooks ...task.Hook)
	OnTaskPaused(hooks ...task.Hook)
	OnTaskDone(hooks ...task.Hook)
	OnTaskStepDone(hooks ...task.StepHook)
	OnTaskDeferStepDone(hooks ...task.StepHook)
}

type taskd struct {
	runner.Runner

	cfg *Cfg

	pools map[string]pond.Pool

	mu               sync.Mutex
	taskIDMap        map[string]struct{}   // task id map include pending, except paused
	taskMap          map[string]*task.Task // task map include pending, except paused
	taskIDRunningMap map[string]struct{}   // running task id map
	taskPausedMap    map[string]*task.Task // paused task map

	createHooks        []task.Hook
	initHooks          []task.Hook
	submitHooks        []task.Hook
	startHooks         []task.Hook
	pausingHooks       []task.Hook
	pausedHooks        []task.Hook
	doneHooks          []task.Hook
	stepDoneHooks      []task.StepHook
	deferStepDoneHooks []task.StepHook
}

func New() Taskd {
	return &taskd{
		Runner:           runner.Create(string(DaemonTypeTaskd)),
		taskMap:          make(map[string]*task.Task),
		taskIDMap:        make(map[string]struct{}),
		taskIDRunningMap: make(map[string]struct{}),
		taskPausedMap:    make(map[string]*task.Task),
		pools:            make(map[string]pond.Pool),
	}
}

func (td *taskd) Init() error {
	if len(td.cfg.Pools) == 0 {
		return errs.New("no pools")
	}
	for _, poolCfg := range td.cfg.Pools {
		td.pools[poolCfg.Name] = pond.NewPool(poolCfg.Size, pond.WithQueueSize(poolCfg.QueueSize))
	}
	return td.Runner.Init()
}

func (td *taskd) Start() error {
	<-td.Stopping()
	td.waitAllTaskDone()
	for _, pool := range td.pools {
		pool.Stop()
	}
	return td.Runner.Start()
}

func (td *taskd) Stop() error {
	td.Cancel()
	return nil
}

func (td *taskd) getPool(taskCfg *task.Cfg) pond.Pool {
	p := td.pools[taskCfg.Pool]
	if p == nil {
		panic("task pool not exists: " + taskCfg.Pool)
	}
	return p
}

func (td *taskd) SetCfg(cfg any) {
	td.cfg = cfg.(*Cfg)
}

func (td *taskd) SubmitTask(taskCfg *task.Cfg) (*task.Task, error) {
	return td.createInitSubmit(td.Ctx(), taskCfg, false)
}

func (td *taskd) SubmitTaskAndWait(ctx context.Context, taskCfg *task.Cfg) (*task.Task, error) {
	return td.createInitSubmit(ctx, taskCfg, true)
}

func (td *taskd) StopTask(taskID string) error {
	select {
	case <-td.Stopping():
		return ErrStopping
	default:
	}

	isPaused, _ := td.unmarkTaskIfPaused(taskID)
	if isPaused {
		// task is paused, just unmark it
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

	runner.StopAndWait(t)
	return nil
}

func (td *taskd) PauseTask(taskID string) error {
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

	td.hookTask(t, nil, td.pausingHooks, "pausing", nil)
	runner.StopAndWait(t)
	td.markTaskPaused(t)
	td.hookTask(t, nil, td.pausedHooks, "paused", nil)
	return nil
}

func (td *taskd) ResumeTask(taskID string) (*task.Task, error) {
	select {
	case <-td.Stopping():
		return nil, ErrStopping
	default:
	}

	isPaused, t := td.unmarkTaskIfPaused(taskID)
	if !isPaused {
		return nil, ErrTaskNotPaused
	}

	newT, err := td.createInitSubmit(td.Ctx(), t.Cfg, false, func(newT *task.Task, err error, hed *task.HookExtraData) {
		data := t.LoadAll()
		for k, v := range data {
			newT.Store(k, v)
		}

		for i, newStep := range newT.Steps() {
			data = t.Steps()[i].LoadAll()
			for k, v := range data {
				newStep.Store(k, v)
			}
		}
		for i, newStep := range newT.DeferSteps() {
			data = t.DeferSteps()[i].LoadAll()
			for k, v := range data {
				newStep.Store(k, v)
			}
		}
	})

	if err != nil {
		td.markTaskPaused(t)
		return newT, err
	}

	return newT, nil
}

func (td *taskd) waitAllTaskDone() {
	for _, t := range td.ListTasks() {
		<-t.Done()
	}
}

func (td *taskd) createInit(ctx context.Context, taskCfg *task.Cfg, extra *task.HookExtraData, beforeInit ...task.Hook) (*task.Task, error) {
	err := v.Struct(taskCfg)
	if err != nil {
		return nil, errs.Wrap(err, "invalid task cfg")
	}

	t, err := td.createTask(taskCfg)
	if err == nil {
		for k, value := range taskCfg.Values {
			t.Store(k, value)
		}

		t.SetCtx(ctx)
		t.Inherit(td)
		t.WithLoggerFields("task_id", t.Cfg.ID, "task_type", t.Cfg.Type)
	}
	td.hookTask(t, err, td.createHooks, "create", extra)
	if err != nil {
		return t, errs.Wrap(err, "create task failed")
	}

	t.HookStepDone(td.stepDoneHooks...)
	t.HookDeferStepDone(td.deferStepDoneHooks...)

	for _, h := range beforeInit {
		h(t, nil, extra)
	}

	err = td.initTask(t)
	td.hookTask(t, err, td.initHooks, "init", extra)
	if err != nil {
		return t, errs.Wrap(err, "init task failed")
	}

	return t, nil
}

func (td *taskd) submit(t *task.Task, wait bool) {
	extra := &task.HookExtraData{Wait: wait}

	f := func() {
		td.markTaskRunning(t.Cfg.ID)
		td.hookTask(t, nil, td.startHooks, "start", extra)

		err := runner.Run(t)
		td.unmarkTaskAndTaskID(t.Cfg.ID)
		td.hookTask(t, err, td.doneHooks, "done", extra)
	}

	td.markTask(t)

	pt := td.getPool(t.Cfg).Submit(f)
	if wait {
		pt.Wait()
	}

	td.hookTask(t, nil, td.submitHooks, "submit", extra)
}

func (td *taskd) createInitSubmit(ctx context.Context, taskCfg *task.Cfg, wait bool, beforeInit ...task.Hook) (*task.Task, error) {
	select {
	case <-td.Stopping():
		return nil, ErrStopping
	default:
	}

	hookExtra := &task.HookExtraData{Wait: wait}

	marked := td.markTaskID(taskCfg.ID)
	if !marked {
		return nil, ErrTaskAlreadyExists
	}

	t, err := td.createInit(ctx, taskCfg, hookExtra, beforeInit...)
	if err != nil {
		td.unmarkTaskID(taskCfg.ID)
		return nil, errs.Wrap(err, "create init task failed")
	}

	select {
	case <-td.Stopping():
		return nil, ErrStopping
	default:
	}

	td.submit(t, wait)
	return t, nil
}

func (td *taskd) createTask(cfg *task.Cfg) (t *task.Task, err error) {
	defer func() {
		e := recover()
		if e != nil {
			err = errs.PanicToErrWithMsg(e, "panic on create task")
		}
	}()
	return plugin.CreateWithCfg[plugin.Type, *task.Task](task.PluginTypeTask, cfg), nil
}

func (td *taskd) initTask(t *task.Task) (err error) {
	defer func() {
		e := recover()
		if e != nil {
			err = errs.PanicToErrWithMsg(e, "panic on init task")
		}
	}()

	return runner.Init(t)
}

func (td *taskd) markTaskID(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDMap[taskID]
	if exists {
		return false
	}
	_, exists = td.taskPausedMap[taskID]
	if exists {
		return false
	}

	td.taskIDMap[taskID] = struct{}{}
	return true
}

func (td *taskd) unmarkTaskID(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDMap[taskID]
	if !exists {
		return false
	}
	delete(td.taskIDMap, taskID)
	return true
}

func (td *taskd) unmarkTaskAndTaskID(taskID string) {
	td.mu.Lock()
	defer td.mu.Unlock()
	delete(td.taskIDRunningMap, taskID)
	delete(td.taskIDMap, taskID)
	delete(td.taskMap, taskID)
}

func (td *taskd) markTask(t *task.Task) {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.taskMap[t.Cfg.ID] = t
}

func (td *taskd) markTaskRunning(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDRunningMap[taskID]
	if exists {
		return false
	}
	td.taskIDRunningMap[taskID] = struct{}{}
	return true
}

func (td *taskd) markTaskPaused(t *task.Task) {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.taskPausedMap[t.Cfg.ID] = t
}

func (td *taskd) unmarkTaskIfPaused(taskID string) (bool, *task.Task) {
	td.mu.Lock()
	defer td.mu.Unlock()
	t, exists := td.taskPausedMap[taskID]
	if !exists {
		return false, t
	}
	delete(td.taskPausedMap, taskID)
	return true, t
}

func (td *taskd) ListTasksCfg() []*task.Cfg {
	td.mu.Lock()
	defer td.mu.Unlock()
	tasks := make([]*task.Task, len(td.taskMap)+len(td.taskPausedMap))
	i := 0
	for _, t := range td.taskMap {
		tasks[i] = t
		i++
	}
	for _, t := range td.taskPausedMap {
		tasks[i] = t
		i++
	}
	cfgs := make([]*task.Cfg, len(tasks))
	for i = range tasks {
		cfgs[i] = tasks[i].Cfg
	}
	return cfgs
}

func (td *taskd) ListTasks() []*task.Task {
	td.mu.Lock()
	defer td.mu.Unlock()
	tasks := make([]*task.Task, len(td.taskMap))
	i := 0
	for _, t := range td.taskMap {
		tasks[i] = t
		i++
	}
	return tasks
}

func (td *taskd) hookTask(t *task.Task, err error, hooks []task.Hook, hookType string, extra *task.HookExtraData) {
	for i, h := range hooks {
		func(idx int, h task.Hook) {
			defer func() {
				err := recover()
				if err != nil {
					if t == nil {
						td.Error("panic on hook task", errs.PanicToErr(err), "idx", idx, "hook", reflects.GetFuncName(h), "hook_type", hookType)
					} else {
						td.Error("panic on hook task", errs.PanicToErr(err), "idx", idx, "hook", reflects.GetFuncName(h), "hook_type", hookType, "task_id", t.Cfg.ID, "task_type", t.Cfg.Type)
					}
				}
			}()
			h(t, err, extra)
		}(i, h)
	}
}

func (td *taskd) getTask(taskID string) *task.Task {
	td.mu.Lock()
	defer td.mu.Unlock()
	return td.taskMap[taskID]
}

func (td *taskd) IsTaskExists(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDMap[taskID]
	if exists {
		return true
	}
	_, exists = td.taskPausedMap[taskID]
	return exists
}

func (td *taskd) IsTaskPending(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDMap[taskID]
	if !exists {
		return false
	}
	_, isRunning := td.taskIDRunningMap[taskID]
	_, isPaused := td.taskPausedMap[taskID]
	return !isRunning && !isPaused
}

func (td *taskd) IsTaskRunning(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDRunningMap[taskID]
	return exists
}

func (td *taskd) IsTaskPaused(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskPausedMap[taskID]
	return exists
}

func (td *taskd) ListTaskIDs() []string {
	td.mu.Lock()
	defer td.mu.Unlock()
	ids := make([]string, len(td.taskIDMap)+len(td.taskPausedMap))
	i := 0
	for id := range td.taskIDMap {
		ids[i] = id
		i++
	}
	for id := range td.taskPausedMap {
		ids[i] = id
		i++
	}
	return ids
}

func (td *taskd) ListPendingTaskIDs() []string {
	td.mu.Lock()
	defer td.mu.Unlock()
	ids := make([]string, len(td.taskIDMap)-len(td.taskIDRunningMap))
	i := 0
	for id := range td.taskIDMap {
		if _, isRunning := td.taskIDRunningMap[id]; isRunning {
			continue
		}

		ids[i] = id
		i++
	}
	return ids
}

func (td *taskd) ListRunningTaskIDs() []string {
	td.mu.Lock()
	defer td.mu.Unlock()
	ids := make([]string, len(td.taskIDRunningMap))
	i := 0
	for id := range td.taskIDRunningMap {
		ids[i] = id
		i++
	}
	return ids
}

func (td *taskd) ListPausedTaskIDs() []string {
	td.mu.Lock()
	defer td.mu.Unlock()
	ids := make([]string, len(td.taskPausedMap))
	i := 0
	for id := range td.taskPausedMap {
		ids[i] = id
		i++
	}
	return ids
}

func (td *taskd) GetTaskCfg(taskID string) (*task.Cfg, error) {
	td.mu.Lock()
	defer td.mu.Unlock()
	t, exists := td.taskMap[taskID]
	if !exists {
		return nil, ErrTaskNotExists
	}
	return t.Cfg, nil
}

func (td *taskd) OnTaskCreate(hooks ...task.Hook) {
	td.createHooks = append(td.createHooks, hooks...)
}

func (td *taskd) OnTaskInit(hooks ...task.Hook) {
	td.initHooks = append(td.initHooks, hooks...)
}

func (td *taskd) OnTaskSubmit(hooks ...task.Hook) {
	td.submitHooks = append(td.submitHooks, hooks...)
}

func (td *taskd) OnTaskStart(hooks ...task.Hook) {
	td.startHooks = append(td.startHooks, hooks...)
}

func (td *taskd) OnTaskDone(hooks ...task.Hook) {
	td.doneHooks = append(td.doneHooks, hooks...)
}

func (td *taskd) OnTaskPaused(hooks ...task.Hook) {
	td.pausedHooks = append(td.pausedHooks, hooks...)
}

func (td *taskd) OnTaskPausing(hooks ...task.Hook) {
	td.pausingHooks = append(td.pausingHooks, hooks...)
}

func (td *taskd) OnTaskStepDone(hooks ...task.StepHook) {
	td.stepDoneHooks = append(td.stepDoneHooks, hooks...)
}

func (td *taskd) OnTaskDeferStepDone(hooks ...task.StepHook) {
	td.deferStepDoneHooks = append(td.deferStepDoneHooks, hooks...)
}
