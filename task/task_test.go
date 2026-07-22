package task

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/donkeywon/golib/logs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/task/step"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Cfg tests
// =============================================================================

func TestNewCfg(t *testing.T) {
	c := NewCfg()
	require.NotNil(t, c)
	assert.Empty(t, c.ID)
	assert.Nil(t, c.Steps)
	assert.Nil(t, c.DeferSteps)
}

func TestCfg_SetID(t *testing.T) {
	c := NewCfg()
	result := c.SetID("test-id")
	assert.Equal(t, "test-id", c.ID)
	// SetID returns *Cfg for chaining
	assert.Equal(t, &c, result)
}

func TestCfg_Add(t *testing.T) {
	c := NewCfg()
	result := c.Add(step.TypeCmd, nil)
	assert.Len(t, c.Steps, 1)
	assert.Equal(t, step.TypeCmd, c.Steps[0].Type)
	assert.Equal(t, &c, result)

	c.Add("custom_type", "some_cfg")
	assert.Len(t, c.Steps, 2)
	assert.Equal(t, step.Type("custom_type"), c.Steps[1].Type)
	assert.Equal(t, "some_cfg", c.Steps[1].Cfg)
}

func TestCfg_Defer(t *testing.T) {
	c := NewCfg()
	result := c.Defer(step.TypeCmd, nil)
	assert.Len(t, c.DeferSteps, 1)
	assert.Equal(t, step.TypeCmd, c.DeferSteps[0].Type)
	assert.Equal(t, &c, result)

	c.Defer("defer_type", "defer_cfg")
	assert.Len(t, c.DeferSteps, 2)
	assert.Equal(t, step.Type("defer_type"), c.DeferSteps[1].Type)
}

// =============================================================================
// Task construction and configuration tests
// =============================================================================

func TestNew(t *testing.T) {
	tsk := New()
	require.NotNil(t, tsk)
	// Hook slices are nil by default (zero value)
	assert.Nil(t, tsk.beforeStepRunHooks)
	assert.Nil(t, tsk.afterStepDoneHooks)
	assert.Nil(t, tsk.beforeDeferStepRunHooks)
	assert.Nil(t, tsk.afterDeferStepDoneHooks)
}

func TestTask_SetCfg(t *testing.T) {
	task := New()
	cfg := Cfg{ID: "test-task"}
	task.SetCfg(cfg)
	assert.Equal(t, "test-task", task.cfg.ID)

	// SetCfg does a value type assertion — wrong type panics
	assert.Panics(t, func() {
		task.SetCfg("not a Cfg")
	})
}

func TestTask_Cfg(t *testing.T) {
	task := New()
	cfg := Cfg{ID: "my-task"}
	task.SetCfg(cfg)
	assert.Equal(t, "my-task", task.Cfg().ID)
}

func TestTask_BeforeStepRun(t *testing.T) {
	task := New()
	assert.Empty(t, task.beforeStepRunHooks)

	called := false
	hook := func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		called = true
		return nil
	}
	task.BeforeStepRun(hook)
	assert.Len(t, task.beforeStepRunHooks, 1)

	// Calling the hook
	_ = task.beforeStepRunHooks[0](context.Background(), task, 0, nil, nil)
	assert.True(t, called)
}

func TestTask_AfterStepDone(t *testing.T) {
	task := New()
	called := false
	hook := func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		called = true
		return nil
	}
	task.AfterStepDone(hook)
	assert.Len(t, task.afterStepDoneHooks, 1)
	_ = task.afterStepDoneHooks[0](context.Background(), task, 0, nil, nil)
	assert.True(t, called)
}

func TestTask_BeforeDeferStepRun(t *testing.T) {
	task := New()
	called := false
	hook := func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		called = true
		return nil
	}
	task.BeforeDeferStepRun(hook)
	assert.Len(t, task.beforeDeferStepRunHooks, 1)
	_ = task.beforeDeferStepRunHooks[0](context.Background(), task, 0, nil, nil)
	assert.True(t, called)
}

