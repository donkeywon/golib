package runner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRunner is a Runner that allows control over each lifecycle method's behavior.
// All lifecycle methods MUST be called through runner.Init/Start/Stop, not directly.
type testRunner struct {
	Base

	initErr  error
	startErr error
	stopErr  error

	initFn  func()
	startFn func()
	stopFn  func()

	initCalled  atomic.Bool
	startCalled atomic.Bool
	stopCalled  atomic.Bool
}

func (tr *testRunner) Init(ctx context.Context) error {
	tr.initCalled.Store(true)
	_ = tr.Base.Init(ctx)
	if tr.initFn != nil {
		tr.initFn()
	}
	return tr.initErr
}

func (tr *testRunner) Start(ctx context.Context) error {
	tr.startCalled.Store(true)
	if tr.startFn != nil {
		tr.startFn()
	}
	if tr.startErr != nil {
		return tr.startErr
	}
	return tr.Base.Start(ctx)
}

func (tr *testRunner) Stop(ctx context.Context) error {
	tr.stopCalled.Store(true)
	if tr.stopFn != nil {
		tr.stopFn()
	}
	return tr.stopErr
}

// TestNormalLifecycle verifies the happy path: Init -> Start -> Stop -> Done.
func TestNormalLifecycle(t *testing.T) {
	r := &testRunner{}
	ctx := context.Background()

	require.NoError(t, Init(ctx, r))
	assert.True(t, r.initCalled.Load(), "Init was not called on runner")

	done := make(chan error, 1)
	go func() { done <- Start(ctx, r) }()

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, Stop(ctx, r))

	assert.NoError(t, <-done, "Start returned error")
	assert.True(t, r.startCalled.Load(), "Start was not called on runner")
	assert.True(t, r.stopCalled.Load(), "Stop was not called on runner")

	select {
	case <-r.Done():
	default:
		t.Fatal("Done channel was not closed")
	}
}

func TestStartWithCancelledContext(t *testing.T) {
	r := &testRunner{}
	require.NoError(t, Init(context.Background(), r))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Start(ctx, r)
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, r.startCalled.Load(), "Start was unexpectedly called")
}

func TestStartContextCancellationDuringStart(t *testing.T) {
	r := &testRunner{}
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, Init(context.Background(), r))

	done := make(chan error, 1)
	go func() { done <- Start(ctx, r) }()

	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-done
	assert.ErrorIs(t, err, context.Canceled)
	assert.True(t, r.stopCalled.Load(), "Stop was not called after context cancellation")
}

func TestStopBeforeStart(t *testing.T) {
	r := &testRunner{}
	require.NoError(t, Init(context.Background(), r))

	assert.Panics(t, func() {
		_ = Stop(context.Background(), r)
	}, "expected panic for Stop before Start")
}

func TestStartTwice(t *testing.T) {
	r := &testRunner{}
	require.NoError(t, Init(context.Background(), r))

	ctx := context.Background()
	started := make(chan struct{})
	r.startFn = func() { close(started) }

	done := make(chan error, 1)
	go func() { done <- Start(ctx, r) }()
	<-started

	panicCaught := make(chan any, 1)
	go func() {
		defer func() { panicCaught <- recover() }()
		_ = Start(ctx, r)
	}()

	p := <-panicCaught
	if p == nil {
		_ = Stop(ctx, r)
		<-done
		t.Fatal("expected panic for Start twice")
	}

	_ = Stop(ctx, r)
	<-done
}

func TestStopIdempotent(t *testing.T) {
	r := &testRunner{}
	ctx := context.Background()
	require.NoError(t, Init(ctx, r))

	done := make(chan error, 1)
	go func() { done <- Start(ctx, r) }()
	time.Sleep(10 * time.Millisecond)

	require.NoError(t, Stop(ctx, r))
	assert.True(t, r.stopCalled.Load(), "Stop was not called on first attempt")

	r.stopCalled.Store(false)
	assert.NoError(t, Stop(ctx, r), "second Stop returned error")
	assert.False(t, r.stopCalled.Load(), "Stop was called again on second attempt")

	assert.NoError(t, <-done)
}

func TestStopAndWait(t *testing.T) {
	r := &testRunner{}
	ctx := context.Background()
	require.NoError(t, Init(ctx, r))

	go func() { _ = Start(ctx, r) }()
	time.Sleep(10 * time.Millisecond)

	require.NoError(t, StopAndWait(ctx, r))

	select {
	case <-r.Done():
	default:
		t.Fatal("Done was not closed after StopAndWait")
	}
}

func TestStopAndWaitAfterStop(t *testing.T) {
	r := &testRunner{}
	ctx := context.Background()
	require.NoError(t, Init(ctx, r))

	go func() { _ = Start(ctx, r) }()
	time.Sleep(10 * time.Millisecond)

	require.NoError(t, Stop(ctx, r))
	require.NoError(t, StopAndWait(ctx, r))
}

func TestStopAndWaitBlocksUntilDone(t *testing.T) {
	r := &testRunner{}
	ctx := context.Background()
	require.NoError(t, Init(ctx, r))

	go func() { _ = Start(ctx, r) }()
	time.Sleep(10 * time.Millisecond)

	require.NoError(t, StopAndWait(ctx, r))

	select {
	case <-r.Done():
	default:
		t.Fatal("Done was not closed after StopAndWait")
	}
}

