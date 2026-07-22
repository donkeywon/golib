package boot

import (
	"context"
	"testing"

	"github.com/donkeywon/golib/logs"
	"github.com/donkeywon/golib/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCfgPath(t *testing.T) {
	opt := CfgPath("/tmp/test.yaml")
	b := newBooter()
	opt(b)
	assert.Equal(t, "/tmp/test.yaml", b.options.CfgPath)
}

func TestCfgPath_Empty(t *testing.T) {
	b := newBooter()
	assert.Empty(t, b.options.CfgPath)
}

func TestEnvPrefix(t *testing.T) {
	opt := EnvPrefix("MYAPP")
	b := newBooter()
	opt(b)
	assert.Equal(t, "MYAPP", b.options.envPrefix)
}

func TestEnvPrefix_Empty(t *testing.T) {
	b := newBooter()
	assert.Empty(t, b.options.envPrefix)
}

func TestWithLoggerCreator(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		opt := WithLoggerCreator("logger", logs.NopLoggerCreator)
		b := newBooter()
		opt(b)
		assert.Equal(t, "logger", b.options.loggerCfgKey)
		assert.NotNil(t, b.options.loggerCreator)
	})

	t.Run("empty key panics", func(t *testing.T) {
		assert.Panics(t, func() {
			WithLoggerCreator("", logs.NopLoggerCreator)
		})
	})

	t.Run("nil creator panics", func(t *testing.T) {
		assert.Panics(t, func() {
			WithLoggerCreator("test", nil)
		})
	})
}

func TestOnCreated(t *testing.T) {
	called := false
	opt := OnCreated("test-daemon", func(ctx context.Context) {
		called = true
	})
	b := newBooter()
	opt(b)
	assert.Len(t, b.options.onCreated, 1)

	// Invoke the callback
	b.options.onCreated["test-daemon"](context.Background())
	assert.True(t, called)
}

func TestOnCreated_MultipleTypes(t *testing.T) {
	order := make([]string, 0)
	b := newBooter()

	opt1 := OnCreated("daemon1", func(ctx context.Context) {
		order = append(order, "daemon1")
	})
	opt2 := OnCreated("daemon2", func(ctx context.Context) {
		order = append(order, "daemon2")
	})

	opt1(b)
	opt2(b)
	assert.Len(t, b.options.onCreated, 2)

	b.options.onCreated["daemon1"](context.Background())
	b.options.onCreated["daemon2"](context.Background())
	assert.Equal(t, []string{"daemon1", "daemon2"}, order)
}

func TestAfterDone(t *testing.T) {
	called := false
	opt := AfterDone("test-daemon", func(ctx context.Context) {
		called = true
	})
	b := newBooter()
	opt(b)
	assert.Len(t, b.options.afterDone, 1)

	b.options.afterDone["test-daemon"](context.Background())
	assert.True(t, called)
}

func TestAfterDone_MultipleTypes(t *testing.T) {
	order := make([]string, 0)
	b := newBooter()

	opt1 := AfterDone("daemon1", func(ctx context.Context) {
		order = append(order, "daemon1")
	})
	opt2 := AfterDone("daemon2", func(ctx context.Context) {
		order = append(order, "daemon2")
	})

	opt1(b)
	opt2(b)
	assert.Len(t, b.options.afterDone, 2)

	b.options.afterDone["daemon2"](context.Background())
	b.options.afterDone["daemon1"](context.Background())
	assert.Equal(t, []string{"daemon2", "daemon1"}, order)
}

func TestCreateOptions(t *testing.T) {
	opts := createOptions()
	require.NotNil(t, opts)
	assert.Empty(t, opts.CfgPath)
	assert.False(t, opts.PrintVersion)
	assert.Empty(t, opts.loggerCfgKey)
	assert.Nil(t, opts.loggerCreator)
	assert.Empty(t, opts.envPrefix)
	assert.NotNil(t, opts.onCreated)
	assert.NotNil(t, opts.afterDone)
	assert.Empty(t, opts.onCreated)
	assert.Empty(t, opts.afterDone)
}

func TestOption_Composition(t *testing.T) {
	b := newBooter()

	CfgPath("/tmp/cfg.yaml")(b)
	EnvPrefix("APP")(b)
	WithLoggerCreator("log", logs.NopLoggerCreator)(b)

	assert.Equal(t, "/tmp/cfg.yaml", b.options.CfgPath)
	assert.Equal(t, "APP", b.options.envPrefix)
	assert.Equal(t, "log", b.options.loggerCfgKey)
}

// newBooter creates a booter with initialized options and daemonsMap for testing.
func newBooter() *booter {
	return &booter{
		options:    createOptions(),
		daemonsMap: make(map[DaemonType]Daemon),
	}
}

// testDaemon implements Daemon for testing.
type testDaemon struct {
	runner.Base
	cfg     testDaemonCfg
	inited  bool
	started bool
	stopped bool
}

type testDaemonCfg struct {
	Name string
}

func (d *testDaemon) SetCfg(cfg any) {
	d.cfg = cfg.(testDaemonCfg)
}

func (d *testDaemon) Init(ctx context.Context) error {
	d.inited = true
	return nil
}

