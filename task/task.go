package task

import (
	"context"
	"fmt"
	"time"

	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/kvs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/task/step"
	"github.com/donkeywon/golib/util/reflects"
	"github.com/donkeywon/golib/util/v"
)

func init() {
	plugin.Reg(PluginTypeTask, New, func() any { return NewCfg() })
}

const PluginTypeTask plugin.Type = "task"

type Type string

type Collector func(*Task) any

type StepHook func(*Task, int, step.Step)

type Hook func(*Task, error, *HookExtraData)

type HookExtraData struct {
	Wait bool
}

type Cfg struct {
	ID         string      `json:"id"          validate:"required" yaml:"id"`
	Steps      []*step.Cfg `json:"steps"       validate:"required" yaml:"steps"`
	DeferSteps []*step.Cfg `json:"defer_steps"                     yaml:"deferSteps"`
	Skip       int         `json:"skip"        validate:"min=0"    yaml:"skip"`
	DeferSkip  int         `json:"defer_skip"  validate:"min=0"    yaml:"deferSkip"`
}

func NewCfg() *Cfg {
	return &Cfg{}
}

func (c *Cfg) SetID(id string) *Cfg {
	c.ID = id
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
	Data           map[string]any   `json:"data"             yaml:"data"`
	StepsData      []map[string]any `json:"steps_data"       yaml:"stepsData"`
	DeferStepsData []map[string]any `json:"defer_steps_data" yaml:"deferStepsData"`
}

type Task struct {
	kvs.Map[string, any]
	runner.Base

	cfg *Cfg

	stepDoneHooks      []StepHook
	deferStepDoneHooks []StepHook

	steps      []step.Step
	deferSteps []step.Step

	cancel context.CancelFunc
}

func New() *Task {
	return &Task{}
}

func (t *Task) SetCfg(cfg any) {
	t.cfg = cfg.(*Cfg)
}

func (t *Task) Init(ctx context.Context) error {
	err := v.Struct(t)
	if err != nil {
		return err
	}

	for _, cfg := range t.cfg.Steps {
		step := plugin.CreateWithCfg[step.Step](cfg.Type, cfg.Cfg)
		t.steps = append(t.steps, step)
	}

	for _, cfg := range t.cfg.DeferSteps {
		step := plugin.CreateWithCfg[step.Step](cfg.Type, cfg.Cfg)
		t.deferSteps = append(t.deferSteps, step)
	}

	for i := t.cfg.Skip; i < len(t.steps); i++ {
		err = runner.Init(ctx, t.steps[i])
		if err != nil {
			return errs.Wrapf(err, "init step failed: %s(%d)", t.cfg.Steps[i].Type, i)
		}
	}

	for i := len(t.cfg.DeferSteps) - 1 - t.cfg.DeferSkip; i >= 0; i-- {
		err = runner.Init(ctx, t.deferSteps[i])
		if err != nil {
			return errs.Wrapf(err, "init defer step failed: %s(%d)", t.cfg.DeferSteps[i].Type, i)
		}
	}

	return nil
}

func (t *Task) Start(ctx context.Context) error {
	ctx, t.cancel = context.WithCancel(ctx)

	defer t.final()
	defer t.runDeferSteps()
	defer t.recoverStepPanic()

	t.Store(consts.FieldStartTimeNano, time.Now().UnixNano())
	t.runSteps()

	return nil
}

func (t *Task) Stop(ctx context.Context) error {
	t.cancel()
	return nil
}

func (t *Task) HookStepDone(hook ...StepHook) {
	t.stepDoneHooks = append(t.stepDoneHooks, hook...)
}

func (t *Task) HookDeferStepDone(hook ...StepHook) {
	t.deferStepDoneHooks = append(t.deferStepDoneHooks, hook...)
}

func (t *Task) Result() *Result {
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

func (t *Task) DeferSteps() []step.Step {
	return t.deferSteps
}

func (t *Task) CurDeferStep() step.Step {
	return t.DeferSteps()[t.cfg.CurDeferStepIdx]
}

func (t *Task) Store(k string, v any) {
	t.Runner.StoreAsString(k, v)
}

func (t *Task) recoverStepPanic() {
	err := recover()
	if err != nil {
		t.AppendError(errs.PanicToErrWithMsg(err, fmt.Sprintf("step(%d) %s panic", t.CurStepIdx, t.Steps()[t.CurStepIdx].Name())))
	}
}

func (t *Task) final() {
	t.Store(consts.FieldStopTimeNano, time.Now().UnixNano())
}

func (t *Task) runSteps() {
	for t.CurStepIdx < len(t.Steps()) {
		select {
		case <-t.Stopping():
			return
		default:
		}

		st := t.Steps()[t.CurStepIdx]
		st.Store(consts.FieldStartTimeNano, time.Now().UnixNano())
		err := runner.Run(st)
		st.Store(consts.FieldStopTimeNano, time.Now().UnixNano())
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
						t.Error("hook step panic", errs.PanicToErr(err), "hook_idx", idx, "hook", reflects.GetFuncName(h), "step_idx", t.CurStepIdx, "step_type", st.Name())
					}
				}()
				h(t, t.CurStepIdx, st)
			}(i, hook)
		}
		if err != nil {
			t.AppendError(errs.Wrapf(err, "run step(%d) %s failed", t.CurStepIdx, st.Name()))
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
			err := runner.Run(deferStep)
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
			if err != nil {
				t.AppendError(errs.Wrapf(err, "run defer(%d) step %s failed", t.CurDeferStepIdx, deferStep.Name()))
			}
		}()
	}
}
