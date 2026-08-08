package task

import (
	"context"
	"errors"
	"testing"

	"github.com/donkeywon/golib/kvs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/task/step"
	"github.com/stretchr/testify/require"
)

const mockStepType step.Type = "mock"

type mockStepCfg struct {
	Name    string
	RunErr  error
	InitErr error
	PanicOn bool
}

type mockStep struct {
	kvs.Map[string, any]
	cfg mockStepCfg
}

func (m *mockStep) SetCfg(cfg any) {
	m.cfg = cfg.(mockStepCfg)
}

func (m *mockStep) Run(ctx context.Context) error {
	if m.cfg.PanicOn {
		panic("mock step panic")
	}
	return m.cfg.RunErr
}

func (m *mockStep) Init(ctx context.Context) error {
	return m.cfg.InitErr
}

// noInitStep 只实现 Run,不实现 Init,覆盖 initializer 接口缺席分支
type noInitStep struct {
	kvs.Map[string, any]
}

func (s *noInitStep) Run(ctx context.Context) error {
	return nil
}

func TestMain(m *testing.M) {
	plugin.Reg(mockStepType, func() step.Step { return &mockStep{} }, func() any { return mockStepCfg{} })
	m.Run()
}

func newTask(t *testing.T, cfg Cfg) *Task {
	t.Helper()
	tk := New()
	tk.SetCfg(cfg)
	return tk
}

func baseCfg() Cfg {
	return NewCfg().SetID("test-task")
}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	return t.Context()
}

// ---- Cfg 与基础方法 ----

func TestCfgChain(t *testing.T) {
	c := NewCfg().SetID("id").Add(mockStepType, mockStepCfg{Name: "s"}).Defer(mockStepType, mockStepCfg{Name: "d"})
	require.Equal(t, "id", c.ID)
	require.Len(t, c.Steps, 1)
	require.Len(t, c.DeferSteps, 1)
	require.Equal(t, mockStepType, c.Steps[0].Type)
	require.Equal(t, mockStepType, c.DeferSteps[0].Type)

	// 值接收者:原值不变
	c2 := c.Add(mockStepType, mockStepCfg{})
	require.Len(t, c.Steps, 1)
	require.Len(t, c2.Steps, 2)
}

func TestSetCfgAndCfg(t *testing.T) {
	tk := newTask(t, baseCfg().SetID("x"))
	require.Equal(t, "x", tk.Cfg().ID)
}

func TestHookRegistration(t *testing.T) {
	tk := New()
	h := func(context.Context, *Task, int, step.Step, error) error { return nil }
	tk.BeforeStepRun(h)
	tk.AfterStepDone(h)
	tk.BeforeDeferStepRun(h)
	tk.AfterDeferStepDone(h)
	require.Len(t, tk.beforeStepRunHooks, 1)
	require.Len(t, tk.afterStepDoneHooks, 1)
	require.Len(t, tk.beforeDeferStepRunHooks, 1)
	require.Len(t, tk.afterDeferStepDoneHooks, 1)

	// 可变参数追加
	tk.BeforeStepRun(h, h)
	require.Len(t, tk.beforeStepRunHooks, 3)
}

func TestTaskPluginRegistered(t *testing.T) {
	// 触发 init 中注册的两个闭包(creator + cfg creator)
	cfg := plugin.CreateCfg[any](PluginTypeTask)
	require.IsType(t, Cfg{}, cfg)

	tk := plugin.Create[*Task, Cfg](PluginTypeTask)
	require.NotNil(t, tk)
}

func TestStepsReturnClone(t *testing.T) {
	tk := newTask(t, baseCfg().Add(mockStepType, mockStepCfg{Name: "s"}).Defer(mockStepType, mockStepCfg{Name: "d"}))
	require.NoError(t, tk.Run(testCtx(t)))

	steps := tk.Steps()
	steps[0] = nil
	require.NotNil(t, tk.Steps()[0], "修改 clone 不应影响内部")

	deferSteps := tk.DeferSteps()
	deferSteps[0] = nil
	require.NotNil(t, tk.DeferSteps()[0])
}

// ---- Run 主流程 ----

