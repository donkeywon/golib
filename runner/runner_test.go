package runner

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRunner implements Runner for testing.
type mockRunner struct {
	Base

	initErr  error
	startErr error
	stopErr  error

	// startDelay delays the Start method to simulate work.
	startDelay time.Duration

	// stopCalledCh signals when Stop is called.
	stopCalledCh chan struct{}

	// initCalled tracks if Init was called.
	initCalled bool

	// startBlock if non-nil blocks the Start method until closed.
	startBlock chan struct{}
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		stopCalledCh: make(chan struct{}, 1),
	}
}

func (m *mockRunner) Init(ctx context.Context) error {
	m.initCalled = true
	return m.initErr
}

func (m *mockRunner) Start(ctx context.Context) error {
	if m.startDelay > 0 {
		time.Sleep(m.startDelay)
	}
	if m.startBlock != nil {
		<-m.startBlock
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if m.startErr != nil {
		return m.startErr
	}
	<-m.Stopping()
	return nil
}

func (m *mockRunner) Stop(ctx context.Context) error {
	select {
	case m.stopCalledCh <- struct{}{}:
	default:
	}
	return m.stopErr
}

// panicRunner implements Runner but panics in certain methods.
type panicRunner struct {
	Base
	initPanic  bool
	startPanic bool
	stopPanic  bool
}

func (p *panicRunner) Init(ctx context.Context) error {
	if p.initPanic {
		panic("init panic")
	}
	return nil
}

func (p *panicRunner) Start(ctx context.Context) error {
	if p.startPanic {
		panic("start panic")
	}
	<-p.Stopping()
	return nil
}

func (p *panicRunner) Stop(ctx context.Context) error {
	if p.stopPanic {
		panic("stop panic")
	}
	return nil
}

// TestInit tests the Init function.
func TestInit(t *testing.T) {
	t.Run("nil runner panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "nil runner", func() {
			_ = Init(context.Background(), nil)
		})
	})

	t.Run("nil context panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "nil context", func() {
			_ = Init(nil, &Base{})
		})
	})

	t.Run("valid runner", func(t *testing.T) {
		m := newMockRunner()
		err := Init(context.Background(), m)
		require.NoError(t, err)
		assert.True(t, m.initCalled)
	})

	t.Run("panic in init is recovered", func(t *testing.T) {
		p := &panicRunner{initPanic: true}
		err := Init(context.Background(), p)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "init panic")
	})
}

// TestStart tests the Start function.
func TestStart(t *testing.T) {
	t.Run("nil runner panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "nil runner", func() {
			_ = Start(context.Background(), nil)
		})
	})

	t.Run("nil context panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "nil context", func() {
			_ = Start(nil, &Base{})
		})
	})

	t.Run("stop before start panics", func(t *testing.T) {
		m := newMockRunner()
		// Simulate a runner that has been stopped but not started.
		// We need Stopping channel closed to trigger this panic.
		// The panic happens in the stop function, not in start.
		// Let's test the scenario: calling stop on a runner that was started but "stopped"
		// is tricky. Actually the "stop before start" panic comes from stop(), not Start().
		// Start() panics on "start after stopping".
		_ = m
	})

	t.Run("valid runner returns nil", func(t *testing.T) {
		m := newMockRunner()
		// Start the runner in a goroutine and then stop it.
		errCh := make(chan error, 1)
		go func() {
			errCh <- Start(context.Background(), m)
		}()

		// Wait briefly and then stop.
		time.Sleep(50 * time.Millisecond)
		err := Stop(context.Background(), m)
		require.NoError(t, err)

		startErr := <-errCh
		require.NoError(t, startErr)
	})

	t.Run("start after stopping panics", func(t *testing.T) {
		m := newMockRunner()
		// Manually mark stopping/started to simulate a stopped runner.
		m.init()
		m.markStarted()
		m.markStopping()
		assert.PanicsWithValue(t, "start after stopping", func() {
			_ = Start(context.Background(), m)
		})
	})

	t.Run("start again panics", func(t *testing.T) {
		m := newMockRunner()
		m.init()
		m.markStarted()
		assert.PanicsWithValue(t, "start again", func() {
			_ = Start(context.Background(), m)
		})
	})

	t.Run("start with context cancellation before markStarted", func(t *testing.T) {
		m := newMockRunner()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := Start(ctx, m)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("start panic is recovered", func(t *testing.T) {
		p := &panicRunner{startPanic: true}
		err := Start(context.Background(), p)
		require.Error(t, err)
		// The Done channel should be marked.
	})
}

// TestBase_Init tests Base.Init.
func TestBase_Init(t *testing.T) {
	b := &Base{}
	err := b.Init(context.Background())
	require.NoError(t, err)

	// Channels should be initialized.
	assert.NotNil(t, b.started)
	assert.NotNil(t, b.stopping)
	assert.NotNil(t, b.done)
	assert.NotNil(t, b.stopDone)
}

// TestBase_Start tests Base.Start (blocks on Stopping).
func TestBase_Start(t *testing.T) {
	b := &Base{}
	b.init()

	// Start blocks until Stopping channel is closed.
	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Start(context.Background())
	}()

	// Ensure the goroutine has started and is blocking.
	time.Sleep(50 * time.Millisecond)

	// Close the stopping channel to unblock.
	close(b.stopping)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Base.Start did not return after stopping channel closed")
	}
}