func TestTask_AfterDeferStepDone(t *testing.T) {
	task := New()
	called := false
	hook := func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		called = true
		return nil
	}
	task.AfterDeferStepDone(hook)
	assert.Len(t, task.afterDeferStepDoneHooks, 1)
	_ = task.afterDeferStepDoneHooks[0](context.Background(), task, 0, nil, nil)
	assert.True(t, called)
}

func TestTask_MultipleHooks(t *testing.T) {
	task := New()
	order := make([]int, 0)

	task.BeforeStepRun(
		func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
			order = append(order, 1)
			return nil
		},
		func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
			order = append(order, 2)
			return nil
		},
	)
	assert.Len(t, task.beforeStepRunHooks, 2)

	for _, h := range task.beforeStepRunHooks {
		_ = h(context.Background(), task, 0, nil, nil)
	}
	assert.Equal(t, []int{1, 2}, order)
}

func TestTask_Steps(t *testing.T) {
	task := New()
	assert.Nil(t, task.Steps())

	task.steps = []step.Step{}
	assert.NotNil(t, task.Steps())
	assert.Empty(t, task.Steps())
}

func TestTask_DeferSteps(t *testing.T) {
	task := New()
	assert.Nil(t, task.DeferSteps())

	task.deferSteps = []step.Step{}
	assert.NotNil(t, task.DeferSteps())
	assert.Empty(t, task.DeferSteps())
}

func TestTask_Init(t *testing.T) {
	// Register a test step type
	testType := step.Type("test_init_step")
	plugin.Reg(testType, func() step.Step { return &testStep{} }, func() any { return &testStepCfg{} })

	task := New()
	cfg := NewCfg()
	cfg.SetID("init-test")
	cfg.Add(testType, &testStepCfg{name: "step1"})
	task.SetCfg(cfg)

	err := task.Init(context.Background())
	require.NoError(t, err)
	assert.Len(t, task.steps, 1)

	// Init with defer steps too
	task2 := New()
	cfg2 := NewCfg()
	cfg2.SetID("init-test-2")
	cfg2.Add(testType, &testStepCfg{name: "main"})
	cfg2.Defer(testType, &testStepCfg{name: "defer"})
	task2.SetCfg(cfg2)

	err = task2.Init(context.Background())
	require.NoError(t, err)
	assert.Len(t, task2.steps, 1)
	assert.Len(t, task2.deferSteps, 1)
}

func TestTask_Init_UnregisteredType(t *testing.T) {
	task := New()
	cfg := NewCfg()
	cfg.SetID("bad-init")
	cfg.Add("nonexistent_type", nil)
	task.SetCfg(cfg)

	assert.Panics(t, func() {
		_ = task.Init(context.Background())
	})
}

func TestTask_Stop(t *testing.T) {
	task := New()
	// Stop on a Task that hasn't been started should still work
	// because the cancel func is nil and Stop will panic
	assert.Panics(t, func() {
		_ = task.Stop(context.Background())
	})
}

func TestErrSkip(t *testing.T) {
	assert.Equal(t, "skip", ErrSkip.Error())
	// ErrSkip is a sentinel error
	assert.Error(t, ErrSkip)
}

func TestTask_PluginReg(t *testing.T) {
	tsk := plugin.CreateWithCfg[*Task](PluginTypeTask, NewCfg())
	require.NotNil(t, tsk)
	require.IsType(t, &Task{}, tsk)
}

// =============================================================================
// testStep implementation
// =============================================================================

type testStepCfg struct {
	name string
}

type testStep struct {
	step.Base
	cfg          testStepCfg
	inited       bool
	started      bool
	initErr      error
	startErr     error
	stopErr      error
	panicOnStart bool
	blocking     bool
	startedCh    chan struct{}
	doneCh       chan struct{}
	onStart      func()
}

func (s *testStep) SetCfg(cfg any) {
	switch c := cfg.(type) {
	case testStepCfg:
		s.cfg = c
	case *testStepCfg:
		s.cfg = *c
	default:
		panic("unexpected type")
	}
}

func (s *testStep) Init(ctx context.Context) error {
	s.inited = true
	return s.initErr
}