func TestInitPanicRecovery(t *testing.T) {
	r := &testRunner{
		initFn: func() { panic("boom in init") },
	}

	err := Init(context.Background(), r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom in init")
}

func TestStartPanicRecovery(t *testing.T) {
	r := &testRunner{
		startFn: func() { panic("boom in start") },
	}

	ctx := context.Background()
	require.NoError(t, Init(ctx, r))

	err := Start(ctx, r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom in start")

	select {
	case <-r.Done():
	default:
		t.Fatal("Done was not closed after Start panic")
	}
}

func TestStopPanicRecovery(t *testing.T) {
	r := &testRunner{
		stopFn: func() { panic("boom in stop") },
	}

	ctx := context.Background()
	require.NoError(t, Init(ctx, r))

	done := make(chan error, 1)
	go func() { done <- Start(ctx, r) }()
	time.Sleep(10 * time.Millisecond)

	err := Stop(ctx, r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom in stop")

	assert.NoError(t, <-done, "Start returned error")
}

func TestInitError(t *testing.T) {
	wantErr := errors.New("init error")
	r := &testRunner{initErr: wantErr}

	err := Init(context.Background(), r)
	assert.ErrorIs(t, err, wantErr)
}

func TestStartError(t *testing.T) {
	wantErr := errors.New("start error")
	r := &testRunner{startErr: wantErr}

	ctx := context.Background()
	require.NoError(t, Init(ctx, r))

	err := Start(ctx, r)
	assert.ErrorIs(t, err, wantErr)
}

func TestStopError(t *testing.T) {
	wantErr := errors.New("stop error")
	r := &testRunner{stopErr: wantErr}

	ctx := context.Background()
	require.NoError(t, Init(ctx, r))

	done := make(chan error, 1)
	go func() { done <- Start(ctx, r) }()
	time.Sleep(10 * time.Millisecond)

	err := Stop(ctx, r)
	assert.ErrorIs(t, err, wantErr)

	assert.NoError(t, <-done, "Start returned error")
}

func TestConcurrentStop(t *testing.T) {
	r := &testRunner{}
	ctx := context.Background()
	require.NoError(t, Init(ctx, r))

	go func() { _ = Start(ctx, r) }()
	time.Sleep(10 * time.Millisecond)

	const n = 10
	done := make(chan struct{})
	for range n {
		go func() {
			_ = Stop(ctx, r)
			done <- struct{}{}
		}()
	}

	for range n {
		<-done
	}
	// No panics = success.
}

func TestNilRunnerPanics(t *testing.T) {
	t.Run("Init", func(t *testing.T) {
		assert.Panics(t, func() { _ = Init(context.Background(), nil) })
	})
	t.Run("Start", func(t *testing.T) {
		assert.Panics(t, func() { _ = Start(context.Background(), nil) })
	})
	t.Run("Stop", func(t *testing.T) {
		assert.Panics(t, func() { _ = Stop(context.Background(), nil) })
	})
}

func TestNilContextPanics(t *testing.T) {
	r := &testRunner{}

	t.Run("Init", func(t *testing.T) {
		assert.Panics(t, func() { _ = Init(nil, r) })
	})
	t.Run("Start", func(t *testing.T) {
		assert.Panics(t, func() { _ = Start(nil, r) })
	})
	t.Run("Stop", func(t *testing.T) {
		assert.Panics(t, func() { _ = Stop(nil, r) })
	})
}

func TestStartInterruptedByStop(t *testing.T) {
	r := &testRunner{}
	ctx := context.Background()
	require.NoError(t, Init(ctx, r))

	done := make(chan error, 1)
	go func() { done <- Start(ctx, r) }()

	time.Sleep(5 * time.Millisecond)

	require.NoError(t, Stop(ctx, r))

	err := <-done
	assert.NoError(t, err, "Start should return nil on interrupted start")

	select {
	case <-r.Done():
	default:
		t.Fatal("Done was not closed")
	}
}

func TestSignalChannels(t *testing.T) {
	r := &testRunner{}
	ctx := context.Background()
	require.NoError(t, Init(ctx, r))

	// Before Start: Started must be open.
	select {
	case <-r.Started():
		t.Fatal("Started should not be closed before Start")
	default:
	}

	done := make(chan error, 1)
	go func() { done <- Start(ctx, r) }()

	time.Sleep(10 * time.Millisecond)
	select {
	case <-r.Started():
	default:
		t.Fatal("Started should be closed after Start")
	}

	require.NoError(t, Stop(ctx, r))

	// After Stop: Stopping and StopDone must be closed.
	select {
	case <-r.Stopping():
	default:
		t.Fatal("Stopping should be closed after Stop")
	}
	select {
	case <-r.StopDone():
	default:
		t.Fatal("StopDone should be closed after Stop")
	}

	assert.NoError(t, <-done)

	// After Start returns: Done must be closed.
	select {
	case <-r.Done():
	default:
		t.Fatal("Done should be closed after Start returns")
	}
}