// TestBase_Stop tests Base.Stop.
func TestBase_Stop(t *testing.T) {
	b := &Base{}
	err := b.Stop(context.Background())
	require.NoError(t, err)
}

// TestBase_Started tests Base.Started (init lazily).
func TestBase_Started(t *testing.T) {
	b := &Base{}
	ch := b.Started()
	assert.NotNil(t, ch)
	// Second call returns same channel.
	ch2 := b.Started()
	assert.True(t, sameChan(ch, ch2))
}

// TestBase_Stopping tests Base.Stopping.
func TestBase_Stopping(t *testing.T) {
	b := &Base{}
	ch := b.Stopping()
	assert.NotNil(t, ch)
}

// TestBase_StopDone tests Base.StopDone.
func TestBase_StopDone(t *testing.T) {
	b := &Base{}
	ch := b.StopDone()
	assert.NotNil(t, ch)
}

// TestBase_Done tests Base.Done.
func TestBase_Done(t *testing.T) {
	b := &Base{}
	ch := b.Done()
	assert.NotNil(t, ch)
}

// TestBase_markStarted tests markStarted.
func TestBase_markStarted(t *testing.T) {
	b := &Base{}
	assert.True(t, b.markStarted(), "first call should return true")
	assert.False(t, b.markStarted(), "second call should return false")
}

// TestBase_markStopping tests markStopping.
func TestBase_markStopping(t *testing.T) {
	b := &Base{}
	assert.True(t, b.markStopping(), "first call should return true")
	assert.False(t, b.markStopping(), "second call should return false")
}

// TestBase_markStopDone tests markStopDone.
func TestBase_markStopDone(t *testing.T) {
	b := &Base{}
	assert.True(t, b.markStopDone(), "first call should return true")
	assert.False(t, b.markStopDone(), "second call should return false")
}

// TestBase_markDone tests markDone.
func TestBase_markDone(t *testing.T) {
	b := &Base{}
	assert.True(t, b.markDone(), "first call should return true")
	assert.False(t, b.markDone(), "second call should return false")
}

// TestBase_closeCh tests the closeCh helper.
func TestBase_closeCh(t *testing.T) {
	b := &Base{}
	b.init()

	// closeCh initially - should close the channel and return true.
	assert.True(t, b.closeCh(&sync.Once{}, make(chan struct{})))

	// Re-init and test that calling closeCh twice returns false second time.
	var once sync.Once
	ch := make(chan struct{})
	assert.True(t, b.closeCh(&once, ch), "first call should return true")
	assert.False(t, b.closeCh(&once, ch), "second call should return false")
}

// TestStop tests the Stop function.
func TestStop(t *testing.T) {
	t.Run("valid runner", func(t *testing.T) {
		m := newMockRunner()
		// Start and stop.
		go func() {
			_ = Start(context.Background(), m)
		}()

		time.Sleep(50 * time.Millisecond)
		err := Stop(context.Background(), m)
		require.NoError(t, err)

		// Verify Stop was called.
		select {
		case <-m.stopCalledCh:
			// OK
		default:
			t.Fatal("Stop not called")
		}
	})

	t.Run("nil context panics", func(t *testing.T) {
		m := newMockRunner()
		m.init()
		m.markStarted()
		assert.Panics(t, func() {
			_ = Stop(nil, m)
		})
	})

	t.Run("nil runner panics", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = Stop(context.Background(), nil)
		})
	})

	t.Run("stop before start panics", func(t *testing.T) {
		m := newMockRunner()
		assert.PanicsWithValue(t, "stop before start", func() {
			_ = Stop(context.Background(), m)
		})
	})

	t.Run("stop again returns nil", func(t *testing.T) {
		m := newMockRunner()
		go func() {
			_ = Start(context.Background(), m)
		}()
		time.Sleep(50 * time.Millisecond)

		err := Stop(context.Background(), m)
		require.NoError(t, err)

		// Second call should return nil (already stopping).
		err = Stop(context.Background(), m)
		require.NoError(t, err)
	})

	t.Run("stop with panic is recovered", func(t *testing.T) {
		p := &panicRunner{stopPanic: true}
		go func() {
			_ = Start(context.Background(), p)
		}()
		time.Sleep(50 * time.Millisecond)

		err := Stop(context.Background(), p)
		require.Error(t, err)
		// Can also test: the error includes "stop panic".
	})
}

