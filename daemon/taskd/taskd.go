package taskd

import (
	"errors"
	"sync"

	"github.com/alitto/pond"
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
	ErrTaskNotPausing      = errors.New("task not pausing")
)

var _td = New()

type Taskd struct {
	runner.Runner
	*Cfg

	pool *pond.WorkerPool

	mu               sync.Mutex
	taskIDMap        map[string]struct{}   // task id map include waiting, except pausing
	taskMap          map[string]*task.Task // task map include waiting, except pausing
	taskIDRunningMap map[string]struct{}   // running task id map
	taskPausingMap   map[string]*task.Task // pausing task map

	createHooks        []task.Hook
	initHooks          []task.Hook
	submitHooks        []task.Hook
	startHooks         []task.Hook
	doneHooks          []task.Hook
	stepDoneHooks      []task.StepHook
	deferStepDoneHooks []task.StepHook
}

func D() *Taskd {
	return _td
}

func New() *Taskd {
	return &Taskd{
		Runner:           runner.Create(string(DaemonTypeTaskd)),
		taskMap:          make(map[string]*task.Task),
		taskIDMap:        make(map[string]struct{}),
		taskIDRunningMap: make(map[string]struct{}),
		taskPausingMap:   make(map[string]*task.Task),
	}
}

func (td *Taskd) Init() error {
	td.pool = pond.New(td.Cfg.PoolSize, td.Cfg.QueueSize)
	return td.Runner.Init()
}

func (td *Taskd) Start() error {
	td.Info("ready for task")
	<-td.Stopping()
	td.waitAllTaskDone()
	td.pool.Stop()
	return td.Runner.Start()
}

func (td *Taskd) Stop() error {
	td.Cancel()
	return nil
}

func (td *Taskd) Type() interface{} {
	return DaemonTypeTaskd
}

func (td *Taskd) GetCfg() interface{} {
	return td.Cfg
}

func (td *Taskd) SubmitTask(taskCfg *task.Cfg) (*task.Task, error) {
	t, _, err := td.createInitSubmit(taskCfg, false, true, nil)
	return t, err
}

func (td *Taskd) SubmitTaskAndWait(taskCfg *task.Cfg) (*task.Task, error) {
	t, _, err := td.createInitSubmit(taskCfg, true, true, nil)
	return t, err
}

func (td *Taskd) TrySubmitTask(taskCfg *task.Cfg) (*task.Task, bool, error) {
	t, submitted, err := td.createInitSubmit(taskCfg, false, false, nil)
	return t, submitted, err
}

