package task

import (
	"time"

	"github.com/donkeywon/golib/common"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util"
)

func init() {
	plugin.Register(PluginTypeTask, func() interface{} { return New() })
	plugin.RegisterCfg(PluginTypeTask, func() interface{} { return NewCfg() })
}

func RegisterCollector(typ Type, c Collector) {
	_collectors[typ] = c
}

var _collectors = make(map[Type]Collector)

const PluginTypeTask plugin.Type = "task"

type Type string

type Collector func(*Task) interface{}

type StepDoneHook func(*Task, int, Step)

type StepCfg struct {
	Type StepType    `json:"type" validate:"required" yaml:"type"`
	Cfg  interface{} `json:"cfg"  validate:"required" yaml:"cfg"`
}

type Cfg struct {
	ID              string     `json:"id"              validate:"required,min=1" yaml:"id"`
	Type            Type       `json:"type"            yaml:"type"`
	Steps           []*StepCfg `json:"steps"           validate:"required,min=1" yaml:"steps"`
	DeferSteps      []*StepCfg `json:"deferSteps"      yaml:"deferSteps"`
	CurStepIdx      int        `json:"curStepIdx"      yaml:"curStepIdx"`
	CurDeferStepIdx int        `json:"curDeferStepIdx" yaml:"curDeferStepIdx"`
}

func NewCfg() *Cfg {
	return &Cfg{}
}

func (c *Cfg) SetID(id string) *Cfg {
	c.ID = id
	return c
}

func (c *Cfg) SetType(t Type) *Cfg {
	c.Type = t
	return c
}

func (c *Cfg) Add(typ StepType, cfg interface{}) *Cfg {
	c.Steps = append(c.Steps, &StepCfg{Type: typ, Cfg: cfg})
	return c
}

func (c *Cfg) Defer(typ StepType, cfg interface{}) *Cfg {
	c.DeferSteps = append(c.DeferSteps, &StepCfg{Type: typ, Cfg: cfg})
	return c
}

type Snapshot struct {
	Cfg              *Cfg                `json:"cfg"              yaml:"cfg"`
	StepsResult      []map[string]string `json:"stepsResult"      yaml:"stepsResult"`
	DeferStepsResult []map[string]string `json:"deferStepsResult" yaml:"deferStepsResult"`
}

type Task struct {
	runner.Runner
	*Cfg

	stepDoneHooks      []StepDoneHook
	deferStepDoneHooks []StepDoneHook

	steps      []Step
	deferSteps []Step
}

func New() *Task {
	return &Task{
		Runner: runner.NewBase("task"),
		Cfg:    NewCfg(),
	}
}

func (t *Task) Init() error {
	err := util.V.Struct(t)
	if err != nil {
		return err
	}

	for i, cfg := range t.Cfg.Steps {
		step := plugin.CreateWithCfg(cfg.Type, cfg.Cfg).(Step)
		step.SetTask(t)
		step.SetCtx(t.Ctx())
		step.WithLogger(t.Logger(), "step", i, "step_type", step.Type())
		t.steps = append(t.steps, step)
	}

	for i, cfg := range t.Cfg.DeferSteps {
		step := plugin.CreateWithCfg(cfg.Type, cfg.Cfg).(Step)
		step.SetTask(t)
		step.SetCtx(t.Ctx())
		step.WithLogger(t.Logger(), "defer_step", i, "defer_step_type", step.Type())
		t.deferSteps = append(t.deferSteps, step)
	}

	for i := t.Cfg.CurStepIdx; i < len(t.steps); i++ {
		err = runner.Init(t.steps[i])
		if err != nil {
			return errs.Wrapf(err, "init step(%d) %s fail", i, t.steps[i].Type())
		}
	}

	for i := len(t.Cfg.DeferSteps) - 1 - t.Cfg.CurDeferStepIdx; i >= 0; i-- {
		err = runner.Init(t.deferSteps[i])
		if err != nil {
			return errs.Wrapf(err, "init defer step(%d) %s fail", i, t.deferSteps[i].Type())
		}
	}

	return nil
}

