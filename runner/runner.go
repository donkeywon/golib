package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/kvs"
	"github.com/donkeywon/golib/log"
)

var Create = newBase // allow to override

type Runner interface {
	kvs.NoErrKVS
	log.Logger

	Init() error
	Start() error
	Stop() error

	Name() string
	SetName(string)
	Ctx() context.Context
	SetCtx(context.Context)
	Cancel()

	Inherit(Runner)
	Parent() Runner

	MarkStarted() bool
	MarkStopping() bool
	MarkStopDone() bool
	MarkDone() bool
	Started() <-chan struct{}
	Stopping() <-chan struct{}
	StopDone() <-chan struct{}
	Done() <-chan struct{}

	AppendError(err ...error)
	Err() error

	WithLoggerFrom(r Runner, kvs ...any)
}

// Init a runner.
func Init(r Runner) (err error) {
	if r == nil {
		panic("nil runner")
	}
	if r.Ctx() == nil {
		panic("nil runner context")
	}

	defer func() {
		p := recover()
		if p != nil {
			if err == nil {
				err = errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on %s init", r.Name()))
			} else {
				err = errors.Join(err, errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on %s init", r.Name())))
			}
		}
	}()
	r.Info("init")
	err = r.Init()
	if err != nil {
		r.Cancel()
		return
	}
	r.Info("init done")
	return
}

// Run a runner and wait it done.
func Run(r Runner) error {
	if r == nil {
		panic("nil runner")
	}
	if r.Ctx() == nil {
		panic("nil runner context")
	}
	run(r)
	return r.Err()
}

// Start a runner in the background.
func Start(r Runner) {
	if r == nil {
		panic("nil runner")
	}
	if r.Ctx() == nil {
		panic("nil runner context")
	}
	go run(r)
}

func run(r Runner) {
	if !r.MarkStarted() {
		r.Info("already started")
		return
	}

	defer func() {
		err := recover()
		if err != nil {
			r.AppendError(errs.PanicToErrWithMsg(err, fmt.Sprintf("panic on %s running", r.Name())))
		}

		if r.MarkStopping() {
			// At this point
			// 1. stopping or canceled before call runner.Run(r)
			// 2. done before call runner.Stop(r)
			// both need to markStopDone
			r.MarkStopDone()
		}
		<-r.StopDone()
		r.Info("done")
		r.MarkDone()
		r.Cancel()
	}()

	select {
	case <-r.Stopping():
		r.Info("already stopping before start")
		return
	case <-r.Ctx().Done():
		r.Info("already canceled before start")
		return
	default:
	}

	go func() {
		select {
		case <-r.Stopping():
			return
		case <-r.Ctx().Done():
			// 当r.Run()立即返回的情况下，上方的defer函数里的r.markStopping()和r.Cancel()可能会在此goroutine执行前就全部执行结束
			// 这种情况下r.Stopping()和r.Ctx().Done()这两个case会同时可进入，会随机进入一个，偶尔就会进入到r.Ctx().Done()这个case里
			// 所以偶尔就会看到明明没有调r.Cancel()，但是却走到r.Ctx().Done()的case里
			// 在这个case里再判断一次r.Stopping()
			select {
			case <-r.Stopping():
				return
			default:
			}
			r.Info("received cancel, stopping")
			stop(r, false)
		}
	}()

	r.Info("starting")
	r.AppendError(r.Start())
}

// Stop runner, in most scenario, Stop is notification action to notify the Runner to stop.
func Stop(r Runner) {
	stop(r, false)
}

// StopAndWait notify Runner to stop and wait it done.
func StopAndWait(r Runner) {
	stop(r, true)
}

func stop(r Runner, wait bool) {
	if !r.MarkStopping() {
		r.Info("already stopping", "wait", wait)
		return
	}

	r.Info("stopping", "wait", wait)

	select {
	case <-r.Started():
		safeStop(r)
		r.Info("stop done")
		r.MarkStopDone()
		if wait {
			<-r.Done()
		}
	default:
		// 这里不直接return的原因是：
		// 有些struct组合了Runner接口，但是并不需要Start，只是依赖Runner的一些公共方法
		// 例如Log相关方法、Value相关方法，Ctx和事件通知相关方法
		// 所以这里支持在Runner没有Start的情况下做Stop操作
		// 这种情况下必须要调用runner.Init执行初始化，Init、Start、Stop都可以不用实现.
		r.Info("stopping before started")
		safeStop(r)
		r.Info("stop done before started")
		r.MarkStopDone()
		r.MarkDone()
	}
}

func safeStop(r Runner) {
	defer func() {
		err := recover()
		if err != nil {
			r.AppendError(errs.PanicToErrWithMsg(err, "panic on stopping"))
		}
	}()
	r.AppendError(r.Stop())
}

type baseRunner struct {
	kvs.NoErrKVS
	log.Logger

	initialized  atomic.Bool
	ctx          context.Context
	cancel       context.CancelFunc
	err          error
	parent       Runner
	started      chan struct{}
	done         chan struct{}
	stopDone     chan struct{}
	stopping     chan struct{}
	name         string
	startedOnce  sync.Once
	doneOnce     sync.Once
	stopDoneOnce sync.Once
	stoppingOnce sync.Once
	cancelOnce   sync.Once
	errMu        sync.Mutex
}

func newBase(name string) Runner {
	br := &baseRunner{
		Logger:   log.NewNopLogger(),
		name:     name,
		started:  make(chan struct{}),
		stopping: make(chan struct{}),
		stopDone: make(chan struct{}),
		done:     make(chan struct{}),
		NoErrKVS: kvs.NewMapKVS(),
	}
	return br
}

func (br *baseRunner) markInitialized() bool {
	return br.initialized.CompareAndSwap(false, true)
}

func (br *baseRunner) SetName(n string) {
	if br.initialized.Load() {
		panic("set name after initialized")
	}
	br.name = n
}

func (br *baseRunner) Name() string {
	return br.name
}

func (br *baseRunner) Init() error {
	if !br.markInitialized() {
		panic("init twice")
	}
	if br.Logger == nil {
		br.Logger = log.NewNopLogger()
	}
	if br.started == nil {
		br.started = make(chan struct{})
	}
	if br.stopping == nil {
		br.stopping = make(chan struct{})
	}
	if br.stopDone == nil {
		br.stopDone = make(chan struct{})
	}
	if br.done == nil {
		br.done = make(chan struct{})
	}
	if br.NoErrKVS == nil {
		br.NoErrKVS = kvs.NewMapKVS()
	}
	return nil
}

func (br *baseRunner) Start() error {
	<-br.Stopping()
	return nil
}

func (br *baseRunner) Stop() error {
	return nil
}

func (br *baseRunner) Inherit(r Runner) {
	if br.parent != nil {
		panic("inherit twice")
	}
	if br.initialized.Load() {
		panic("inherit after initialized")
	}
	br.WithLoggerFrom(r)
	if br.ctx == nil {
		br.SetCtx(r.Ctx())
	}
	br.parent = r
}

func (br *baseRunner) Parent() Runner {
	return br.parent
}

func (br *baseRunner) SetCtx(ctx context.Context) {
	if br.initialized.Load() {
		panic("set context after initialized")
	}
	if ctx == nil {
		panic("nil context")
	}
	if br.ctx != nil {
		panic("context already set")
	}
	br.ctx, br.cancel = context.WithCancel(ctx)
}

func (br *baseRunner) Cancel() {
	br.cancelOnce.Do(func() {
		if br.cancel != nil {
			br.Debug("cancel")
			br.cancel()
		}
	})
}

func (br *baseRunner) Ctx() context.Context {
	return br.ctx
}

func (br *baseRunner) Started() <-chan struct{} {
	return br.started
}

func (br *baseRunner) Stopping() <-chan struct{} {
	return br.stopping
}

func (br *baseRunner) StopDone() <-chan struct{} {
	return br.stopDone
}

func (br *baseRunner) Done() <-chan struct{} {
	return br.done
}

func (br *baseRunner) WithLoggerFrom(r Runner, kvs ...any) {
	br.Logger = r.WithLoggerName(br.Name())
	br.WithLoggerFields(kvs...)
}

func (br *baseRunner) MarkStarted() bool {
	marked := false
	br.startedOnce.Do(func() {
		close(br.started)
		marked = true
	})
	br.Debug("mark started", "marked", marked)
	return marked
}

func (br *baseRunner) MarkStopping() bool {
	marked := false
	br.stoppingOnce.Do(func() {
		close(br.stopping)
		marked = true
	})
	br.Debug("mark stopping", "marked", marked)
	return marked
}

func (br *baseRunner) MarkStopDone() bool {
	marked := false
	br.stopDoneOnce.Do(func() {
		close(br.stopDone)
		marked = true
	})
	br.Debug("mark stop done", "marked", marked)
	return marked
}

func (br *baseRunner) MarkDone() bool {
	marked := false
	br.doneOnce.Do(func() {
		close(br.done)
		marked = true
	})
	br.Debug("mark done", "marked", marked)
	return marked
}

func (br *baseRunner) AppendError(err ...error) {
	br.errMu.Lock()
	defer br.errMu.Unlock()
	var res []error
	switch et := br.err.(type) {
	case interface{ UnWrap() []error }:
		res = make([]error, 0, len(et.UnWrap())+len(err))
		res = append(res, et.UnWrap()...)
	default:
		res = make([]error, 0, 1+len(err))
		res = append(res, br.err)
	}
	res = append(res, err...)
	br.err = errors.Join(res...)
}

func (br *baseRunner) Err() error {
	br.errMu.Lock()
	defer br.errMu.Unlock()
	return br.err
}
