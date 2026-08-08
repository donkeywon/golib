package task

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/kvs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/task/step"
	"github.com/donkeywon/golib/util/reflects"
	"github.com/rs/zerolog"
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

type initializer interface {
	Init(context.Context) error
}

type Cfg struct {
	ID         string     `json:"id"          validate:"required" yaml:"id"`
	Steps      []step.Cfg `json:"steps"       validate:"required" yaml:"steps"`
	DeferSteps []step.Cfg `json:"defer_steps"                     yaml:"deferSteps"`
}

func NewCfg() Cfg {
	return Cfg{}
}

func (c Cfg) SetID(id string) Cfg {
	c.ID = id
	return c
}

func (c Cfg) Add(typ step.Type, cfg any) Cfg {
	c.Steps = append(c.Steps, step.Cfg{Type: typ, Cfg: cfg})
	return c
}

func (c Cfg) Defer(typ step.Type, cfg any) Cfg {
	c.DeferSteps = append(c.DeferSteps, step.Cfg{Type: typ, Cfg: cfg})
	return c
}

type Task struct {
	kvs.Map[string, any]

	cfg Cfg

	beforeStepRunHooks      []StepHook
	afterStepDoneHooks      []StepHook
	beforeDeferStepRunHooks []StepHook
	afterDeferStepDoneHooks []StepHook

	steps      []step.Step
	deferSteps []step.Step

	l *zerolog.Logger
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

func (t *Task) Run(ctx context.Context) (err error) {
	t.l = zerolog.Ctx(ctx)

	for _, cfg := range t.cfg.Steps {
		step := plugin.CreateWithCfg[step.Step](cfg.Type, cfg.Cfg)
		t.steps = append(t.steps, step)
	}

	for _, cfg := range t.cfg.DeferSteps {
		step := plugin.CreateWithCfg[step.Step](cfg.Type, cfg.Cfg)
		t.deferSteps = append(t.deferSteps, step)
	}

	defer func() {
		derr := t.runDeferSteps(ctx)
		if derr != nil {
			err = errors.Join(err, derr)
		}
	}()
	return t.runSteps(ctx)
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
	return slices.Clone(t.steps)
}

func (t *Task) DeferSteps() []step.Step {
	return slices.Clone(t.deferSteps)
}

func (t *Task) runSteps(ctx context.Context) error {
	var err error
	for i, st := range t.steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		typ := t.cfg.Steps[i].Type

		err = hookStep(ctx, t.beforeStepRunHooks, i, typ, st, nil, t, false)
		if err != nil {
			if errors.Is(err, ErrSkip) {
				t.l.Info().Err(err).Int("step_idx", i).Str("step_type", string(typ)).Msg("skip step")
				continue
			}
			return errs.Wrapf(err, "hook before run step failed: %s(%d)", typ, i)
		}

		err = t.runStep(ctx, i, typ, st, false)
		if err != nil {
			err = errs.Wrapf(err, "run step failed: %s(%d)", typ, i)
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
				t.l.Info().AnErr("hook_err", herr).AnErr("step_err", err).Int("step_idx", i).Str("step_type", string(typ)).Msg("skip step err")
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
	allErr := make([]error, 0, len(t.deferSteps))
	for i, st := range slices.Backward(t.deferSteps) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		typ := t.cfg.DeferSteps[i].Type
		err := hookStep(ctx, t.beforeDeferStepRunHooks, i, typ, st, nil, t, true)
		if err != nil {
			if errors.Is(err, ErrSkip) {
				t.l.Info().Err(err).Int("defer_step_idx", i).Str("defer_step_type", string(typ)).Msg("skip defer step")
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
				t.l.Info().AnErr("hook_err", herr).AnErr("defer_step_err", err).Int("defer_step_idx", i).Str("defer_step_type", string(typ)).Msg("skip defer step err")
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
			err = errors.Join(err, errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on run %s: %s(%d)", stepMsgName, typ, i)))
		}
	}()

	if initer, ok := st.(initializer); ok {
		err = initer.Init(ctx)
		if err != nil {
			return errs.Wrapf(err, "init %s failed: %s(%d)", stepMsgName, typ, i)
		}
	}

	err = st.Run(ctx)
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
		err = errors.Join(err, pe)
	}()
	return h(ctx, t, stepIdx, st, stepErr)
}
