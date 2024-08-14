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
	Runner:      runner.Create(string(DaemonTypeTaskd)),
	taskMap:     &sync.Map{},
	taskMarkMap: &sync.Map{},
}

type Taskd struct {
	runner.Runner
	*Cfg

	pool        *pond.WorkerPool
	taskMarkMap *sync.Map
	taskMap     *sync.Map
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
	var allTask []*task.Task
	td.taskMap.Range(func(_, value any) bool {
		allTask = append(allTask, value.(*task.Task))
		return true
	})
	for _, t := range allTask {
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
	if err != nil {
		td.unmarkTaskID(taskCfg.ID)
		td.Error("create task fail", err, "cfg", taskCfg)
		return t, false, errs.Wrap(err, "create task fail")
	}

	err = td.initTask(t)
	if err != nil {
		td.unmarkTaskID(taskCfg.ID)
		td.Error("init task fail", err, "cfg", taskCfg)
		return t, false, errs.Wrap(err, "init task fail")
	}

	f := func() {
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
	_, loaded := td.taskMarkMap.LoadOrStore(taskID, struct{}{})
	return !loaded
}

func (td *Taskd) unmarkTaskID(taskID string) {
	td.taskMarkMap.Delete(taskID)
}

func (td *Taskd) markTask(t *task.Task) {
	td.taskMap.Store(t.Cfg.ID, t)
}

func (td *Taskd) unmarkTask(t *task.Task) {
	td.taskMap.Delete(t.Cfg.ID)
}

func (td *Taskd) listenTask(t *task.Task) {
	<-t.Done()
	td.Info("listen done by task done", "task_id", t.Cfg.ID)
	td.unmarkTaskID(t.Cfg.ID)
	td.unmarkTask(t)
}

func (td *Taskd) HasTask(taskID string) bool {
	_, ok := td.taskMarkMap.Load(taskID)
	return ok
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