func (s *testStep) Start(ctx context.Context) error {
	if s.onStart != nil {
		s.onStart()
	}
	if s.panicOnStart {
		panic("start panic")
	}
	s.started = true
	if s.startedCh != nil {
		close(s.startedCh)
	}
	defer func() {
		if s.doneCh != nil {
			close(s.doneCh)
		}
	}()
	if s.startErr != nil {
		return s.startErr
	}
	if s.blocking {
		<-s.Stopping()
	}
	return nil
}

func (s *testStep) Stop(ctx context.Context) error {
	return s.stopErr
}

// =============================================================================
// Helpers
// =============================================================================

// newTestTask creates a Task initialized with the given steps and cfg types.
func newTestTask(steps []step.Step, stepTypes []step.Type) *Task {
	t := New()
	t.l = slog.New(slog.DiscardHandler)
	cfgs := make([]step.Cfg, len(steps))
	for i := range steps {
		typ := step.Type("test")
		if i < len(stepTypes) {
			typ = stepTypes[i]
		}
		cfgs[i] = step.Cfg{Type: typ, Cfg: nil}
	}
	t.cfg = Cfg{ID: "test-task", Steps: cfgs}
	t.steps = steps
	return t
}

// newTestTaskWithDefer creates a Task initialized with the given defer steps.
func newTestTaskWithDefer(steps []step.Step) *Task {
	t := New()
	t.l = slog.New(slog.DiscardHandler)
	cfgs := make([]step.Cfg, len(steps))
	for i := range steps {
		cfgs[i] = step.Cfg{Type: "test", Cfg: nil}
	}
	t.cfg = Cfg{ID: "test-task", DeferSteps: cfgs}
	t.deferSteps = steps
	return t
}

// newTestTaskWithBoth creates a Task with both regular and defer steps.
func newTestTaskWithBoth(steps, deferSteps []step.Step) *Task {
	t := New()
	t.l = slog.New(slog.DiscardHandler)
	stepCfgs := make([]step.Cfg, len(steps))
	for i := range steps {
		stepCfgs[i] = step.Cfg{Type: "test", Cfg: nil}
	}
	deferCfgs := make([]step.Cfg, len(deferSteps))
	for i := range deferSteps {
		deferCfgs[i] = step.Cfg{Type: "test", Cfg: nil}
	}
	t.cfg = Cfg{ID: "test-task", Steps: stepCfgs, DeferSteps: deferCfgs}
	t.steps = steps
	t.deferSteps = deferSteps
	return t
}

// newTS creates a fresh testStep with default zero values (non-blocking, no errors).
func newTS() *testStep {
	return &testStep{Base: step.Base{}}
}

// loggerCtx returns a context with a discard logger, as expected by Task.Start.
func loggerCtx() context.Context {
	return logs.CtxWith(context.Background(), slog.New(slog.DiscardHandler))
}

// =============================================================================
// hookStep tests
// =============================================================================

func TestHookStep(t *testing.T) {
	t.Run("no hooks returns nil", func(t *testing.T) {
		err := hookStep(context.Background(), nil, 0, "test", nil, nil, nil, false)
		require.NoError(t, err)
	})

	t.Run("hook returns error", func(t *testing.T) {
		expectedErr := errors.New("hook error")
		hooks := []StepHook{
			func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
				return expectedErr
			},
		}
		err := hookStep(context.Background(), hooks, 0, "test", nil, nil, nil, false)
		require.Error(t, err)
	})

	t.Run("multiple hooks, one fails", func(t *testing.T) {
		called := make([]bool, 2)
		hooks := []StepHook{
			func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
				called[0] = true
				return errors.New("first error")
			},
			func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
				called[1] = true
				return nil
			},
		}
		err := hookStep(context.Background(), hooks, 0, "test", nil, nil, nil, false)
		require.Error(t, err)
		// All hooks should have been called
		assert.True(t, called[0])
		assert.True(t, called[1])
	})

	t.Run("hook with ErrSkip", func(t *testing.T) {
		hooks := []StepHook{
			func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
				return ErrSkip
			},
		}
		err := hookStep(context.Background(), hooks, 0, "test", nil, nil, nil, false)
		// hookStep propagates errors via errors.Join, ErrSkip will be returned
		require.Error(t, err)
	})

	t.Run("defer hooks", func(t *testing.T) {
		hooks := []StepHook{
			func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
				return errors.New("defer hook err")
			},
		}
		err := hookStep(context.Background(), hooks, 0, "test", nil, nil, nil, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "defer hook err")
	})

	t.Run("empty hook slice", func(t *testing.T) {
		err := hookStep(context.Background(), []StepHook{}, 0, "test", nil, nil, nil, false)
		require.NoError(t, err)
	})
}