func (td *Taskd) StopTask(taskID string) error {
	select {
	case <-td.Stopping():
		return ErrStopping
	default:
	}

	isPausing, _ := td.unmarkTaskIfPausing(taskID)
	if isPausing {
		// task is pausing, just unmark it
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

	go runner.Stop(t)
	<-t.Done()
	td.Info("task stopped", "task_id", taskID)
	return nil
}

func (td *Taskd) PauseTask(taskID string) error {
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

	td.Info("pausing task", "task_id", taskID)
	go runner.Stop(t)
	<-t.Done()
	td.Info("task paused", "task_id", taskID)
	td.markTaskPausing(t)
	return nil
}

func (td *Taskd) ResumeTask(taskID string) (*task.Task, error) {
	select {
	case <-td.Stopping():
		return nil, ErrStopping
	default:
	}

	isPausing, t := td.unmarkTaskIfPausing(taskID)
	if !isPausing {
		return nil, ErrTaskNotPausing
	}

	td.Info("resuming task", "task_id", taskID)
	newT, _, err := td.createInitSubmit(t.Cfg, false, true, []task.Hook{
		func(newT *task.Task, err error, hed *task.HookExtraData) {
			for i, newStep := range newT.Steps() {
				data := t.Steps()[i].LoadAll()
				for k, v := range data {
					newStep.Store(k, v)
				}
			}
			for i, newStep := range newT.DeferSteps() {
				data := t.DeferSteps()[i].LoadAll()
				for k, v := range data {
					newStep.Store(k, v)
				}
			}
		},
	})

	if err != nil {
		td.Error("resume task fail", err, "task_id", taskID)
		td.markTaskPausing(t)
		td.unmarkTaskAndTaskID(t.Cfg.ID)
		return newT, err
	}

	return newT, nil
}

func (td *Taskd) waitAllTaskDone() {
	for _, t := range td.allTask() {
		<-t.Done()
	}
}

func (td *Taskd) createInit(taskCfg *task.Cfg) (*task.Task, error) {
	err := v.Struct(taskCfg)
	if err != nil {
		return nil, errs.Wrap(err, "invalid task cfg")
	}

	t, err := td.createTask(taskCfg)
	td.hookTask(t, err, td.createHooks, "create", nil)
	if err != nil {
		td.Error("create task fail", err, "cfg", taskCfg)
		return t, errs.Wrap(err, "create task fail")
	}

	t.RegisterStepDoneHook(td.stepDoneHooks...)
	t.RegisterDeferStepDoneHook(td.deferStepDoneHooks...)

	err = td.initTask(t)
	td.hookTask(t, err, td.initHooks, "init", nil)
	if err != nil {
		td.Error("init task fail", err, "cfg", taskCfg)
		return t, errs.Wrap(err, "init task fail")
	}

	return t, nil
}

func (td *Taskd) submit(t *task.Task, wait bool, must bool) bool {
	td.Info("submitting task", "task_id", t.Cfg.ID, "wait", wait, "must", must)

	f := func() {
		td.markTaskRunning(t.Cfg.ID)
		td.hookTask(t, nil, td.startHooks, "start", nil)

		td.Info("task starting", "task_id", t.Cfg.ID)
		runner.Start(t)
		td.hookTask(t, nil, td.doneHooks, "done", nil)
		td.unmarkTaskAndTaskID(t.Cfg.ID)

		err := t.Err()
		if err != nil {
			td.Error("task fail", err, "task_id", t.Cfg.ID, "task_type", t.Cfg.Type)
		} else {
			td.Info("task done", "task_id", t.Cfg.ID, "task_type", t.Cfg.Type)
		}
	}

	var submitted bool
	td.markTask(t)
	if !must {
		submitted = td.pool.TrySubmit(f)
		if !submitted {
			td.Warn("try submit fail, full queue and no idle worker", "task_id", t.Cfg.ID)
			td.unmarkTaskAndTaskID(t.Cfg.ID)
		} else {
			td.Info("task submitted", "task_id", t.Cfg.ID)
		}
	} else {
		if wait {
			td.Info("task submit and wait", "task_id", t.Cfg.ID)
			td.pool.SubmitAndWait(f)
			td.Info("task submit and wait done", "task_id", t.Cfg.ID)
		} else {
			td.pool.Submit(f)
			td.Info("task submitted", "task_id", t.Cfg.ID)
		}
		submitted = true
	}
	td.hookTask(t, nil, td.submitHooks, "submit", &task.HookExtraData{Submitted: submitted, SubmitWait: wait})

	return submitted
}

func (td *Taskd) createInitSubmit(taskCfg *task.Cfg, wait bool, must bool, beforeSubmit []task.Hook) (*task.Task, bool, error) {
	select {
	case <-td.Stopping():
		return nil, false, ErrStopping
	default:
	}

	marked := td.markTaskID(taskCfg.ID)
	if !marked {
		td.Warn("task already exists", "id", taskCfg.ID)
		return nil, false, ErrTaskAlreadyExists
	}

	t, err := td.createInit(taskCfg)
	if err != nil {
		td.unmarkTaskID(taskCfg.ID)
		return nil, false, errs.Wrap(err, "create init task fail")
	}

	select {
	case <-td.Stopping():
		return nil, false, ErrStopping
	default:
	}

	for _, h := range beforeSubmit {
		h(t, nil, nil)
	}

	submitted := td.submit(t, wait, must)
	return t, submitted, nil
}

func (td *Taskd) createTask(cfg *task.Cfg) (t *task.Task, err error) {
	defer func() {
		e := recover()
		if e != nil {
			err = errs.PanicToErr(e)
		}
	}()
	return plugin.CreateWithCfg(task.PluginTypeTask, cfg).(*task.Task), nil
}

func (td *Taskd) initTask(t *task.Task) (err error) {
	defer func() {
		e := recover()
		if e != nil {
			err = errs.PanicToErr(e)
		}
	}()

	t.Inherit(td)
	t.WithLoggerFields("task_id", t.Cfg.ID, "task_type", t.Cfg.Type)
	return runner.Init(t)
}

func (td *Taskd) markTaskID(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDMap[taskID]
	if exists {
		return false
	}
	_, exists = td.taskPausingMap[taskID]
	if exists {
		return false
	}

	td.taskIDMap[taskID] = struct{}{}
	return true
}

func (td *Taskd) unmarkTaskID(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDMap[taskID]
	if !exists {
		return false
	}
	delete(td.taskIDMap, taskID)
	return true
}

func (td *Taskd) unmarkTaskAndTaskID(taskID string) {
	td.mu.Lock()
	defer td.mu.Unlock()
	delete(td.taskIDMap, taskID)
	delete(td.taskMap, taskID)
}

func (td *Taskd) markTask(t *task.Task) {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.taskMap[t.Cfg.ID] = t
}

func (td *Taskd) markTaskRunning(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDRunningMap[taskID]
	if exists {
		return false
	}
	td.taskIDRunningMap[taskID] = struct{}{}
	return true
}

func (td *Taskd) markTaskPausing(t *task.Task) {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.taskPausingMap[t.Cfg.ID] = t
}

func (td *Taskd) unmarkTaskIfPausing(taskID string) (bool, *task.Task) {
	td.mu.Lock()
	defer td.mu.Unlock()
	t, exists := td.taskPausingMap[taskID]
	if !exists {
		return false, t
	}
	delete(td.taskPausingMap, taskID)
	return true, t
}

func (td *Taskd) allTask() []*task.Task {
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

func (td *Taskd) hookTask(t *task.Task, err error, hooks []task.Hook, hookType string, extra *task.HookExtraData) {
	for i, h := range hooks {
		func(idx int, h task.Hook) {
			defer func() {
				err := recover()
				if err != nil {
					td.Error("hook task panic", errs.PanicToErr(err), "idx", idx, "hook", reflects.GetFuncName(h), "hook_type", hookType)
				}
			}()
			h(t, err, extra)
		}(i, h)
	}
}

func (td *Taskd) getTask(taskID string) *task.Task {
	td.mu.Lock()
	defer td.mu.Unlock()
	return td.taskMap[taskID]
}

func (td *Taskd) IsTaskExists(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDMap[taskID]
	if exists {
		return true
	}
	_, exists = td.taskPausingMap[taskID]
	return exists
}

func (td *Taskd) IsTaskWaiting(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDMap[taskID]
	if !exists {
		return false
	}
	_, isRunning := td.taskIDRunningMap[taskID]
	_, isPausing := td.taskPausingMap[taskID]
	return !isRunning && !isPausing
}

func (td *Taskd) IsTaskRunning(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDRunningMap[taskID]
	return exists
}

func (td *Taskd) IsTaskPausing(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskPausingMap[taskID]
	return exists
}

func (td *Taskd) ListTaskIDs() []string {
	td.mu.Lock()
	defer td.mu.Unlock()
	ids := make([]string, len(td.taskIDMap))
	i := 0
	for id := range td.taskIDMap {
		ids[i] = id
		i++
	}
	return ids
}

func (td *Taskd) ListWaitingTaskIDs() []string {
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

func (td *Taskd) ListRunningTaskIDs() []string {
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

func (td *Taskd) ListPausingTaskIDs() []string {
	td.mu.Lock()
	defer td.mu.Unlock()
	ids := make([]string, len(td.taskPausingMap))
	i := 0
	for id := range td.taskPausingMap {
		ids[i] = id
		i++
	}
	return ids
}

func (td *Taskd) GetTaskResult(taskID string) (interface{}, error) {
	t := td.getTask(taskID)
	if t == nil {
		return nil, ErrTaskNotExists
	}

	return t.Result(), nil
}

func (td *Taskd) OnTaskCreate(hooks ...task.Hook) {
	td.createHooks = append(td.createHooks, hooks...)
}

func (td *Taskd) OnTaskInit(hooks ...task.Hook) {
	td.initHooks = append(td.initHooks, hooks...)
}

func (td *Taskd) OnTaskSubmit(hooks ...task.Hook) {
	td.submitHooks = append(td.submitHooks, hooks...)
}

func (td *Taskd) OnTaskStart(hooks ...task.Hook) {
	td.startHooks = append(td.startHooks, hooks...)
}

func (td *Taskd) OnTaskDone(hooks ...task.Hook) {
	td.doneHooks = append(td.doneHooks, hooks...)
}

func (td *Taskd) OnTaskStepDone(hooks ...task.StepHook) {
	td.stepDoneHooks = append(td.stepDoneHooks, hooks...)
}

func (td *Taskd) OnTaskDeferStepDone(hooks ...task.StepHook) {
	td.deferStepDoneHooks = append(td.deferStepDoneHooks, hooks...)
}
