package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/logs"
)

type Lifecycle interface {
	Init(context.Context) error
	Start(context.Context) error
	Stop(context.Context) error
}

type Signaler interface {
	MarkInitialized() bool
	MarkStarted() bool
	MarkStopping() bool
	MarkStopDone() bool
	MarkDone() bool
	Initialized() <-chan struct{}
	Started() <-chan struct{}
	Stopping() <-chan struct{}
	StopDone() <-chan struct{}
	Done() <-chan struct{}
}

type Runner interface {
	Lifecycle
	Signaler
}

// Init a runner.
func Init(ctx context.Context, r Runner) (err error) {
	if r == nil {
		panic("nil runner")
	}
	if ctx == nil {
		panic("nil context")
	}
	if !r.MarkInitialized() {
		panic("init again")
	}

	defer func() {
		p := recover()
		if p != nil {
			pe := errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on init runner: %T", r))
			if err == nil {
				err = pe
			} else {
				err = errors.Join(err, pe)
			}
		}
	}()

	l := logs.FromCtx(ctx)
	l.Info("init")
	err = r.Init(ctx)
	if err != nil {
		return
	}
	l.Info("init done")
	return
}

// Run a runner and wait it done.
func Start(ctx context.Context, r Runner) (err error) {
	if r == nil {
		panic("nil runner")
	}
	if ctx == nil {
		panic("nil context")
	}

	select {
	case <-r.Initialized():
	default:
		panic("start before initialized")
	}

	select {
	case <-r.Stopping():
		panic("start after stopping")
	default:
	}

	if !r.MarkStarted() {
		panic("run again")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	l := logs.FromCtx(ctx)
	defer func() {
		p := recover()
		if p != nil {
			pe := errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on run runner: %T", r))
			if err == nil {
				err = pe
			} else {
				err = errors.Join(err, pe)
			}
		}

		l.Info("done")
		r.MarkDone()
	}()

	go func() {
		select {
		case <-r.Stopping():
			return
		case <-ctx.Done():
			select {
			case <-r.Stopping():
				return
			default:
			}
			l.Info("canceling")
			stop(context.Background(), r, false)
		}
	}()

	l.Info("starting")
	return r.Start(ctx)
}

// Stop runner, in most scenario, Stop is notification action to notify the Runner to stop.
func Stop(ctx context.Context, r Runner) error {
	return stop(ctx, r, false)
}

// StopAndWait notify Runner to stop and wait it done.
func StopAndWait(ctx context.Context, r Runner) error {
	return stop(ctx, r, true)
}

func stop(ctx context.Context, r Runner, wait bool) (err error) {
	select {
	case <-r.Started():
	default:
		panic("stop before start")
	}

	l := logs.FromCtx(ctx)
	if !r.MarkStopping() {
		l.Info("already stopping", "wait", wait)
		return waitDone(ctx, r)
	}

	l.Info("stopping", "wait", wait)
	err = safeStop(ctx, r)
	l.Info("stop done")
	r.MarkStopDone()
	if wait {
		waitErr := waitDone(ctx, r)
		if waitErr != nil {
			err = errors.Join(err, errs.Wrapf(waitErr, "wait runner done failed"))
		}
	}
	return
}

func safeStop(ctx context.Context, r Runner) (err error) {
	defer func() {
		p := recover()
		if p == nil {
			return
		}

		pe := errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on stop runner: %T", r))
		if err == nil {
			err = pe
		} else {
			err = errors.Join(err, pe)
		}
	}()
	return r.Stop(ctx)
}

func waitDone(ctx context.Context, r Runner) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.Done():
		return nil
	}
}

type Base struct {
	initialized     chan struct{}
	started         chan struct{}
	stopping        chan struct{}
	done            chan struct{}
	stopDone        chan struct{}
	initializedOnce sync.Once
	startedOnce     sync.Once
	stoppingOnce    sync.Once
	doneOnce        sync.Once
	stopDoneOnce    sync.Once
	initOnce        sync.Once
}

func (br *Base) init() {
	br.initOnce.Do(func() {
		br.initialized = make(chan struct{})
		br.started = make(chan struct{})
		br.stopping = make(chan struct{})
		br.done = make(chan struct{})
		br.stopDone = make(chan struct{})
	})
}

func (br *Base) Init(_ context.Context) error {
	br.init()
	return nil
}

func (br *Base) Start(_ context.Context) error {
	br.init()
	<-br.Stopping()
	return nil
}

func (br *Base) Stop(_ context.Context) error {
	br.init()
	return nil
}

func (br *Base) Initialized() <-chan struct{} {
	br.init()
	return br.initialized
}

func (br *Base) Started() <-chan struct{} {
	br.init()
	return br.started
}

func (br *Base) Stopping() <-chan struct{} {
	br.init()
	return br.stopping
}

func (br *Base) StopDone() <-chan struct{} {
	br.init()
	return br.stopDone
}

func (br *Base) Done() <-chan struct{} {
	br.init()
	return br.done
}

func (br *Base) MarkInitialized() bool {
	return br.closeCh(&br.initializedOnce, br.initialized)
}

func (br *Base) MarkStarted() bool {
	return br.closeCh(&br.startedOnce, br.started)
}

func (br *Base) MarkStopping() bool {
	return br.closeCh(&br.stoppingOnce, br.stopping)
}

func (br *Base) MarkStopDone() bool {
	return br.closeCh(&br.stopDoneOnce, br.stopDone)
}

func (br *Base) MarkDone() bool {
	return br.closeCh(&br.doneOnce, br.done)
}

func (br *Base) closeCh(once *sync.Once, ch chan struct{}) bool {
	br.init()
	closed := false
	once.Do(func() {
		close(ch)
		closed = true
	})
	return closed
}