func (t *Task) Start() error {
	defer t.final()
	defer t.runDeferSteps()
	defer t.recoverStepPanic()

	t.Store(common.FieldStartTimeNano, time.Now().Unix())
	t.runSteps()

	return nil
}

func (t *Task) Stop() error {
	runner.Stop(t.CurStep())
	return nil
}

func (t *Task) RegisterStepDoneHook(hook ...StepDoneHook) {
	t.stepDoneHooks = append(t.stepDoneHooks, hook...)
}

func (t *Task) RegisterDeferStepDoneHook(hook ...StepDoneHook) {
	t.deferStepDoneHooks = append(t.deferStepDoneHooks, hook...)
}

func (t *Task) Result() interface{} {
	c := _collectors[t.Cfg.Type]
	if c == nil {
		return t.Snapshot()
	}
	return c(t)
}

func (t *Task) Snapshot() *Snapshot {
	s := &Snapshot{
		Cfg: t.Cfg,
	}
	for _, step := range t.Steps() {
		s.StepsResult = append(s.StepsResult, step.Collect())
	}
	for _, deferStep := range t.DeferSteps() {
		s.DeferStepsResult = append(s.DeferStepsResult, deferStep.Collect())
	}
	return s
}

func (t *Task) Steps() []Step {
	return t.steps
}

func (t *Task) CurStep() Step {
	return t.Steps()[t.CurStepIdx]
}

func (t *Task) DeferSteps() []Step {
	return t.deferSteps
}

func (t *Task) CurDeferStep() Step {
	return t.DeferSteps()[t.CurDeferStepIdx]
}

func (t *Task) recoverStepPanic() {
	err := recover()
	if err != nil {
		t.AppendError(errs.Wrapf(errs.Errorf("%+v", err), "step %d(%s) panic", t.CurStepIdx, t.CurStep().Type()))
	}
}

func (t *Task) final() {
	t.Store(common.FieldStopTimeNano, time.Now().Unix())
}

func (t *Task) runSteps() {
	for ; t.CurStepIdx < len(t.Steps()); t.CurStepIdx++ {
		select {
		case <-t.Stopping():
			return
		default:
			step := t.Steps()[t.CurStepIdx]
			step.Store(common.FieldStartTimeNano, time.Now().UnixNano())
			runner.Start(step)
			step.Store(common.FieldStopTimeNano, time.Now().UnixNano())
			for _, hook := range t.stepDoneHooks {
				hook(t, t.CurStepIdx, step)
			}
			err := step.Error()
			if err != nil {
				t.AppendError(errs.Wrapf(err, "run step %s(%d) fail", step.Type(), t.CurStepIdx))
				return
			}
		}
	}
}

func (t *Task) runDeferSteps() {
	for ; t.CurDeferStepIdx < len(t.DeferSteps()); t.CurDeferStepIdx++ {
		deferStep := t.deferSteps[len(t.deferSteps)-1-t.CurDeferStepIdx]
		select {
		case <-t.Stopping():
			return
		default:
			func() {
				defer func() {
					err := recover()
					if err != nil {
						t.AppendError(errs.Errorf("defer step(%d) panic: %+v", t.CurDeferStepIdx, err))
					}
				}()

				deferStep.Store(common.FieldStartTimeNano, time.Now().Unix())
				runner.Start(deferStep)
				deferStep.Store(common.FieldStopTimeNano, time.Now().Unix())
				for _, hook := range t.deferStepDoneHooks {
					hook(t, t.CurDeferStepIdx, deferStep)
				}
				err := deferStep.Error()
				if err != nil {
					t.AppendError(errs.Wrapf(err, "run defer step %s(%d) fail", deferStep.Type(), t.CurDeferStepIdx))
				}
			}()
		}
	}
}