// =============================================================================
// hook tests
// =============================================================================

func TestHook(t *testing.T) {
	t.Run("successful hook", func(t *testing.T) {
		called := false
		h := func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
			called = true
			return nil
		}
		err := hook(context.Background(), 0, h, 0, "test", nil, nil, nil, false)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("hook returns error", func(t *testing.T) {
		expectedErr := errors.New("hook failed")
		h := func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
			return expectedErr
		}
		err := hook(context.Background(), 0, h, 0, "test", nil, nil, nil, false)
		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("hook panics", func(t *testing.T) {
		h := func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
			panic("boom")
		}
		err := hook(context.Background(), 0, h, 0, "test", nil, nil, nil, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "panic")
	})

	t.Run("hook panics with existing error", func(t *testing.T) {
		h := func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
			// The err parameter is the step error — not the hook's own return
			panic("boom")
		}
		// When the hook panics, the stepErr is not relevant since
		// the hook didn't return anything. hook recovers and wraps.
		err := hook(context.Background(), 0, h, 0, "test", nil, nil, nil, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "panic")
	})

	t.Run("defer hook panic message includes 'defer step'", func(t *testing.T) {
		h := func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
			panic("boom")
		}
		err := hook(context.Background(), 0, h, 0, "test", nil, nil, nil, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "defer step")
	})

	t.Run("defer hook success", func(t *testing.T) {
		called := false
		h := func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
			called = true
			return nil
		}
		err := hook(context.Background(), 0, h, 0, "test", nil, nil, nil, true)
		require.NoError(t, err)
		assert.True(t, called)
	})
}

// =============================================================================
// runStep tests
// =============================================================================

func TestRunStep_Success(t *testing.T) {
	st := newTS()
	tsk := newTestTask([]step.Step{st}, nil)

	err := tsk.runStep(context.Background(), 0, "test", st, false)
	assert.NoError(t, err)
	assert.True(t, st.inited)
	assert.True(t, st.started)
}

func TestRunStep_Defer_Success(t *testing.T) {
	st := newTS()
	tsk := newTestTask(nil, nil)

	err := tsk.runStep(context.Background(), 0, "test", st, true)
	assert.NoError(t, err)
	assert.True(t, st.inited)
	assert.True(t, st.started)
}