func (d *testDaemon) Start(ctx context.Context) error {
	d.started = true
	return nil
}

func (d *testDaemon) Stop(ctx context.Context) error {
	d.stopped = true
	return nil
}

func TestDaemonType_String(t *testing.T) {
	dt := DaemonType("server")
	assert.Equal(t, "server", string(dt))
}

func TestReg(t *testing.T) {
	// Save the original state
	origTypes := make([]DaemonType, len(_daemonTypes))
	copy(origTypes, _daemonTypes)

	dt := DaemonType("test-reg-daemon")
	Reg(dt, func() Daemon { return &testDaemon{} }, func() any { return &testDaemonCfg{} })

	assert.Contains(t, _daemonTypes, dt)

	// Clean up
	_daemonTypes = origTypes
}

func TestReg_Duplicate(t *testing.T) {
	origTypes := make([]DaemonType, len(_daemonTypes))
	copy(origTypes, _daemonTypes)

	dt := DaemonType("test-reg-dup")
	Reg(dt, func() Daemon { return &testDaemon{} }, func() any { return &testDaemonCfg{} })
	// Register again — should not duplicate in the slice
	Reg(dt, func() Daemon { return &testDaemon{} }, func() any { return &testDaemonCfg{} })

	count := 0
	for _, t := range _daemonTypes {
		if t == dt {
			count++
		}
	}
	assert.Equal(t, 1, count)

	_daemonTypes = origTypes
}

func TestRegCfg(t *testing.T) {
	// Save original state
	origKeys := make([]string, len(_additionalCfgKeys))
	copy(origKeys, _additionalCfgKeys)
	origMap := make(map[string]any)
	for k, v := range _additionalCfgMap {
		origMap[k] = v
	}

	name := "test_cfg"
	cfg := &testDaemonCfg{Name: "test"}
	RegCfg(name, cfg)

	assert.Contains(t, _additionalCfgKeys, name)
	assert.Equal(t, cfg, _additionalCfgMap[name])

	// Clean up — find and remove from slice
	for i, k := range _additionalCfgKeys {
		if k == name {
			_additionalCfgKeys = append(_additionalCfgKeys[:i], _additionalCfgKeys[i+1:]...)
			break
		}
	}
	delete(_additionalCfgMap, name)
}

func TestRegCfg_DuplicatePanics(t *testing.T) {
	name := "test_dup_cfg"
	cfg := &testDaemonCfg{Name: "test"}
	RegCfg(name, cfg)

	assert.Panics(t, func() {
		RegCfg(name, &testDaemonCfg{Name: "test2"})
	})

	// Clean up
	for i, k := range _additionalCfgKeys {
		if k == name {
			_additionalCfgKeys = append(_additionalCfgKeys[:i], _additionalCfgKeys[i+1:]...)
			break
		}
	}
	delete(_additionalCfgMap, name)
}

func TestRegCfg_NonPointerPanics(t *testing.T) {
	assert.Panics(t, func() {
		RegCfg("non_ptr_cfg", testDaemonCfg{Name: "val"})
	})
}

func TestGet(t *testing.T) {
	// Create a booter and populate daemonsMap
	b := newBooter()
	dt := DaemonType("get-test")
	d := &testDaemon{}
	b.daemonsMap = map[DaemonType]Daemon{
		dt: d,
	}
	_b = b

	result := Get[*testDaemon](dt)
	assert.Equal(t, d, result)

	_b = nil
}

func TestGet_NotExistsPanics(t *testing.T) {
	b := newBooter()
	b.daemonsMap = make(map[DaemonType]Daemon)
	_b = b

	assert.Panics(t, func() {
		Get[*testDaemon]("nonexistent")
	})

	_b = nil
}

func TestBootStop_BeforeBoot(t *testing.T) {
	// Stop() panics when _b is nil because runner.Stop checks for nil runner
	_b = nil
	assert.Panics(t, func() {
		_ = Stop(context.Background())
	})
}

func TestCreate(t *testing.T) {
	b := create(
		CfgPath("/tmp/test.yaml"),
		EnvPrefix("TEST"),
	)

	require.NotNil(t, b)
	require.NotNil(t, b.options)
	assert.Equal(t, "/tmp/test.yaml", b.options.CfgPath)
	assert.Equal(t, "TEST", b.options.envPrefix)
	require.NotNil(t, b.daemonsMap)
}

func TestRegCfg_ConflictsWithDaemonType(t *testing.T) {
	// Register a daemon type first
	origTypes := make([]DaemonType, len(_daemonTypes))
	copy(origTypes, _daemonTypes)

	dt := DaemonType("conflict-test")
	Reg(dt, func() Daemon { return &testDaemon{} }, func() any { return &testDaemonCfg{} })

	assert.Panics(t, func() {
		RegCfg(string(dt), &testDaemonCfg{})
	})

	// Clean up
	_daemonTypes = origTypes
}

func TestBoot_Stop_NilBooter(t *testing.T) {
	// Stop() panics when _b is nil because runner.Stop checks for nil runner
	_b = nil
	assert.Panics(t, func() {
		_ = Stop(context.Background())
	})
}