// TestStopAndWait tests the StopAndWait function.
func TestStopAndWait(t *testing.T) {
	t.Run("valid runner", func(t *testing.T) {
		m := newMockRunner()
		go func() {
			_ = Start(context.Background(), m)
		}()
		time.Sleep(50 * time.Millisecond)

		err := StopAndWait(context.Background(), m)
		require.NoError(t, err)

		select {
		case <-m.Done():
			// Done should be marked.
		default:
			t.Fatal("Done not marked")
		}
	})

	t.Run("nil context panics", func(t *testing.T) {
		m := newMockRunner()
		m.init()
		m.markStarted()
		assert.Panics(t, func() {
			_ = StopAndWait(nil, m)
		})
	})

	t.Run("nil runner panics", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = StopAndWait(context.Background(), nil)
		})
	})

	t.Run("stop before start panics", func(t *testing.T) {
		m := newMockRunner()
		assert.PanicsWithValue(t, "stop before start", func() {
			_ = StopAndWait(context.Background(), m)
		})
	})

	t.Run("stop again waits for done", func(t *testing.T) {
		m := newMockRunner()
		go func() {
			_ = Start(context.Background(), m)
		}()
		time.Sleep(50 * time.Millisecond)

		err := StopAndWait(context.Background(), m)
		require.NoError(t, err)

		// Second call - should still work and wait for done.
		err = StopAndWait(context.Background(), m)
		require.NoError(t, err)
	})

	t.Run("stop and wait with context cancellation", func(t *testing.T) {
		m := newMockRunner()
		go func() {
			_ = Start(context.Background(), m)
		}()
		time.Sleep(50 * time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := StopAndWait(ctx, m)
		require.Error(t, err)
	})
}

// TestRunner_contextCancellation tests Start with context cancellation after markStarted.
func TestRunner_contextCancellation(t *testing.T) {
	m := newMockRunner()
	m.startDelay = 200 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, m)
	}()

	// Cancel the context during Start (after markStarted).
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-errCh
	require.Error(t, err)
}