func TestRunStep_InitError(t *testing.T) {
	st := newTS()
	st.initErr = errors.New("init failed")
	tsk := newTestTask([]step.Step{st}, nil)

	err := tsk.runStep(context.Background(), 0, "badStep", st, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init step failed")
	assert.Contains(t, err.Error(), "init failed")
	assert.True(t, st.inited)
	assert.False(t, st.started)
}

func TestRunStep_InitError_Defer(t *testing.T) {
	st := newTS()
	st.initErr = errors.New("init failed")
	tsk := newTestTask(nil, nil)

	err := tsk.runStep(context.Background(), 0, "badStep", st, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init defer step failed")
	assert.Contains(t, err.Error(), "init failed")
}

func TestRunStep_StartError(t *testing.T) {
	st := newTS()
	st.startErr = errors.New("start failed")
	tsk := newTestTask([]step.Step{st}, nil)

	err := tsk.runStep(context.Background(), 0, "badStep", st, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start step failed")
	assert.Contains(t, err.Error(), "start failed")
	assert.True(t, st.inited)
	assert.True(t, st.started)
}

func TestRunStep_StartError_Defer(t *testing.T) {
	st := newTS()
	st.startErr = errors.New("start failed")
	tsk := newTestTask(nil, nil)

	err := tsk.runStep(context.Background(), 0, "badStep", st, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start defer step failed")
	assert.Contains(t, err.Error(), "start failed")
}

func TestRunStep_Panic(t *testing.T) {
	st := newTS()
	st.panicOnStart = true
	tsk := newTestTask([]step.Step{st}, nil)

	err := tsk.runStep(context.Background(), 0, "panicStep", st, false)
	require.Error(t, err)
	// runner.Start recovers the panic first, then runStep wraps the error
	assert.Contains(t, err.Error(), "panic on start runner")
	assert.Contains(t, err.Error(), "start step failed")
}

func TestRunStep_Panic_Defer(t *testing.T) {
	st := newTS()
	st.panicOnStart = true
	tsk := newTestTask(nil, nil)

	err := tsk.runStep(context.Background(), 0, "panicStep", st, true)
	require.Error(t, err)
	// runner.Start recovers the panic first, then runStep wraps with "defer step" prefix
	assert.Contains(t, err.Error(), "panic on start runner")
	assert.Contains(t, err.Error(), "start defer step failed")
}

func TestRunStep_Blocking_ContextDone(t *testing.T) {
	st := &testStep{
		Base:      step.Base{},
		blocking:  true,
		startedCh: make(chan struct{}),
	}
	tsk := newTestTask([]step.Step{st}, nil)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- tsk.runStep(ctx, 0, "test", st, false)
	}()

	// Wait for step to enter Start
	<-st.startedCh

	// Cancel context to unblock
	cancel()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("runStep did not return after context cancel")
	}
}

// =============================================================================
// runSteps tests
// =============================================================================

func TestRunSteps_EmptySteps(t *testing.T) {
	tsk := newTestTask(nil, nil)
	err := tsk.runSteps(context.Background())
	assert.NoError(t, err)
}

func TestRunSteps_SingleStep_Success(t *testing.T) {
	st := newTS()
	tsk := newTestTask([]step.Step{st}, nil)

	err := tsk.runSteps(context.Background())
	assert.NoError(t, err)
	assert.True(t, st.started)
}

func TestRunSteps_MultipleSteps_Success(t *testing.T) {
	order := make([]int, 0)
	st1 := &testStep{Base: step.Base{}, onStart: func() { order = append(order, 1) }}
	st2 := &testStep{Base: step.Base{}, onStart: func() { order = append(order, 2) }}
	st3 := &testStep{Base: step.Base{}, onStart: func() { order = append(order, 3) }}
	tsk := newTestTask([]step.Step{st1, st2, st3}, nil)

	err := tsk.runSteps(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, order)
}

func TestRunSteps_BeforeHook_ErrSkip(t *testing.T) {
	st := newTS()
	tsk := newTestTask([]step.Step{st}, nil)
	tsk.BeforeStepRun(func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		return ErrSkip
	})

	err := tsk.runSteps(context.Background())
	assert.NoError(t, err)
	assert.False(t, st.started, "step should be skipped, not started")
}

func TestRunSteps_BeforeHook_Error(t *testing.T) {
	st := newTS()
	tsk := newTestTask([]step.Step{st}, nil)
	hookErr := errors.New("before hook error")
	tsk.BeforeStepRun(func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		return hookErr
	})

	err := tsk.runSteps(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook before run step failed")
	assert.False(t, st.started, "step should not be started when before hook errors")
}

func TestRunSteps_StepError_AfterHookNil(t *testing.T) {
	st := newTS()
	st.startErr = errors.New("step start error")
	tsk := newTestTask([]step.Step{st}, nil)
	tsk.AfterStepDone(func(ctx context.Context, tk *Task, idx int, s step.Step, err error) error {
		assert.Error(t, err)
		return nil
	})

	err := tsk.runSteps(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run step faild")
}

func TestRunSteps_StepError_AfterHookError(t *testing.T) {
	st := newTS()
	st.startErr = errors.New("step start error")
	tsk := newTestTask([]step.Step{st}, nil)
	hookErr := errors.New("after hook error")
	tsk.AfterStepDone(func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		return hookErr
	})

	err := tsk.runSteps(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "step start error")
	assert.Contains(t, err.Error(), "after hook error")
}

func TestRunSteps_StepError_AfterHookErrSkip(t *testing.T) {
	st := newTS()
	st.startErr = errors.New("step start error")
	tsk := newTestTask([]step.Step{st}, nil)
	tsk.AfterStepDone(func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		return ErrSkip
	})

	// After-hook returns ErrSkip → skip the error, continue
	err := tsk.runSteps(context.Background())
	assert.NoError(t, err)
}

