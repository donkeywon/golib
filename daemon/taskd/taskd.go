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
	"github.com/donkeywon/golib/util"
)

const DaemonTypeTaskd boot.DaemonType = "taskd"

var (
	ErrStopping          = errors.New("stopping, reject")
	ErrTaskAlreadyExists = errors.New("task already exists")
)

var _t = &Taskd{
	Runner:        runner.Create(string(DaemonTypeTaskd)),
	taskMap:       make(map[string]*task.Task),
	taskIDMarkMap: make(map[string]struct{}),
}

type Taskd struct {
	runner.Runner
	*Cfg

	pool *pond.WorkerPool

	mu               sync.Mutex
	taskIDMarkMap    map[string]struct{}
	taskIDRunningMap map[string]struct{}
	taskIDPausingMap map[string]struct{}
	taskMap          map[string]*task.Task

	createHooks        []task.Hook
	initHooks          []task.Hook
	submitHooks        []task.Hook
	startHooks         []task.Hook
	doneHooks          []task.Hook
	stepDoneHooks      []task.StepHook
	deferStepDoneHooks []task.StepHook
}

func New() *Taskd {
	return _t
}

func (td *Taskd) Init() error {
	td.pool = pond.New(td.Cfg.PoolSize, td.Cfg.QueueSize)
	return td.Runner.Init()
}

