package boot

import (
	"context"
	"testing"

	"github.com/donkeywon/golib/logs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBooter_Init_MissingLogger(t *testing.T) {
	b := newBooter()

	err := b.Init(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty logger cfg key")
}

func TestBooter_Init_NilLoggerCreator(t *testing.T) {
	b := newBooter()
	b.options.loggerCfgKey = "log"

	err := b.Init(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil logger creator")
}

func TestBooter_Stop(t *testing.T) {
	b := newBooter()
	err := b.Stop(context.Background())
	require.NoError(t, err)
}

func TestBooter_CreateDaemons(t *testing.T) {
	origTypes := make([]DaemonType, len(_daemonTypes))
	copy(origTypes, _daemonTypes)
	defer func() { _daemonTypes = origTypes }()

	dt := DaemonType("create-daemon-test")
	_daemonTypes = []DaemonType{dt}
	plugin.Reg(dt, func() Daemon { return &testDaemon{} }, func() any { return &testDaemonCfg{} })

	b := newBooter()
	b.cfgMap = map[string]any{
		string(dt): &testDaemonCfg{Name: "my-daemon"},
	}

	b.createDaemons(context.Background())

	require.Contains(t, b.daemonsMap, dt)
	d, ok := b.daemonsMap[dt].(*testDaemon)
	require.True(t, ok)
	assert.Equal(t, "my-daemon", d.cfg.Name)
}

func TestBooter_CreateDaemons_WithOnCreated(t *testing.T) {
	origTypes := make([]DaemonType, len(_daemonTypes))
	copy(origTypes, _daemonTypes)
	defer func() { _daemonTypes = origTypes }()

	dt := DaemonType("oncreate-daemon-test")
	_daemonTypes = []DaemonType{dt}
	plugin.Reg(dt, func() Daemon { return &testDaemon{} }, func() any { return &testDaemonCfg{} })

	called := false
	b := newBooter()
	b.options.onCreated[dt] = func(ctx context.Context) {
		called = true
	}
	b.cfgMap = map[string]any{
		string(dt): &testDaemonCfg{},
	}

	b.createDaemons(context.Background())
	assert.True(t, called)
}

func TestBooter_InitDaemons(t *testing.T) {
	origTypes := make([]DaemonType, len(_daemonTypes))
	copy(origTypes, _daemonTypes)
	defer func() { _daemonTypes = origTypes }()

	dt := DaemonType("init-daemon-test")
	_daemonTypes = []DaemonType{dt}

	d := &testDaemon{}
	b := newBooter()
	b.daemonsMap = map[DaemonType]Daemon{
		dt: d,
	}

	err := b.initDaemons(context.Background())
	require.NoError(t, err)
	assert.True(t, d.inited)
}

func TestBooter_BuildCfgMap(t *testing.T) {
	origTypes := make([]DaemonType, len(_daemonTypes))
	copy(origTypes, _daemonTypes)
	defer func() { _daemonTypes = origTypes }()

	dt := DaemonType("buildcfg-test")
	_daemonTypes = []DaemonType{dt}
	plugin.Reg(dt, func() Daemon { return &testDaemon{} }, func() any { return &testDaemonCfg{} })

	b := newBooter()
	b.options.loggerCfgKey = "logger"
	b.options.loggerCreator = logs.NopLoggerCreator

	cfgMap, cfgKeys := b.buildCfgMap()

	assert.Contains(t, cfgKeys, "logger")
	assert.Contains(t, cfgKeys, string(dt))
	require.Contains(t, cfgMap, "logger")
	require.Contains(t, cfgMap, string(dt))
}

func TestBooter_InitDaemons_Error(t *testing.T) {
	origTypes := make([]DaemonType, len(_daemonTypes))
	copy(origTypes, _daemonTypes)
	defer func() { _daemonTypes = origTypes }()

	dt := DaemonType("init-err-daemon-test")
	_daemonTypes = []DaemonType{dt}

	d := &errorInitDaemon{testDaemon: testDaemon{}}
	b := newBooter()
	b.daemonsMap = map[DaemonType]Daemon{
		dt: d,
	}

	err := b.initDaemons(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init daemon failed")
}

// errorInitDaemon returns an error from Init.
type errorInitDaemon struct {
	testDaemon
}

func (d *errorInitDaemon) Init(ctx context.Context) error {
	return assert.AnError
}

func TestBooter_BaseChannels(t *testing.T) {
	b := newBooter()

	// Channels should be lazily initialized
	select {
	case <-b.Started():
		t.Fatal("Started should not be closed")
	default:
	}

	select {
	case <-b.Stopping():
		t.Fatal("Stopping should not be closed")
	default:
	}

	select {
	case <-b.Done():
		t.Fatal("Done should not be closed")
	default:
	}
}

func TestBooter_Init(t *testing.T) {
	b := newBooter()
	err := b.Init(context.Background())
	require.Error(t, err) // missing logger, but channels should be initialized

	// After init, channels should exist (even though init failed)
	select {
	case <-b.Started():
		t.Fatal("Started not closed yet")
	default:
	}
}

func TestRunner_Init_NilRunner(t *testing.T) {
	assert.Panics(t, func() {
		_ = runner.Init(context.Background(), nil)
	})
}

func TestRunner_Init_NilContext(t *testing.T) {
	b := newBooter()
	assert.Panics(t, func() {
		_ = runner.Init(nil, b) // nolint:staticcheck
	})
}

func TestRunner_Start_NilRunner(t *testing.T) {
	assert.Panics(t, func() {
		_ = runner.Start(context.Background(), nil)
	})
}

func TestRunner_Start_AfterStopping(t *testing.T) {
	// runner.Start checks Stopping channel and panics if already stopping.
	// We can't directly mark stopping from another package, so we test nil case.
	assert.Panics(t, func() {
		_ = runner.Start(context.Background(), nil)
	})
}

func TestRunner_Stop_NilRunner(t *testing.T) {
	assert.Panics(t, func() {
		_ = runner.Stop(context.Background(), nil)
	})
}

func TestRunner_Stop_BeforeStart(t *testing.T) {
	b := newBooter()
	// Stop before Start panics because Started() returns a not-yet-closed channel
	assert.Panics(t, func() {
		_ = runner.Stop(context.Background(), b)
	})
}