func TestRunSteps_StepSuccess_AfterHookError(t *testing.T) {
	st := newTS()
	tsk := newTestTask([]step.Step{st}, nil)
	hookErr := errors.New("after hook error")
	tsk.AfterStepDone(func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		return hookErr
	})

	err := tsk.runSteps(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook after step done failed")
	assert.True(t, st.started, "step should have been started")
}

func TestRunSteps_StepSuccess_AfterHookErrSkip(t *testing.T) {
	st := newTS()
	tsk := newTestTask([]step.Step{st}, nil)
	tsk.AfterStepDone(func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		return ErrSkip
	})

	err := tsk.runSteps(context.Background())
	assert.NoError(t, err)
	assert.True(t, st.started, "step should have been started")
}

// =============================================================================
// runDeferSteps tests
// =============================================================================

func TestRunDeferSteps_Empty(t *testing.T) {
	tsk := newTestTaskWithDefer(nil)
	err := tsk.runDeferSteps(context.Background())
	assert.NoError(t, err)
}

func TestRunDeferSteps_Success(t *testing.T) {
	st := newTS()
	tsk := newTestTaskWithDefer([]step.Step{st})

	err := tsk.runDeferSteps(context.Background())
	assert.NoError(t, err)
	assert.True(t, st.started)
}

func TestRunDeferSteps_ReverseOrder(t *testing.T) {
	order := make([]int, 0)
	st1 := &testStep{Base: step.Base{}, onStart: func() { order = append(order, 1) }}
	st2 := &testStep{Base: step.Base{}, onStart: func() { order = append(order, 2) }}
	st3 := &testStep{Base: step.Base{}, onStart: func() { order = append(order, 3) }}
	// Register in cfg order: 1, 2, 3 → defer runs: 3, 2, 1
	tsk := newTestTaskWithDefer([]step.Step{st1, st2, st3})

	err := tsk.runDeferSteps(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, []int{3, 2, 1}, order)
}

func TestRunDeferSteps_BeforeHook_ErrSkip(t *testing.T) {
	st := newTS()
	tsk := newTestTaskWithDefer([]step.Step{st})
	tsk.BeforeDeferStepRun(func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		return ErrSkip
	})

	err := tsk.runDeferSteps(context.Background())
	assert.NoError(t, err)
	assert.False(t, st.started, "step should be skipped")
}

func TestRunDeferSteps_BeforeHook_Error_Continues(t *testing.T) {
	st := newTS()
	tsk := newTestTaskWithDefer([]step.Step{st})
	hookErr := errors.New("before defer hook error")
	tsk.BeforeDeferStepRun(func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		return hookErr
	})

	// In runDeferSteps, before-hook error accumulates but step still runs
	err := tsk.runDeferSteps(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook before run defer step failed")
	assert.True(t, st.started, "step should still run after before-hook error")
}

func TestRunDeferSteps_StepError_Accumulates(t *testing.T) {
	st := newTS()
	st.startErr = errors.New("defer step error")
	tsk := newTestTaskWithDefer([]step.Step{st})

	err := tsk.runDeferSteps(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "defer step error")
}

func TestRunDeferSteps_StepError_AfterHookNil(t *testing.T) {
	st := newTS()
	st.startErr = errors.New("defer step error")
	tsk := newTestTaskWithDefer([]step.Step{st})
	tsk.AfterDeferStepDone(func(ctx context.Context, tk *Task, idx int, s step.Step, err error) error {
		assert.Error(t, err)
		return nil
	})

	err := tsk.runDeferSteps(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "defer step error")
}

func TestRunDeferSteps_StepError_AfterHookError(t *testing.T) {
	st := newTS()
	st.startErr = errors.New("defer step error")
	tsk := newTestTaskWithDefer([]step.Step{st})
	hookErr := errors.New("after defer hook error")
	tsk.AfterDeferStepDone(func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		return hookErr
	})

	err := tsk.runDeferSteps(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "defer step error")
	assert.Contains(t, err.Error(), "after defer hook error")
}

