package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/donkeywon/golib/errs"
)

type Lifecycle interface {
	Init(context.Context) error
	Start(context.Context) error
	Stop(context.Context) error
}

type Signaler interface {
	Started() <-chan struct{}
	Stopping() <-chan struct{}
	StopDone() <-chan struct{}
	Done() <-chan struct{}
}

type marker interface {
	markStarted() bool
	markStopping() bool
	markStopDone() bool
	markDone() bool
}

type Runner interface {
	Lifecycle
	Signaler

	marker
}

func Init(ctx context.Context, r Runner) (err error) {
	if r == nil {
		panic("nil runner")
	}
	if ctx == nil {
		panic("nil context")
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

	return r.Init(ctx)
}

func Start(ctx context.Context, r Runner) (err error) {
	if r == nil {
		panic("nil runner")
	}
	if ctx == nil {
		panic("nil context")
	}

	select {
	case <-r.Stopping():
		panic("start after stopping")
	default:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if !r.markStarted() {
		panic("start again")
	}

	defer func() {
		p := recover()
		if p != nil {
			pe := errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on start runner: %T", r))
			if err == nil {
				err = pe
			} else {
				err = errors.Join(err, pe)
			}
		}

		r.markDone()
	}()

	go func() {
		select {
		case <-r.Stopping():
			return
		case <-r.Done():
			return
		case <-ctx.Done():
			select {
			case <-r.Stopping():
				return
			case <-r.Done():
				return
			default:
			}
			stop(context.WithoutCancel(ctx), r, false)
		}
	}()

	return r.Start(ctx)
}

func Stop(ctx context.Context, r Runner) error {
	return stop(ctx, r, false)
}

func StopAndWait(ctx context.Context, r Runner) error {
	return stop(ctx, r, true)
}

func stop(ctx context.Context, r Runner, wait bool) (err error) {
	if ctx == nil {
		panic("nil context")
	}
	if r == nil {
		panic("nil runner")
	}

	select {
	case <-r.Started():
	default:
		panic("stop before start")
	}

	if !r.markStopping() {
		if wait {
			return waitDone(ctx, r)
		}
		return nil
	}

	err = safeStop(ctx, r)
	r.markStopDone()
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
	started      chan struct{}
	stopping     chan struct{}
	done         chan struct{}
	stopDone     chan struct{}
	startedOnce  sync.Once
	stoppingOnce sync.Once
	doneOnce     sync.Once
	stopDoneOnce sync.Once
	initOnce     sync.Once
}

func (br *Base) init() {
	br.initOnce.Do(func() {
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
	<-br.Stopping()
	return nil
}

func (br *Base) Stop(_ context.Context) error {
	return nil
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

func (br *Base) markStarted() bool {
	br.init()
	return br.closeCh(&br.startedOnce, br.started)
}

func (br *Base) markStopping() bool {
	br.init()
	return br.closeCh(&br.stoppingOnce, br.stopping)
}

func (br *Base) markStopDone() bool {
	br.init()
	return br.closeCh(&br.stopDoneOnce, br.stopDone)
}

func (br *Base) markDone() bool {
	br.init()
	return br.closeCh(&br.doneOnce, br.done)
}

func (br *Base) closeCh(once *sync.Once, ch chan struct{}) bool {
	closed := false
	once.Do(func() {
		close(ch)
		closed = true
	})
	return closed
}