func TestRun_Success(t *testing.T) {
	var order []string
	tk := newTask(t, baseCfg().
		Add(mockStepType, mockStepCfg{Name: "s1"}).
		Add(mockStepType, mockStepCfg{Name: "s2"}).
		Defer(mockStepType, mockStepCfg{Name: "d1"}).
		Defer(mockStepType, mockStepCfg{Name: "d2"}))
	tk.BeforeStepRun(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		order = append(order, "before-"+st.(*mockStep).cfg.Name)
		return nil
	})
	tk.BeforeDeferStepRun(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		order = append(order, "before-defer-"+st.(*mockStep).cfg.Name)
		return nil
	})

	require.NoError(t, tk.Run(testCtx(t)))
	require.Equal(t, []string{"before-s1", "before-s2", "before-defer-d2", "before-defer-d1"}, order, "defer steps 应反向执行")
}

func TestRun_NoDeferSteps(t *testing.T) {
	tk := newTask(t, baseCfg().Add(mockStepType, mockStepCfg{Name: "s"}))
	require.NoError(t, tk.Run(testCtx(t)))
}

func TestRun_StepFailed(t *testing.T) {
	var deferRan bool
	tk := newTask(t, baseCfg().
		Add(mockStepType, mockStepCfg{Name: "ok"}).
		Add(mockStepType, mockStepCfg{Name: "bad", RunErr: errors.New("boom")}).
		Defer(mockStepType, mockStepCfg{Name: "d"}))
	tk.BeforeDeferStepRun(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		deferRan = true
		return nil
	})

	err := tk.Run(testCtx(t))
	require.Error(t, err)
	require.ErrorContains(t, err, "run step failed")
	require.True(t, deferRan, "step 失败后 defer steps 必须执行")
}

func TestRun_DeferStepFailed(t *testing.T) {
	tk := newTask(t, baseCfg().
		Add(mockStepType, mockStepCfg{Name: "ok"}).
		Defer(mockStepType, mockStepCfg{Name: "d", RunErr: errors.New("defer boom")}))

	err := tk.Run(testCtx(t))
	require.Error(t, err)
	require.ErrorContains(t, err, "run defer step failed")
}

func TestRun_Canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(testCtx(t))
	cancel()

	tk := newTask(t, baseCfg().Add(mockStepType, mockStepCfg{Name: "s"}))
	err := tk.Run(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestRun_StepPanic(t *testing.T) {
	tk := newTask(t, baseCfg().Add(mockStepType, mockStepCfg{Name: "p", PanicOn: true}))
	err := tk.Run(testCtx(t))
	require.Error(t, err)
	require.ErrorContains(t, err, "panic on run step")
}

func TestRun_StepInitError(t *testing.T) {
	tk := newTask(t, baseCfg().Add(mockStepType, mockStepCfg{Name: "i", InitErr: errors.New("init boom")}))
	err := tk.Run(testCtx(t))
	require.Error(t, err)
	require.ErrorContains(t, err, "init step failed")
}

// ---- hook 分支 ----

func TestRun_BeforeHookSkip(t *testing.T) {
	var ran []string
	tk := newTask(t, baseCfg().
		Add(mockStepType, mockStepCfg{Name: "skip-me"}).
		Add(mockStepType, mockStepCfg{Name: "s2"}))
	tk.BeforeStepRun(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		if st.(*mockStep).cfg.Name == "skip-me" {
			return ErrSkip
		}
		return nil
	})
	tk.AfterStepDone(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		ran = append(ran, st.(*mockStep).cfg.Name)
		return nil
	})

	require.NoError(t, tk.Run(testCtx(t)))
	require.Equal(t, []string{"s2"}, ran, "被跳过的 step 不应执行,后续 step 继续")
}

func TestRun_BeforeHookError(t *testing.T) {
	tk := newTask(t, baseCfg().Add(mockStepType, mockStepCfg{Name: "s"}))
	tk.BeforeStepRun(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		return errors.New("hook fail")
	})

	err := tk.Run(testCtx(t))
	require.Error(t, err)
	require.ErrorContains(t, err, "hook before run step failed")
}