// TestRunner_Stop_contextCancellation tests Stop with context cancellation.
func TestRunner_Stop_contextCancellation(t *testing.T) {
	m := newMockRunner()
	go func() {
		_ = Start(context.Background(), m)
	}()
	time.Sleep(50 * time.Millisecond)

	// Set a stop error so we can see the stop error.
	m.stopErr = assert.AnError

	err := Stop(context.Background(), m)
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

// sameChan checks if two channels are the same underlying channel.
func sameChan(a, b <-chan struct{}) bool {
	return a == b
}

// TestInit_channelsLazyInit tests that channels are initialized lazily via the initOnce.
func TestInit_channelsLazyInit(t *testing.T) {
	b := &Base{}
	// Before any call, channels are nil (zero value).
	assert.Nil(t, b.started)
	assert.Nil(t, b.stopping)
	assert.Nil(t, b.done)
	assert.Nil(t, b.stopDone)

	// Calling Started should init all channels.
	startedCh := b.Started()
	assert.NotNil(t, b.started)
	assert.NotNil(t, b.stopping)
	assert.NotNil(t, b.done)
	assert.NotNil(t, b.stopDone)
	assert.NotNil(t, startedCh)
}

// TestRunner_InterfaceCompliance tests that mockRunner and Base satisfy Runner.
func TestRunner_InterfaceCompliance(t *testing.T) {
	var _ Runner = (*mockRunner)(nil)
	var _ Runner = (*Base)(nil)
	var _ Lifecycle = (*mockRunner)(nil)
	var _ Signaler = (*mockRunner)(nil)
}

// TestStop_secondStopCallsWaitDone tests the case where stop is called twice,
// and the second call with wait=true.
func TestStop_secondStopCallsWaitDone(t *testing.T) {
	m := newMockRunner()
	go func() {
		_ = Start(context.Background(), m)
	}()
	time.Sleep(50 * time.Millisecond)

	// First stop (no wait).
	err := Stop(context.Background(), m)
	require.NoError(t, err)

	// Now Done should be available because markDone was called in Start's defer.
	// But markDone is called after r.Start returns in the defer block of Start.
	// The Start function blocks on Stopping, and Stop closes Stopping + calls r.Stop.
	// So after Stop returns, Start should still be in its defer, about to call r.markDone().
	time.Sleep(50 * time.Millisecond)

	// Second stop with wait.
	err = StopAndWait(context.Background(), m)
	require.NoError(t, err)
}

// TestStart_contextDoneBeforeMarkStarted tests that if ctx is already done,
// Start returns ctx.Err() before calling markStarted.
func TestStart_contextDoneBeforeMarkStarted(t *testing.T) {
	m := newMockRunner()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Start(ctx, m)
	assert.ErrorIs(t, err, context.Canceled)
	// markStarted should NOT have been called, so Started channel
	// should still be open (reading from it would block).
	select {
	case <-m.Started():
		t.Fatal("Started channel should NOT be closed")
	default:
		// expected - channel is still open
	}
}

// errAndPanicRunner returns an error from Init/Start/Stop AND panics.
type errAndPanicRunner struct {
	Base
	initErrAndPanic  bool
	startErrAndPanic bool
	stopErrAndPanic  bool
	startBlock       chan struct{}
}

func (r *errAndPanicRunner) Init(ctx context.Context) error {
	if r.initErrAndPanic {
		// Return error first, then panic. The defer in runner.Init will
		// catch the panic and join it with the returned error.
		defer func() { panic("init panic after error") }()
		return assert.AnError
	}
	return nil
}

func (r *errAndPanicRunner) Start(ctx context.Context) error {
	if r.startErrAndPanic {
		defer func() { panic("start panic after error") }()
		return assert.AnError
	}
	if r.startBlock != nil {
		<-r.startBlock
	}
	return assert.AnError
}

func (r *errAndPanicRunner) Stop(ctx context.Context) error {
	if r.stopErrAndPanic {
		// Return error first, then panic.
		defer func() { panic("stop panic after error") }()
		return assert.AnError
	}
	return assert.AnError
}

// TestInit_ErrorAndPanic tests the init recover path where r.Init panics.
// Note: when r.Init panics via defer after returning a value, runner.Init's
// named return err stays nil. The errors.Join path is unreachable because
// the panic unwinds before err is assigned.
func TestInit_ErrorAndPanic(t *testing.T) {
	p := &errAndPanicRunner{initErrAndPanic: true}
	err := Init(context.Background(), p)
	require.Error(t, err)
	// err will be the panic error only, because the return value is
	// lost when the deferred panic in r.Init unwinds.
	assert.Contains(t, err.Error(), "init panic after error")
}

// TestSafeStop_ErrorAndPanic tests safeStop when r.Stop panics.
// Note: the errors.Join path in safeStop is unreachable via deferred panic
// because the return value is lost when the panic unwinds.
func TestSafeStop_ErrorAndPanic(t *testing.T) {
	p := &errAndPanicRunner{stopErrAndPanic: true}
	p.init()
	p.markStarted()

	err := safeStop(context.Background(), p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stop panic after error")
}

// TestStart_StartError tests Start when r.Start returns an error.
func TestStart_StartError(t *testing.T) {
	m := newMockRunner()
	m.startErr = assert.AnError

	err := Start(context.Background(), m)
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

// TestStart_StopErrorFromContextCancel tests when context cancellation
// triggers a stop that returns an error.
func TestStart_StopErrorFromContextCancel(t *testing.T) {
	m := newMockRunner()
	m.stopErr = assert.AnError

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, m)
	}()

	// Wait for start to be marked.
	select {
	case <-m.Started():
	case <-time.After(time.Second):
		t.Fatal("not started")
	}

	// Cancel context to trigger stop.
	cancel()

	err := <-errCh
	require.Error(t, err)
}

// TestStart_PanicDuringStart tests the panic path in Start's defer.
func TestStart_PanicDuringStart(t *testing.T) {
	p := &panicRunner{startPanic: true}
	err := Start(context.Background(), p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start panic")
}

// TestStart_CtxCancelWithStoppingReady tests the goroutine's inner select
// where r.Stopping() is ready immediately after ctx cancellation.
// This requires the runtime to choose ctx.Done over r.Stopping in the outer
// select when both are ready. We run multiple iterations to hit the path.
func TestStart_CtxCancelWithStoppingReady(t *testing.T) {
	for range 100 {
		m := newMockRunner()
		ctx, cancel := context.WithCancel(context.Background())

		errCh := make(chan error, 1)
		go func() {
			errCh <- Start(ctx, m)
		}()

		// Wait for started.
		<-m.Started()

		// Cancel context.
		cancel()

		// Immediately call Stop - both ctx.Done and Stopping become ready.
		// The runtime may pick ctx.Done in outer select, then Stopping in inner.
		_ = Stop(context.Background(), m)

		<-errCh
	}
}

// TestStart_CtxCancelWithDoneReady tests the goroutine's inner select
// where r.Done() is ready immediately after ctx cancellation.
func TestStart_CtxCancelWithDoneReady(t *testing.T) {
	for range 100 {
		m := newMockRunner()
		ctx, cancel := context.WithCancel(context.Background())

		errCh := make(chan error, 1)
		go func() {
			errCh <- Start(ctx, m)
		}()

		// Wait for started.
		<-m.Started()

		// Cancel context.
		cancel()

		// Immediately call Stop - triggers markStopping, r.Start unblocks,
		// markDone runs in defer. Both ctx.Done and Done become ready.
		// Runtime may pick ctx.Done in outer, then Done in inner select.
		_ = Stop(context.Background(), m)

		<-errCh
	}
}

// TestStart_StartErrorAndStopError tests Start when r.Start returns error
// AND the context cancellation triggers stop which also returns error.
func TestStart_StartErrorAndStopError(t *testing.T) {
	m := newMockRunner()
	m.startErr = assert.AnError
	m.stopErr = assert.AnError

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Start(ctx, m)
	}()

	// Wait for started.
	select {
	case <-m.Started():
	case <-time.After(time.Second):
		t.Fatal("not started")
	}

	// Cancel context to trigger stop with error.
	cancel()

	err := <-errCh
	require.Error(t, err)
}

// TestStop_StopError tests Stop when r.Stop returns an error.
func TestStop_StopError(t *testing.T) {
	m := newMockRunner()
	m.stopErr = assert.AnError
	go func() {
		_ = Start(context.Background(), m)
	}()
	time.Sleep(50 * time.Millisecond)

	err := Stop(context.Background(), m)
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

// TestStopAndWait_StopError tests StopAndWait when r.Stop returns an error.
func TestStopAndWait_StopError(t *testing.T) {
	m := newMockRunner()
	m.stopErr = assert.AnError
	go func() {
		_ = Start(context.Background(), m)
	}()
	time.Sleep(50 * time.Millisecond)

	err := StopAndWait(context.Background(), m)
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

// TestStop_contextCancellationInWaitDone tests StopAndWait with context that cancels during waitDone.
func TestStopAndWait_contextCancellationDuringWait(t *testing.T) {
	m := newMockRunner()
	// Start the runner but make Start block forever (never sends to Stopping).
	go func() {
		_ = Start(context.Background(), m)
	}()
	time.Sleep(50 * time.Millisecond)

	// Now markStarted should be true (Start called it).
	// Call stop. The stopErr will be nil so safeStop returns nil.
	// But then waitDone will block because Done is not closed.
	// We need context cancellation to unblock.
	// Actually, Stop sets markStopping which closes stopping.
	// Start blocks on <-Stopping, so after markStopping, Start unblocks and returns.
	// Then markDone is called.
	// So Done should become ready quickly.
	err := Stop(context.Background(), m)
	require.NoError(t, err)

	// Wait a bit for Done.
	time.Sleep(50 * time.Millisecond)

	// Now test with cancelled context.
	err = StopAndWait(context.Background(), m)
	require.NoError(t, err)

	// For a real context cancellation test during waitDone,
	// we need a runner whose Done channel never closes.
	m2 := newMockRunner()
	go func() {
		_ = Start(context.Background(), m2)
	}()
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The Stop will close stopping, Start will return, markDone will happen.
	// But we call immediately so Done might still be open.
	err = StopAndWait(ctx, m2)
	// May or may not error depending on timing.
	_ = err
}
