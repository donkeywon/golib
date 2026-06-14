package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/kvs"
	"github.com/donkeywon/golib/logs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/task/step"
	"github.com/donkeywon/golib/util/reflects"
	"github.com/donkeywon/golib/util/v"
)

var (
	ErrSkip = errors.New("skip")
)

func init() {
	plugin.Reg(PluginTypeTask, New, func() any { return NewCfg() })
}

const PluginTypeTask plugin.Type = "task"

type Type string

type StepHook func(context.Context, *Task, int, step.Step, error) error

type Cfg struct {
	ID         string     `json:"id"          validate:"required" yaml:"id"`
	Steps      []step.Cfg `json:"steps"       validate:"required" yaml:"steps"`
	DeferSteps []step.Cfg `json:"defer_steps"                     yaml:"deferSteps"`
}

func NewCfg() Cfg {
	return Cfg{}
}

func (c *Cfg) SetID(id string) *Cfg {
	c.ID = id
	return c
}

func (c *Cfg) Add(typ step.Type, cfg any) *Cfg {
	c.Steps = append(c.Steps, step.Cfg{Type: typ, Cfg: cfg})
	return c
}

func (c *Cfg) Defer(typ step.Type, cfg any) *Cfg {
	c.DeferSteps = append(c.DeferSteps, step.Cfg{Type: typ, Cfg: cfg})
	return c
}

type Task struct {
	kvs.Map[string, any]
	runner.Base

	cfg Cfg

	beforeStepRunHooks      []StepHook
	afterStepDoneHooks      []StepHook
	beforeDeferStepRunHooks []StepHook
	afterDeferStepDoneHooks []StepHook

	steps      []step.Step
	deferSteps []step.Step

	l      *slog.Logger
	cancel context.CancelFunc
}

func New() *Task {
	return &Task{}
}

func (t *Task) SetCfg(cfg any) {
	t.cfg = cfg.(Cfg)
}

func (t *Task) Cfg() Cfg {
	return t.cfg
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

	return nil
}

func (t *Task) Start(ctx context.Context) (err error) {
	t.l = logs.FromCtx(ctx)
	ctx, t.cancel = context.WithCancel(ctx)

	defer func() {
		derr := t.runDeferSteps(ctx)
		if derr != nil {
			err = errors.Join(err, derr)
		}
	}()
	return t.runSteps(ctx)
}

func (t *Task) Stop(ctx context.Context) error {
	t.cancel()
	return nil
}

func (t *Task) BeforeStepRun(hook ...StepHook) {
	t.beforeStepRunHooks = append(t.beforeStepRunHooks, hook...)
}

func (t *Task) AfterStepDone(hook ...StepHook) {
	t.afterStepDoneHooks = append(t.afterStepDoneHooks, hook...)
}

func (t *Task) BeforeDeferStepRun(hook ...StepHook) {
	t.beforeDeferStepRunHooks = append(t.beforeDeferStepRunHooks, hook...)
}

func (t *Task) AfterDeferStepDone(hook ...StepHook) {
	t.afterDeferStepDoneHooks = append(t.afterDeferStepDoneHooks, hook...)
}

func (t *Task) Steps() []step.Step {
	return t.steps
}

func (t *Task) DeferSteps() []step.Step {
	return t.deferSteps
}

func (t *Task) runSteps(ctx context.Context) error {
	var err error
	for i := 0; i < len(t.steps); i++ {
		select {
		case <-t.Stopping():
			return nil
		default:
		}

		typ := t.cfg.Steps[i].Type
		st := t.steps[i]

		err = hookStep(ctx, t.beforeStepRunHooks, i, typ, st, nil, t, false)
		if err != nil {
			if errors.Is(err, ErrSkip) {
				t.l.Info("skip step", "hook_err", err, "step_idx", i, "step_type", typ)
				continue
			}
			return errs.Wrapf(err, "hook before run step failed: %s(%d)", typ, i)
		}

		err = t.runStep(ctx, i, typ, st, false)
		if err != nil {
			err = errs.Wrapf(err, "run step faild: %s(%d)", typ, i)
		}

		herr := hookStep(ctx, t.afterStepDoneHooks, i, typ, st, err, t, false)
		if herr == nil {
			if err != nil {
				return err
			}
			continue
		}

		if errors.Is(herr, ErrSkip) {
			if err != nil {
				t.l.Info("skip step err", "hook_err", herr, "step_err", err, "step_idx", i, "step_type", typ)
			}
			continue
		}

		herr = errs.Wrapf(herr, "hook after step done failed: %s(%d)", typ, i)
		if err == nil {
			return herr
		} else {
			return errors.Join(err, herr)
		}
	}
	return nil
}