func TestRun_AfterHookSkip_StepFailed(t *testing.T) {
	tk := newTask(t, baseCfg().
		Add(mockStepType, mockStepCfg{Name: "bad", RunErr: errors.New("boom")}).
		Add(mockStepType, mockStepCfg{Name: "s2"}))
	tk.AfterStepDone(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		return ErrSkip
	})

	// step 失败被 ErrSkip 吞掉,继续后续 step
	require.NoError(t, tk.Run(testCtx(t)))
}

func TestRun_AfterHookSkip_Success(t *testing.T) {
	tk := newTask(t, baseCfg().Add(mockStepType, mockStepCfg{Name: "s"}))
	tk.AfterStepDone(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		return ErrSkip
	})
	require.NoError(t, tk.Run(testCtx(t)))
}

func TestRun_AfterHookError_StepSuccess(t *testing.T) {
	tk := newTask(t, baseCfg().Add(mockStepType, mockStepCfg{Name: "s"}))
	tk.AfterStepDone(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		return errors.New("after hook fail")
	})

	err := tk.Run(testCtx(t))
	require.Error(t, err)
	require.ErrorContains(t, err, "hook after step done failed")
}

func TestRun_AfterHookError_StepFailed(t *testing.T) {
	tk := newTask(t, baseCfg().Add(mockStepType, mockStepCfg{Name: "bad", RunErr: errors.New("boom")}))
	tk.AfterStepDone(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		return errors.New("after hook fail")
	})

	err := tk.Run(testCtx(t))
	require.Error(t, err)
	require.ErrorContains(t, err, "hook after step done failed")
	require.ErrorContains(t, err, "run step failed")
}

// ---- defer steps 分支 ----

func TestRun_DeferBeforeHookSkip(t *testing.T) {
	var ran []string
	tk := newTask(t, baseCfg().
		Defer(mockStepType, mockStepCfg{Name: "skip-d"}).
		Defer(mockStepType, mockStepCfg{Name: "d2"}))
	tk.BeforeDeferStepRun(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		if st.(*mockStep).cfg.Name == "skip-d" {
			return ErrSkip
		}
		return nil
	})
	tk.AfterDeferStepDone(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		ran = append(ran, st.(*mockStep).cfg.Name)
		return nil
	})

	require.NoError(t, tk.Run(testCtx(t)))
	require.Equal(t, []string{"d2"}, ran)
}

func TestRun_DeferBeforeHookError(t *testing.T) {
	var stepRan bool
	tk := newTask(t, baseCfg().
		Add(mockStepType, mockStepCfg{Name: "s"}).
		Defer(mockStepType, mockStepCfg{Name: "d"}))
	tk.BeforeDeferStepRun(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		return errors.New("defer hook fail")
	})
	tk.AfterDeferStepDone(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		stepRan = true
		return nil
	})

	// before hook 失败后 defer step 仍执行,错误收集返回
	err := tk.Run(testCtx(t))
	require.Error(t, err)
	require.ErrorContains(t, err, "hook before run defer step failed")
	require.True(t, stepRan, "before hook 失败后 defer step 仍应执行")
}

func TestRun_DeferAfterHookSkip_StepFailed(t *testing.T) {
	tk := newTask(t, baseCfg().
		Add(mockStepType, mockStepCfg{Name: "s"}).
		Defer(mockStepType, mockStepCfg{Name: "d", RunErr: errors.New("boom")}))
	tk.AfterDeferStepDone(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		return ErrSkip
	})

	// defer step 失败被 ErrSkip 吞掉
	require.NoError(t, tk.Run(testCtx(t)))
}

func TestRun_DeferAfterHookSkip_Success(t *testing.T) {
	tk := newTask(t, baseCfg().
		Add(mockStepType, mockStepCfg{Name: "s"}).
		Defer(mockStepType, mockStepCfg{Name: "d"}))
	tk.AfterDeferStepDone(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		return ErrSkip
	})
	require.NoError(t, tk.Run(testCtx(t)))
}

func TestRun_DeferAfterHookError(t *testing.T) {
	tk := newTask(t, baseCfg().
		Add(mockStepType, mockStepCfg{Name: "s"}).
		Defer(mockStepType, mockStepCfg{Name: "d"}))
	tk.AfterDeferStepDone(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		return errors.New("defer after hook fail")
	})

	err := tk.Run(testCtx(t))
	require.Error(t, err)
	require.ErrorContains(t, err, "hook after defer step done failed")
}