func TestRunDeferSteps_StepError_AfterHookErrSkip(t *testing.T) {
	st := newTS()
	st.startErr = errors.New("defer step error")
	tsk := newTestTaskWithDefer([]step.Step{st})
	tsk.AfterDeferStepDone(func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		return ErrSkip
	})

	// After-hook ErrSkip → step error is silently skipped (logged but not returned)
	err := tsk.runDeferSteps(context.Background())
	assert.NoError(t, err)
}

func TestRunDeferSteps_StepSuccess_AfterHookError(t *testing.T) {
	st := newTS()
	tsk := newTestTaskWithDefer([]step.Step{st})
	hookErr := errors.New("after defer hook error")
	tsk.AfterDeferStepDone(func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		return hookErr
	})

	err := tsk.runDeferSteps(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook after defer step done failed")
	assert.True(t, st.started)
}

func TestRunDeferSteps_StepSuccess_AfterHookErrSkip(t *testing.T) {
	st := newTS()
	tsk := newTestTaskWithDefer([]step.Step{st})
	tsk.AfterDeferStepDone(func(ctx context.Context, t *Task, idx int, s step.Step, err error) error {
		return ErrSkip
	})

	err := tsk.runDeferSteps(context.Background())
	assert.NoError(t, err)
	assert.True(t, st.started)
}