func (td *Taskd) Start() error {
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

func (td *Taskd) Submit(taskCfg *task.Cfg) (*task.Task, error) {
	t, _, err := td.submit(taskCfg, false, true)
	return t, err
}

func (td *Taskd) SubmitAndWait(taskCfg *task.Cfg) (*task.Task, error) {
	t, _, err := td.submit(taskCfg, true, true)
	return t, err
}

func (td *Taskd) TrySubmit(taskCfg *task.Cfg) (*task.Task, bool, error) {
	t, submitted, err := td.submit(taskCfg, false, false)
	return t, submitted, err
}

func (td *Taskd) waitAllTaskDone() {
	for _, t := range td.allTask() {
		<-t.Done()
	}
}

func (td *Taskd) submit(taskCfg *task.Cfg, wait bool, must bool) (*task.Task, bool, error) {
	select {
	case <-td.Stopping():
		return nil, false, ErrStopping
	default:
	}

	err := util.V.Struct(taskCfg)
	if err != nil {
		return nil, false, errs.Wrap(err, "invalid task cfg")
	}

	td.Info("receive task", "cfg", taskCfg)

	marked := td.markTaskID(taskCfg.ID)
	if !marked {
		td.Warn("task already exists", "id", taskCfg.ID)
		return nil, false, ErrTaskAlreadyExists
	}

	t, err := td.createTask(taskCfg)
	td.hookCreate(t, err)
	if err != nil {
		td.unmarkTaskID(taskCfg.ID)
		td.Error("create task fail", err, "cfg", taskCfg)
		return t, false, errs.Wrap(err, "create task fail")
	}

	t.RegisterStepDoneHook(td.stepDoneHooks...)
	t.RegisterDeferStepDoneHook(td.deferStepDoneHooks...)

	err = td.initTask(t)
	td.hookInit(t, err)
	if err != nil {
		td.unmarkTaskID(taskCfg.ID)
		td.Error("init task fail", err, "cfg", taskCfg)
		return t, false, errs.Wrap(err, "init task fail")
	}

	f := func() {
		td.hookStart(t)
		go td.listenTask(t)
		runner.Start(t)
	}

	var submitted bool
	if !must {
		submitted = td.pool.TrySubmit(f)
		if !submitted {
			td.unmarkTaskID(taskCfg.ID)
		} else {
			td.markTask(t)
		}
	} else {
		td.markTask(t)
		if wait {
			td.pool.SubmitAndWait(f)
		} else {
			td.pool.Submit(f)
		}
		submitted = true
	}
	td.hookSubmit(t, submitted, wait)

	return t, submitted, t.Err()
}

func (td *Taskd) createTask(cfg *task.Cfg) (t *task.Task, err error) {
	defer func() {
		e := recover()
		if e != nil {
			err = errs.Errorf("%v", e)
		}
	}()
	return plugin.CreateWithCfg(task.PluginTypeTask, cfg).(*task.Task), nil
}

func (td *Taskd) initTask(t *task.Task) (err error) {
	defer func() {
		e := recover()
		if e != nil {
			err = errs.Errorf("%v", e)
		}
	}()

	t.Inherit(td)
	t.WithLoggerFields("task_id", t.Cfg.ID, "task_type", t.Cfg.Type)
	return runner.Init(t)
}

func (td *Taskd) markTaskID(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDMarkMap[taskID]
	if exists {
		return false
	}
	td.taskIDMarkMap[taskID] = struct{}{}
	return true
}

func (td *Taskd) unmarkTaskID(taskID string) {
	td.mu.Lock()
	defer td.mu.Unlock()
	delete(td.taskIDMarkMap, taskID)
}

func (td *Taskd) markTask(t *task.Task) {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.taskMap[t.Cfg.ID] = t
}

func (td *Taskd) unmarkTask(t *task.Task) {
	td.mu.Lock()
	defer td.mu.Unlock()
	delete(td.taskMap, t.Cfg.ID)
}

func (td *Taskd) markTaskRunning(taskID string) {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.taskIDRunningMap[taskID] = struct{}{}
}

func (td *Taskd) unmarkTaskRunning(taskID string) {
	td.mu.Lock()
	defer td.mu.Unlock()
	delete(td.taskIDRunningMap, taskID)
}

func (td *Taskd) unmarkTaskRunningAndMarkTaskPausing(taskID string) {
	td.mu.Lock()
	defer td.mu.Unlock()
	delete(td.taskIDRunningMap, taskID)
	td.taskIDPausingMap[taskID] = struct{}{}
}

func (td *Taskd) unmarkTaskPausing(taskID string) {
	td.mu.Lock()
	defer td.mu.Unlock()
	delete(td.taskIDPausingMap, taskID)
}

func (td *Taskd) unmarkTaskPausingAndMarkTaskRunning(taskID string) {
	td.mu.Lock()
	defer td.mu.Unlock()
	delete(td.taskIDPausingMap, taskID)
	td.taskIDRunningMap[taskID] = struct{}{}
}

func (td *Taskd) allTask() []*task.Task {
	td.mu.Lock()
	defer td.mu.Unlock()
	tasks := make([]*task.Task, 0, len(td.taskMap))
	for _, t := range td.taskMap {
		tasks = append(tasks, t)
	}
	return tasks
}

func (td *Taskd) listenTask(t *task.Task) {
	<-t.Done()
	td.hookDone(t)
	td.Info("listen done by task done", "task_id", t.Cfg.ID)
	td.unmarkTaskID(t.Cfg.ID)
	td.unmarkTask(t)
}

func (td *Taskd) hookCreate(t *task.Task, err error) {
	for _, h := range td.createHooks {
		h(t, err, nil)
	}
}

func (td *Taskd) hookInit(t *task.Task, err error) {
	for _, h := range td.initHooks {
		h(t, err, nil)
	}
}

func (td *Taskd) hookSubmit(t *task.Task, submitted bool, wait bool) {
	for _, h := range td.submitHooks {
		h(t, nil, &task.HookExtraData{Submitted: submitted, SubmitWait: wait})
	}
}

func (td *Taskd) hookStart(t *task.Task) {
	for _, h := range td.startHooks {
		h(t, nil, nil)
	}
}

func (td *Taskd) hookDone(t *task.Task) {
	for _, h := range td.doneHooks {
		h(t, t.Err(), nil)
	}
}

func (td *Taskd) HasTask(taskID string) bool {
	td.mu.Lock()
	defer td.mu.Unlock()
	_, exists := td.taskIDMarkMap[taskID]
	return exists
}

func Submit(cfg *task.Cfg) (*task.Task, error) {
	return _t.Submit(cfg)
}

func SubmitAndWait(cfg *task.Cfg) (*task.Task, error) {
	return _t.SubmitAndWait(cfg)
}

func TrySubmit(cfg *task.Cfg) (*task.Task, bool, error) {
	return _t.TrySubmit(cfg)
}

func HasTask(taskID string) bool {
	return _t.HasTask(taskID)
}

func OnTaskCreate(hooks ...task.Hook) {
	_t.createHooks = append(_t.createHooks, hooks...)
}

func OnTaskInit(hooks ...task.Hook) {
	_t.initHooks = append(_t.initHooks, hooks...)
}

func OnTaskSubmit(hooks ...task.Hook) {
	_t.submitHooks = append(_t.submitHooks, hooks...)
}

func OnTaskStart(hooks ...task.Hook) {
	_t.startHooks = append(_t.startHooks, hooks...)
}

func OnTaskDone(hooks ...task.Hook) {
	_t.doneHooks = append(_t.doneHooks, hooks...)
}

func OnTaskStepDone(hooks ...task.StepHook) {
	_t.stepDoneHooks = append(_t.stepDoneHooks, hooks...)
}

func OnTaskDeferStepDone(hooks ...task.StepHook) {
	_t.deferStepDoneHooks = append(_t.deferStepDoneHooks, hooks...)
}
