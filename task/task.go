package task

import (
	"fmt"
	"time"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/conv"
	"github.com/donkeywon/golib/util/reflects"
	"github.com/donkeywon/golib/util/v"
)

func init() {
	plugin.RegWithCfg(PluginTypeTask, func() any { return New() }, func() any { return NewCfg() })
}

const PluginTypeTask plugin.Type = "task"

type Type string

type Collector func(*Task) any

type StepHook func(*Task, int, Step)

type Hook func(*Task, error, *HookExtraData)

type HookExtraData struct {
	Submitted  bool
	SubmitWait bool
}

type StepCfg struct {
	Type StepType `json:"type" validate:"required" yaml:"type"`
	Cfg  any      `json:"cfg"  validate:"required" yaml:"cfg"`
}

type Cfg struct {
	ID              string     `json:"id"              validate:"required" yaml:"id"`
	Type            Type       `json:"type"            validate:"required" yaml:"type"`
	Steps           []*StepCfg `json:"steps"           validate:"required" yaml:"steps"`
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

func (c *Cfg) Add(typ StepType, cfg any) *Cfg {
	c.Steps = append(c.Steps, &StepCfg{Type: typ, Cfg: cfg})
	return c
}

func (c *Cfg) Defer(typ StepType, cfg any) *Cfg {
	c.DeferSteps = append(c.DeferSteps, &StepCfg{Type: typ, Cfg: cfg})
	return c
}

type Result struct {
	Data           map[string]any   `json:"data"           yaml:"data"`
	StepsData      []map[string]any `json:"stepsData"      yaml:"stepsData"`
	DeferStepsData []map[string]any `json:"deferStepsData" yaml:"deferStepsData"`
}

type Task struct {
	runner.Runner
	plugin.Plugin
	*Cfg

	stepDoneHooks      []StepHook
	deferStepDoneHooks []StepHook

	steps      []Step
	deferSteps []Step

	collector Collector
}

func New() *Task {
	return &Task{
		Runner: runner.Create("task"),
		Cfg:    NewCfg(),
	}
}

func (t *Task) Init() error {
	err := v.Struct(t)
	if err != nil {
		return err
	}

	for i, cfg := range t.Cfg.Steps {
		step, er := t.createStep(i, cfg, false)
		if er != nil {
			return errs.Wrapf(er, "create step(%d) %s failed", i, cfg.Type)
		}
		t.steps = append(t.steps, step)
	}

	for i, cfg := range t.Cfg.DeferSteps {
		step, er := t.createStep(i, cfg, true)
		if er != nil {
			return errs.Wrapf(er, "create defer step(%d) %s failed", i, cfg.Type)
		}
		t.deferSteps = append(t.deferSteps, step)
	}

	for i := t.Cfg.CurStepIdx; i < len(t.steps); i++ {
		err = runner.Init(t.steps[i])
		if err != nil {
			return errs.Wrapf(err, "init step(%d) %s failed", i, t.steps[i].Type())
		}
	}

	for i := len(t.Cfg.DeferSteps) - 1 - t.Cfg.CurDeferStepIdx; i >= 0; i-- {
		err = runner.Init(t.deferSteps[i])
		if err != nil {
			return errs.Wrapf(err, "init defer step(%d) %s failed", i, t.deferSteps[i].Type())
		}
	}

	return t.Runner.Init()
}

func (t *Task) Start() error {
	defer t.final()
	defer t.runDeferSteps()
	defer t.recoverStepPanic()

	t.Store(consts.FieldStartTimeNano, time.Now().Unix())
	t.runSteps()

	return nil
}

func (t *Task) Stop() error {
	runner.Stop(t.CurStep())
	return nil
}

func (t *Task) RegisterStepDoneHook(hook ...StepHook) {
	t.stepDoneHooks = append(t.stepDoneHooks, hook...)
}

func (t *Task) RegisterDeferStepDoneHook(hook ...StepHook) {
	t.deferStepDoneHooks = append(t.deferStepDoneHooks, hook...)
}

func (t *Task) RegisterCollector(c Collector) {
	t.collector = c
}

func (t *Task) Result() any {
	if t.collector == nil {
		return t.result()
	}
	return t.collector(t)
}

func (t *Task) result() *Result {
	r := &Result{}
	for _, step := range t.Steps() {
		v := step.LoadAll()
		r.StepsData = append(r.StepsData, v)
	}
	for _, deferStep := range t.DeferSteps() {
		v := deferStep.LoadAll()
		r.DeferStepsData = append(r.DeferStepsData, v)
	}
	return r
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

func (t *Task) Store(k string, v any) {
	t.Runner.StoreAsString(k, v)
}

func (t *Task) Type() any {
	return PluginTypeTask
}

func (t *Task) GetCfg() any {
	return t.Cfg
}

func (t *Task) createStep(idx int, cfg *StepCfg, isDefer bool) (Step, error) {
	var (
		err         error
		stepOrDefer string
	)
	if isDefer {
		stepOrDefer = "defer_step"
	} else {
		stepOrDefer = "step"
	}
	stepCfg := plugin.CreateCfg(cfg.Type)
	err = conv.ConvertOrMerge(stepCfg, cfg.Cfg)
	if err != nil {
		return nil, errs.Wrapf(err, "invalid %s(%s) cfg", stepOrDefer, cfg.Type)
	}

	step := plugin.CreateWithCfg(cfg.Type, stepCfg).(Step)
	step.SetTask(t)
	step.Inherit(t)
	step.WithLoggerFields(stepOrDefer, idx, stepOrDefer+"_type", step.Type())
	return step, nil
}

func (t *Task) recoverStepPanic() {
	err := recover()
	if err != nil {
		t.AppendError(errs.PanicToErrWithMsg(err, fmt.Sprintf("step(%d) %s panic", t.CurStepIdx, t.CurStep().Type())))
	}
}

func (t *Task) final() {
	t.Store(consts.FieldStopTimeNano, time.Now().Unix())
}

func (t *Task) runSteps() {
	for t.CurStepIdx < len(t.Steps()) {
		select {
		case <-t.Stopping():
			return
		default:
		}

		step := t.Steps()[t.CurStepIdx]
		step.Store(consts.FieldStartTimeNano, time.Now().UnixNano())
		runner.Start(step)
		step.Store(consts.FieldStopTimeNano, time.Now().UnixNano())
		select {
		case <-t.Stopping():
			return
		default:
			t.CurStepIdx++
		}

		for i, hook := range t.stepDoneHooks {
			func(idx int, h StepHook) {
				defer func() {
					err := recover()
					if err != nil {
						t.Error("hook step panic", errs.PanicToErr(err), "hook_idx", idx, "hook", reflects.GetFuncName(h), "step_idx", t.CurStepIdx, "step_type", step.Type())
					}
				}()
				h(t, t.CurStepIdx, step)
			}(i, hook)
		}
		err := step.Err()
		if err != nil {
			t.AppendError(errs.Wrapf(err, "run step(%d) %s failed", t.CurStepIdx, step.Type()))
			return
		}
	}
}

func (t *Task) runDeferSteps() {
	for t.CurDeferStepIdx < len(t.DeferSteps()) {
		select {
		case <-t.Stopping():
			return
		default:
		}

		deferStep := t.deferSteps[len(t.deferSteps)-1-t.CurDeferStepIdx]
		func() {
			defer func() {
				err := recover()
				if err != nil {
					t.AppendError(errs.PanicToErrWithMsg(err, fmt.Sprintf("defer step(%d) %s panic", t.CurDeferStepIdx, t.CurDeferStep().Type())))
				}
			}()

			deferStep.Store(consts.FieldStartTimeNano, time.Now().Unix())
			runner.Start(deferStep)
			deferStep.Store(consts.FieldStopTimeNano, time.Now().Unix())
			select {
			case <-t.Stopping():
				return
			default:
				t.CurDeferStepIdx++
			}

			for i, hook := range t.deferStepDoneHooks {
				func(idx int, h StepHook) {
					defer func() {
						err := recover()
						if err != nil {
							t.Error("hook defer step panic", errs.PanicToErr(err), "hook_idx", idx, "hook", reflects.GetFuncName(h), "step_idx", t.CurDeferStepIdx, "step_type", deferStep.Type())
						}
					}()
					h(t, t.CurDeferStepIdx, deferStep)
				}(i, hook)
			}
			err := deferStep.Err()
			if err != nil {
				t.AppendError(errs.Wrapf(err, "run defer(%d) step %s failed", t.CurDeferStepIdx, deferStep.Type()))
			}
		}()
	}
}
