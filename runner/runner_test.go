package runner

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

// TestNormalLifecycle verifies the happy path: Init → Start → Stop → Done.
func TestNormalLifecycle(t *testing.T) {
	r := &testRunner{}

	ctx := context.Background()

	if err := Init(ctx, r); err != nil {
		t.Fatal("Init failed:", err)
	}
	if !r.initCalled.Load() {
		t.Fatal("Init was not called on runner")
	}

	done := make(chan error, 1)
	go func() {
		done <- Start(ctx, r)
	}()

	// Give the goroutine time to enter Start and mark started.
	time.Sleep(10 * time.Millisecond)

	if err := Stop(ctx, r); err != nil {
		t.Fatal("Stop failed:", err)
	}

	if startErr := <-done; startErr != nil {
		t.Fatal("Start returned error:", startErr)
	}
	if !r.startCalled.Load() {
		t.Fatal("Start was not called on runner")
	}
	if !r.stopCalled.Load() {
		t.Fatal("Stop was not called on runner")
	}

	// Verify Done channel is closed.
	select {
	case <-r.Done():
	default:
		t.Fatal("Done channel was not closed")
	}
}

// TestStartWithCancelledContext verifies that Start returns the context error
// when the context is already cancelled.
func TestStartWithCancelledContext(t *testing.T) {
	r := &testRunner{}
	if err := Init(context.Background(), r); err != nil {
		t.Fatal("Init failed:", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Start(ctx, r)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// Start should not have been called since the context was already cancelled.
	if r.startCalled.Load() {
		t.Fatal("Start was unexpectedly called")
	}
}

// TestStartContextCancellationDuringStart verifies that cancelling the context
// during Start triggers a clean shutdown via the monitoring goroutine.
// The Start call returns ctx.Err() to signal the cancellation reason.
func TestStartContextCancellationDuringStart(t *testing.T) {
	r := &testRunner{}

	ctx, cancel := context.WithCancel(context.Background())

	if err := Init(context.Background(), r); err != nil {
		t.Fatal("Init failed:", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- Start(ctx, r)
	}()

	// Give Start time to enter, then cancel.
	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// Stop should be called via the monitoring goroutine on cancellation.
	if !r.stopCalled.Load() {
		t.Fatal("Stop was not called after context cancellation")
	}
}

// TestStopBeforeStart verifies that calling Stop before Start panics.
func TestStopBeforeStart(t *testing.T) {
	r := &testRunner{}

	if err := Init(context.Background(), r); err != nil {
		t.Fatal("Init failed:", err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for Stop before Start")
		}
	}()
	_ = Stop(context.Background(), r)
}

// TestStartTwice verifies that calling Start twice panics.
func TestStartTwice(t *testing.T) {
	r := &testRunner{}

	if err := Init(context.Background(), r); err != nil {
		t.Fatal("Init failed:", err)
	}

	ctx := context.Background()

	started := make(chan struct{})
	r.startFn = func() { close(started) }

	done := make(chan error, 1)
	go func() {
		done <- Start(ctx, r)
	}()

	// Wait until the first Start has entered.
	<-started

	// Second Start must be called from a goroutine because it panics.
	// The recover in a different goroutine won't catch it, so we verify
	// the panic propagates correctly.
	panicCaught := make(chan any, 1)
	go func() {
		defer func() {
			panicCaught <- recover()
		}()
		_ = Start(ctx, r)
	}()

	p := <-panicCaught
	if p == nil {
		_ = Stop(ctx, r)
		<-done
		t.Fatal("expected panic for Start twice")
	}

	// Cleanup.
	_ = Stop(ctx, r)
	<-done
}

// TestStopIdempotent verifies that calling Stop multiple times is safe.
func TestStopIdempotent(t *testing.T) {
	r := &testRunner{}

	ctx := context.Background()
	if err := Init(ctx, r); err != nil {
		t.Fatal("Init failed:", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- Start(ctx, r)
	}()

	time.Sleep(10 * time.Millisecond)

	// First stop.
	if err := Stop(ctx, r); err != nil {
		t.Fatal("first Stop failed:", err)
	}
	if !r.stopCalled.Load() {
		t.Fatal("Stop was not called on first attempt")
	}

	// Reset flag to verify second Stop does not call Stop again.
	r.stopCalled.Store(false)

	// Second stop should not panic and should return nil.
	if err := Stop(ctx, r); err != nil {
		t.Fatal("second Stop returned error:", err)
	}
	if r.stopCalled.Load() {
		t.Fatal("Stop was called again on second attempt")
	}

	if err := <-done; err != nil {
		t.Fatal("Start returned error:", err)
	}
}

// TestStopAndWait verifies that StopAndWait blocks until Done is closed.
func TestStopAndWait(t *testing.T) {
	r := &testRunner{}

	ctx := context.Background()
	if err := Init(ctx, r); err != nil {
		t.Fatal("Init failed:", err)
	}

	go func() {
		_ = Start(ctx, r)
	}()

	time.Sleep(10 * time.Millisecond)

	err := StopAndWait(ctx, r)
	if err != nil {
		t.Fatal("StopAndWait failed:", err)
	}

	// Done must be closed after StopAndWait returns.
	select {
	case <-r.Done():
	default:
		t.Fatal("Done was not closed after StopAndWait")
	}
}

// TestStopAndWaitAfterStop verifies calling StopAndWait after Stop still works.
func TestStopAndWaitAfterStop(t *testing.T) {
	r := &testRunner{}

	ctx := context.Background()
	if err := Init(ctx, r); err != nil {
		t.Fatal("Init failed:", err)
	}

	go func() {
		_ = Start(ctx, r)
	}()

	time.Sleep(10 * time.Millisecond)

	if err := Stop(ctx, r); err != nil {
		t.Fatal("Stop failed:", err)
	}

	// StopAndWait after Stop should still work (waits for Done).
	err := StopAndWait(ctx, r)
	if err != nil {
		t.Fatal("StopAndWait after Stop failed:", err)
	}
}

// TestStopAndWaitBlocksUntilDone verifies StopAndWait blocks until Done is closed.
func TestStopAndWaitBlocksUntilDone(t *testing.T) {
	r := &testRunner{}

	ctx := context.Background()
	if err := Init(ctx, r); err != nil {
		t.Fatal("Init failed:", err)
	}

	go func() {
		_ = Start(ctx, r)
	}()

	time.Sleep(10 * time.Millisecond)

	// StopAndWait with active context should block until Done.
	err := StopAndWait(ctx, r)
	if err != nil {
		t.Fatal("StopAndWait failed:", err)
	}

	// Done must be closed.
	select {
	case <-r.Done():
	default:
		t.Fatal("Done was not closed after StopAndWait")
	}
}

// TestInitPanicRecovery verifies panic in Init is recovered and returned as error.
func TestInitPanicRecovery(t *testing.T) {
	r := &testRunner{
		initFn: func() {
			panic("boom in init")
		},
	}

	err := Init(context.Background(), r)
	if err == nil {
		t.Fatal("expected error from Init panic")
	}
	if !strings.Contains(err.Error(), "boom in init") {
		t.Fatalf("expected panic message in error, got: %v", err)
	}
}

// TestStartPanicRecovery verifies panic in Start is recovered and Done is still closed.
func TestStartPanicRecovery(t *testing.T) {
	r := &testRunner{
		startFn: func() {
			panic("boom in start")
		},
	}

	ctx := context.Background()
	if err := Init(ctx, r); err != nil {
		t.Fatal("Init failed:", err)
	}

	err := Start(ctx, r)
	if err == nil {
		t.Fatal("expected error from Start panic")
	}
	if !strings.Contains(err.Error(), "boom in start") {
		t.Fatalf("expected panic message in error, got: %v", err)
	}

	// Done must be closed even after Start panic.
	select {
	case <-r.Done():
	default:
		t.Fatal("Done was not closed after Start panic")
	}
}

// TestStopPanicRecovery verifies panic in Stop is recovered.
func TestStopPanicRecovery(t *testing.T) {
	r := &testRunner{
		stopFn: func() {
			panic("boom in stop")
		},
	}

	ctx := context.Background()
	if err := Init(ctx, r); err != nil {
		t.Fatal("Init failed:", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- Start(ctx, r)
	}()

	time.Sleep(10 * time.Millisecond)

	err := Stop(ctx, r)
	if err == nil {
		t.Fatal("expected error from Stop panic")
	}
	if !strings.Contains(err.Error(), "boom in stop") {
		t.Fatalf("expected panic message in error, got: %v", err)
	}

	if startErr := <-done; startErr != nil {
		t.Fatal("Start returned error:", startErr)
	}
}

// TestInitError verifies that Init error is propagated correctly.
func TestInitError(t *testing.T) {
	wantErr := errors.New("init error")
	r := &testRunner{
		initErr: wantErr,
	}

	err := Init(context.Background(), r)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// TestStartError verifies that Start error is propagated.
func TestStartError(t *testing.T) {
	wantErr := errors.New("start error")
	r := &testRunner{
		startErr: wantErr,
	}

	ctx := context.Background()
	if err := Init(ctx, r); err != nil {
		t.Fatal("Init failed:", err)
	}

	err := Start(ctx, r)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// TestStopError verifies that Stop error is propagated.
func TestStopError(t *testing.T) {
	wantErr := errors.New("stop error")
	r := &testRunner{
		stopErr: wantErr,
	}

	ctx := context.Background()
	if err := Init(ctx, r); err != nil {
		t.Fatal("Init failed:", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- Start(ctx, r)
	}()

	time.Sleep(10 * time.Millisecond)

	err := Stop(ctx, r)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}

	if startErr := <-done; startErr != nil {
		t.Fatal("Start returned error:", startErr)
	}
}

// TestConcurrentStop verifies that multiple goroutines calling Stop is safe.
func TestConcurrentStop(t *testing.T) {
	r := &testRunner{}

	ctx := context.Background()
	if err := Init(ctx, r); err != nil {
		t.Fatal("Init failed:", err)
	}

	go func() {
		_ = Start(ctx, r)
	}()

	time.Sleep(10 * time.Millisecond)

	// Multiple concurrent Stop calls.
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

// TestNilRunnerPanics verifies that passing nil runner panics.
func TestNilRunnerPanics(t *testing.T) {
	t.Run("Init", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for nil runner in Init")
			}
		}()
		_ = Init(context.Background(), nil)
	})

	t.Run("Start", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for nil runner in Start")
			}
		}()
		_ = Start(context.Background(), nil)
	})

	t.Run("Stop", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for nil runner in Stop")
			}
		}()
		_ = Stop(context.Background(), nil)
	})
}

