package task

import (
	"fmt"
	"time"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/task/step"
	"github.com/donkeywon/golib/util/reflects"
	"github.com/donkeywon/golib/util/v"
)

func init() {
	plugin.RegWithCfg(PluginTypeTask, New, func() any { return NewCfg() })
}

const PluginTypeTask plugin.Type = "task"

type Type string

type Collector func(*Task) any

type StepHook func(*Task, int, step.Step)

type Hook func(*Task, error, *HookExtraData)

type HookExtraData struct {
	Submitted  bool
	SubmitWait bool
}

type Cfg struct {
	ID              string      `json:"id"              validate:"required" yaml:"id"`
	Type            Type        `json:"type"            validate:"required" yaml:"type"`
	Steps           []*step.Cfg `json:"steps"           validate:"required" yaml:"steps"`
	DeferSteps      []*step.Cfg `json:"deferSteps"      yaml:"deferSteps"`
	CurStepIdx      int         `json:"curStepIdx"      yaml:"curStepIdx"`
	CurDeferStepIdx int         `json:"curDeferStepIdx" yaml:"curDeferStepIdx"`
	Pool            string      `json:"pool"            yaml:"pool"`
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

func (c *Cfg) Add(typ step.Type, cfg any) *Cfg {
	c.Steps = append(c.Steps, &step.Cfg{Type: typ, Cfg: cfg})
	return c
}

func (c *Cfg) Defer(typ step.Type, cfg any) *Cfg {
	c.DeferSteps = append(c.DeferSteps, &step.Cfg{Type: typ, Cfg: cfg})
	return c
}

type Result struct {
	Data           map[string]any   `json:"data"           yaml:"data"`
	StepsData      []map[string]any `json:"stepsData"      yaml:"stepsData"`
	DeferStepsData []map[string]any `json:"deferStepsData" yaml:"deferStepsData"`
}

type Task struct {
	runner.Runner
	*Cfg

	stepDoneHooks      []StepHook
	deferStepDoneHooks []StepHook

	steps      []step.Step
	deferSteps []step.Step

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
		step := t.createStep(i, cfg, false)
		t.steps = append(t.steps, step)
	}

	for i, cfg := range t.Cfg.DeferSteps {
		step := t.createStep(i, cfg, true)
		t.deferSteps = append(t.deferSteps, step)
	}

	for i := t.Cfg.CurStepIdx; i < len(t.steps); i++ {
		err = runner.Init(t.steps[i])
		if err != nil {
			return errs.Wrapf(err, "init step(%d) %s failed", i, t.steps[i].Name())
		}
	}

	for i := len(t.Cfg.DeferSteps) - 1 - t.Cfg.CurDeferStepIdx; i >= 0; i-- {
		err = runner.Init(t.deferSteps[i])
		if err != nil {
			return errs.Wrapf(err, "init defer step(%d) %s failed", i, t.deferSteps[i].Name())
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

func (t *Task) HookStepDone(hook ...StepHook) {
	t.stepDoneHooks = append(t.stepDoneHooks, hook...)
}

func (t *Task) HookDeferStepDone(hook ...StepHook) {
	t.deferStepDoneHooks = append(t.deferStepDoneHooks, hook...)
}

func (t *Task) SetCollector(c Collector) {
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

func (t *Task) Steps() []step.Step {
	return t.steps
}

func (t *Task) CurStep() step.Step {
	return t.Steps()[t.CurStepIdx]
}

func (t *Task) DeferSteps() []step.Step {
	return t.deferSteps
}

func (t *Task) CurDeferStep() step.Step {
	return t.DeferSteps()[t.CurDeferStepIdx]
}

func (t *Task) Store(k string, v any) {
	t.Runner.StoreAsString(k, v)
}

func (t *Task) createStep(idx int, stepCfg *step.Cfg, isDefer bool) step.Step {
	var (
		stepOrDefer string
	)
	if isDefer {
		stepOrDefer = "defer_step"
	} else {
		stepOrDefer = "step"
	}

	s := plugin.CreateWithCfg[step.Type, step.Step](stepCfg.Type, stepCfg.Cfg)
	s.Inherit(t)
	s.SetParent(t)
	s.WithLoggerFields(stepOrDefer, idx, stepOrDefer+"_type", s.Name())
	return s
}

func (t *Task) recoverStepPanic() {
	err := recover()
	if err != nil {
		t.AppendError(errs.PanicToErrWithMsg(err, fmt.Sprintf("step(%d) %s panic", t.CurStepIdx, t.CurStep().Name())))
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
						t.Error("hook step panic", errs.PanicToErr(err), "hook_idx", idx, "hook", reflects.GetFuncName(h), "step_idx", t.CurStepIdx, "step_type", step.Name())
					}
				}()
				h(t, t.CurStepIdx, step)
			}(i, hook)
		}
		err := step.Err()
		if err != nil {
			t.AppendError(errs.Wrapf(err, "run step(%d) %s failed", t.CurStepIdx, step.Name()))
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
					t.AppendError(errs.PanicToErrWithMsg(err, fmt.Sprintf("defer step(%d) %s panic", t.CurDeferStepIdx, t.CurDeferStep().Name())))
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
							t.Error("hook defer step panic", errs.PanicToErr(err), "hook_idx", idx, "hook", reflects.GetFuncName(h), "step_idx", t.CurDeferStepIdx, "step_type", deferStep.Name())
						}
					}()
					h(t, t.CurDeferStepIdx, deferStep)
				}(i, hook)
			}
			err := deferStep.Err()
			if err != nil {
				t.AppendError(errs.Wrapf(err, "run defer(%d) step %s failed", t.CurDeferStepIdx, deferStep.Name()))
			}
		}()
	}
}