func TestRun_DeferAfterHookError_StepFailed(t *testing.T) {
	tk := newTask(t, baseCfg().
		Add(mockStepType, mockStepCfg{Name: "s"}).
		Defer(mockStepType, mockStepCfg{Name: "d", RunErr: errors.New("boom")}))
	tk.AfterDeferStepDone(func(ctx context.Context, t *Task, i int, st step.Step, err error) error {
		return errors.New("defer after hook fail")
	})

	err := tk.Run(testCtx(t))
	require.Error(t, err)
	require.ErrorContains(t, err, "hook after defer step done failed")
	require.ErrorContains(t, err, "run defer step failed")
}

// ---- 内部函数直接测试(white-box) ----

func TestRunDeferSteps_Canceled(t *testing.T) {
	tk := newTask(t, baseCfg().Defer(mockStepType, mockStepCfg{Name: "d"}))
	tk.deferSteps = []step.Step{&mockStep{cfg: mockStepCfg{Name: "d"}}}

	ctx, cancel := context.WithCancel(testCtx(t))
	cancel()
	err := tk.runDeferSteps(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestRunStep_Success(t *testing.T) {
	tk := newTask(t, baseCfg())
	require.NoError(t, tk.runStep(testCtx(t), 0, mockStepType, &mockStep{}, false))
}

func TestRunStep_NoInitializer(t *testing.T) {
	tk := newTask(t, baseCfg())
	require.NoError(t, tk.runStep(testCtx(t), 0, mockStepType, &noInitStep{}, false))
}

func TestRunStep_RunError(t *testing.T) {
	tk := newTask(t, baseCfg())
	err := tk.runStep(testCtx(t), 0, mockStepType, &mockStep{cfg: mockStepCfg{RunErr: errors.New("boom")}}, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "start step failed")
}

func TestRunStep_InitError(t *testing.T) {
	tk := newTask(t, baseCfg())
	err := tk.runStep(testCtx(t), 0, mockStepType, &mockStep{cfg: mockStepCfg{InitErr: errors.New("init boom")}}, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "init step failed")
}

func TestRunStep_Panic(t *testing.T) {
	tk := newTask(t, baseCfg())
	err := tk.runStep(testCtx(t), 0, mockStepType, &mockStep{cfg: mockStepCfg{PanicOn: true}}, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "panic on run step")
}

func TestRunStep_DeferPanic(t *testing.T) {
	tk := newTask(t, baseCfg())
	err := tk.runStep(testCtx(t), 0, mockStepType, &mockStep{cfg: mockStepCfg{PanicOn: true}}, true)
	require.Error(t, err)
	require.ErrorContains(t, err, "panic on run defer step")
}

func TestHookStep_Empty(t *testing.T) {
	err := hookStep(testCtx(t), nil, 0, mockStepType, &mockStep{}, nil, &Task{}, false)
	require.NoError(t, err)
}

func TestHookStep_CollectsErrors(t *testing.T) {
	h1 := func(context.Context, *Task, int, step.Step, error) error { return errors.New("h1") }
	h2 := func(context.Context, *Task, int, step.Step, error) error { return errors.New("h2") }
	err := hookStep(testCtx(t), []StepHook{h1, h2}, 0, mockStepType, &mockStep{}, nil, &Task{}, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "h1")
	require.ErrorContains(t, err, "h2")
}

func TestHook_Recover(t *testing.T) {
	h := func(context.Context, *Task, int, step.Step, error) error { panic("hook panic") }
	err := hook(testCtx(t), 0, h, 0, mockStepType, &mockStep{}, nil, &Task{}, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "panic on hook step")
}

func TestHook_RecoverDefer(t *testing.T) {
	h := func(context.Context, *Task, int, step.Step, error) error { panic("hook panic") }
	err := hook(testCtx(t), 0, h, 0, mockStepType, &mockStep{}, nil, &Task{}, true)
	require.Error(t, err)
	require.ErrorContains(t, err, "panic on hook defer step")
}