func (t *Task) runDeferSteps(ctx context.Context) error {
	allErr := make([]error, 0, len(t.cfg.DeferSteps))
	for i := len(t.cfg.DeferSteps) - 1; i >= 0; i-- {
		select {
		case <-t.Stopping():
			return nil
		default:
		}

		typ := t.cfg.DeferSteps[i].Type
		st := t.deferSteps[i]

		err := hookStep(ctx, t.beforeDeferStepRunHooks, i, typ, st, nil, t, true)
		if err != nil {
			if errors.Is(err, ErrSkip) {
				t.l.Info("skip defer step", "hook_err", err, "defer_step_idx", i, "defer_step_type", typ)
				continue
			}
			allErr = append(allErr, errs.Wrapf(err, "hook before run defer step failed: %s(%d)", typ, i))
		}

		err = t.runStep(ctx, i, typ, st, true)
		if err != nil {
			err = errs.Wrapf(err, "run defer step failed: %s(%d)", typ, i)
		}

		herr := hookStep(ctx, t.afterDeferStepDoneHooks, i, typ, st, err, t, true)
		if herr == nil {
			if err != nil {
				allErr = append(allErr, err)
			}
			continue
		}

		if errors.Is(herr, ErrSkip) {
			if err != nil {
				t.l.Info("skip defer step err", "hook_err", herr, "defer_step_err", err, "defer_step_idx", i, "defer_step_type", typ)
			}
			continue
		}

		herr = errs.Wrapf(herr, "hook after defer step done failed: %s(%d)", typ, i)
		if err == nil {
			allErr = append(allErr, herr)
		} else {
			allErr = append(allErr, err, herr)
		}

	}
	return errors.Join(allErr...)
}

func (t *Task) runStep(ctx context.Context, i int, typ step.Type, st step.Step, isDefer bool) (err error) {
	stepMsgName := "step"
	if isDefer {
		stepMsgName = "defer step"
	}
	defer func() {
		p := recover()
		if p != nil {
			err = errors.Join(err, errs.PanicToErrWithMsg(err, fmt.Sprintf("panic on run %s: %s(%d)", stepMsgName, typ, i)))
		}
	}()

	err = runner.Init(ctx, st)
	if err != nil {
		return errs.Wrapf(err, "init %s failed: %s(%d)", stepMsgName, typ, i)
	}

	err = runner.Start(ctx, st)
	if err != nil {
		return errs.Wrapf(err, "start %s failed: %s(%d)", stepMsgName, typ, i)
	}
	return nil
}

func hookStep(ctx context.Context, hooks []StepHook, stepIdx int, typ step.Type, st step.Step, stepErr error, t *Task, isDefer bool) (err error) {
	allErr := make([]error, 0, len(hooks))
	for i, h := range hooks {
		err := hook(ctx, i, h, stepIdx, typ, st, stepErr, t, isDefer)
		if err != nil {
			allErr = append(allErr, err)
		}
	}
	return errors.Join(allErr...)
}

func hook(ctx context.Context, hookIdx int, h StepHook, stepIdx int, typ step.Type, st step.Step, stepErr error, t *Task, isDefer bool) (err error) {
	stepMsgName := "step"
	if isDefer {
		stepMsgName = "defer step"
	}

	defer func() {
		p := recover()
		if p == nil {
			return
		}

		pe := errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on hook %s: %s(%d) %s(%d)", stepMsgName, typ, stepIdx, reflects.GetFuncName(h), hookIdx))
		if err == nil {
			err = pe
		} else {
			err = errors.Join(err, pe)
		}
	}()
	return h(ctx, t, stepIdx, st, stepErr)
}