func TestRunDeferSteps_MultipleErrors_AllAccumulate(t *testing.T) {
	st1 := newTS()
	st1.startErr = errors.New("error1")
	st2 := newTS()
	st2.startErr = errors.New("error2")
	tsk := newTestTaskWithDefer([]step.Step{st1, st2})

	err := tsk.runDeferSteps(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error2") // st2 runs first (reverse order)
	assert.Contains(t, err.Error(), "error1")
}

// =============================================================================
// Start integration tests
// =============================================================================

func TestStart_DeferStepsRunOnSuccess(t *testing.T) {
	// Register a test step type for Init
	testType := step.Type("test_start_defer")
	plugin.Reg(testType, func() step.Step { return &testStep{} }, func() any { return &testStepCfg{} })

	s1 := newTS()
	s2 := newTS()

	task := newTestTaskWithBoth([]step.Step{s1}, []step.Step{s2})
	// Override cfg so Init can work with the registered plugin type
	task.cfg = Cfg{
		ID:         "test-task",
		Steps:      []step.Cfg{{Type: testType, Cfg: &testStepCfg{name: "s1"}}},
		DeferSteps: []step.Cfg{{Type: testType, Cfg: &testStepCfg{name: "s2"}}},
	}
	task.steps = []step.Step{s1}
	task.deferSteps = []step.Step{s2}

	ctx := loggerCtx()
	err := runner.Start(ctx, task)
	assert.NoError(t, err)

	assert.True(t, s1.started, "regular step should have started")
	assert.True(t, s2.started, "defer step should have started")
}

func TestStart_BlockingStep_StopUnblocks(t *testing.T) {
	st := &testStep{
		Base:      step.Base{},
		blocking:  true,
		startedCh: make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	task := newTestTask([]step.Step{st}, nil)

	ctx := loggerCtx()

	done := make(chan error, 1)
	go func() {
		done <- runner.Start(ctx, task)
	}()

	// Wait for step to start blocking
	<-st.startedCh

	// Stop the task (closes Stopping channel and cancels ctx)
	err := runner.Stop(ctx, task)
	require.NoError(t, err)

	// Wait for step to actually finish
	<-st.doneCh

	select {
	case err = <-done:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("runner.Start did not return after Stop")
	}
}

func TestStart_StopDuringDeferSteps(t *testing.T) {
	st1 := newTS()
	st2 := &testStep{
		Base:      step.Base{},
		blocking:  true,
		startedCh: make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	task := newTestTaskWithBoth([]step.Step{st1}, []step.Step{st2})

	ctx := loggerCtx()

	done := make(chan error, 1)
	go func() {
		done <- runner.Start(ctx, task)
	}()

	// Wait for defer step to start blocking
	<-st2.startedCh

	// Stop: this closes t.Stopping, which runDeferSteps checks at iteration top
	err := runner.Stop(ctx, task)
	require.NoError(t, err)

	// Wait for step to actually finish
	<-st2.doneCh

	select {
	case err = <-done:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("runner.Start did not return after Stop during defer")
	}
}

// TestRunStep_NilRunner triggers runStep's own panic recovery by passing nil to runner.Init
// (which panics before registering its own defer).
func TestRunStep_NilRunner(t *testing.T) {
	tsk := newTestTask(nil, nil)

	err := tsk.runStep(context.Background(), 0, "test", nil, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "panic on run step")
}

func TestRunStep_NilRunner_Defer(t *testing.T) {
	tsk := newTestTask(nil, nil)

	err := tsk.runStep(context.Background(), 0, "test", nil, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "panic on run defer step")
}

// TestStart_DeferStepsError tests the path where runSteps succeeds but runDeferSteps fails.
func TestStart_DeferStepsError(t *testing.T) {
	s1 := newTS()
	s2 := newTS()
	s2.startErr = errors.New("defer step error")

	task := newTestTaskWithBoth([]step.Step{s1}, []step.Step{s2})

	ctx := loggerCtx()
	err := runner.Start(ctx, task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "defer step error")
	assert.True(t, s1.started, "regular step should have started")
	assert.True(t, s2.started, "defer step should have been attempted")
}

// TestStart_StopSignalInRunSteps tests the Stopping signal path in runSteps.
// A blocking step is interrupted via Stop; after it unblocks, the next iteration
// finds Stopping is already closed and returns nil.
func TestStart_StopSignalInRunSteps(t *testing.T) {
	st1 := &testStep{
		Base:      step.Base{},
		blocking:  true,
		startedCh: make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	st2 := newTS()
	st3 := newTS()
	task := newTestTask([]step.Step{st1, st2, st3}, nil)

	ctx := loggerCtx()

	done := make(chan error, 1)
	go func() {
		done <- runner.Start(ctx, task)
	}()

	// Wait for step 1 to start blocking
	<-st1.startedCh

	// Stop the task: closes t.Stopping and cancels ctx
	err := runner.Stop(ctx, task)
	require.NoError(t, err)

	// Wait for step 1 to actually finish (context cancelled → unblocked → returned)
	<-st1.doneCh

	// Now runSteps has proceeded to the next iteration and hit the Stopping check
	// Read runner.Start result (should be done shortly after doneCh)
	select {
	case err = <-done:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("runner.Start did not return after Stop")
	}

	// Step 1 started, but step 2 should not have (Stopping prevented it)
	assert.True(t, st1.started)
	assert.False(t, st2.started, "step 2 should be skipped due to Stopping signal")
	assert.False(t, st3.started)
}

// TestStart_StopSignalInRunDeferSteps tests the Stopping signal path in runDeferSteps.
// Defer steps run in reverse order: d2 (index 1) first, then d1 (index 0).
// d2 blocks; after Stop unblocks it, the Stopping check prevents d1 from starting.
func TestStart_StopSignalInRunDeferSteps(t *testing.T) {
	s1 := newTS()
	d1 := newTS()    // index 0, runs second (skipped by Stopping)
	d2 := &testStep{ // index 1, runs first (blocking)
		Base:      step.Base{},
		blocking:  true,
		startedCh: make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	task := newTestTaskWithBoth([]step.Step{s1}, []step.Step{d1, d2})

	ctx := loggerCtx()

	done := make(chan error, 1)
	go func() {
		done <- runner.Start(ctx, task)
	}()

	// Wait for d2 (index 1, runs first in reverse order) to start blocking
	<-d2.startedCh

	// Stop: closes t.Stopping and cancels ctx → d2 unblocks → next iteration sees Stopping
	err := runner.Stop(ctx, task)
	require.NoError(t, err)

	// Wait for d2 to actually finish
	<-d2.doneCh

	// Now read runner.Start result
	select {
	case err = <-done:
		assert.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("runner.Start did not return after Stop during defer")
	}

	assert.True(t, s1.started)
	assert.True(t, d2.started, "d2 (index 1, first in reverse order) should have started")
	assert.False(t, d1.started, "d1 (index 0, second in reverse order) should be skipped by Stopping")
}