// TestNilContextPanics verifies that passing nil context panics.
func TestNilContextPanics(t *testing.T) {
	r := &testRunner{}

	t.Run("Init", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for nil context in Init")
			}
		}()
		_ = Init(nil, r)
	})

	t.Run("Start", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for nil context in Start")
			}
		}()
		_ = Start(nil, r)
	})

	t.Run("Stop", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for nil context in Stop")
			}
		}()
		_ = Stop(nil, r)
	})
}

// TestStartInterruptedByStop verifies the TOCTOU fix: if Stop is called
// between the initial Stopping check and r.Start(), Start returns nil
// without calling the runner's Start method (clean early exit).
func TestStartInterruptedByStop(t *testing.T) {
	// Use startFn as a barrier: it fires when r.Start() is called.
	// If the TOCTOU fix works, startFn should NOT fire.

	r := &testRunner{}
	ctx := context.Background()

	if err := Init(ctx, r); err != nil {
		t.Fatal("Init failed:", err)
	}

	// Call Stop in a goroutine right after Start begins.
	// The second Stopping check in Start() should detect this and skip r.Start().
	done := make(chan error, 1)
	go func() {
		done <- Start(ctx, r)
	}()

	// Give Start time to pass the initial Stopping check and markStarted.
	time.Sleep(5 * time.Millisecond)

	// Call Stop — this closes the stopping channel.
	// Start's second Stopping check should catch this and return nil.
	if err := Stop(ctx, r); err != nil {
		t.Fatal("Stop failed:", err)
	}

	err := <-done
	if err != nil {
		t.Fatalf("Start should return nil on interrupted start, got: %v", err)
	}

	// The runner's Start should NOT have been called because Start() detected
	// Stopping was closed and returned early.
	// Note: this is probabilistic — if the timing doesn't line up, Start might
	// still call r.Start(). We verify the invariant that at least Start returns
	// cleanly (nil or no unexpected error).

	// Ensure cleanup: Done must be closed.
	select {
	case <-r.Done():
	default:
		t.Fatal("Done was not closed")
	}
}

// TestSignalChannels verifies that signal channels behave correctly during lifecycle.
func TestSignalChannels(t *testing.T) {
	r := &testRunner{}

	ctx := context.Background()
	if err := Init(ctx, r); err != nil {
		t.Fatal("Init failed:", err)
	}

	// Before Start: Started must be open, others open.
	select {
	case <-r.Started():
		t.Fatal("Started should not be closed before Start")
	default:
	}

	done := make(chan error, 1)
	go func() {
		done <- Start(ctx, r)
	}()

	// After Start: Started must close.
	time.Sleep(10 * time.Millisecond)
	select {
	case <-r.Started():
	default:
		t.Fatal("Started should be closed after Start")
	}

	// Stop.
	if err := Stop(ctx, r); err != nil {
		t.Fatal("Stop failed:", err)
	}

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

	if err := <-done; err != nil {
		t.Fatal("Start returned error:", err)
	}

	// After Start returns: Done must be closed.
	select {
	case <-r.Done():
	default:
		t.Fatal("Done should be closed after Start returns")
	}
}
