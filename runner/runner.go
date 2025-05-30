package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"

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
	initCtx()

	Inherit(Runner)

	Started() <-chan struct{}
	Stopping() <-chan struct{}
	StopDone() <-chan struct{}
	Done() <-chan struct{}
	markStarted() bool
	markStopping() bool
	markStopDone() bool
	markDone() bool

	GetChild(string) Runner
	Children() []Runner
	AppendRunner(Runner) bool
	WaitChildrenDone()
	SetParent(Runner)
	Parent() Runner
	OnChildDone(Runner) error

	AppendError(err ...error)
	Err() error
	SelfErr() error
	ChildrenErr() error

	WithLoggerFrom(r Runner, kvs ...any)
}

// Init a runner and init its children in order.
func Init(r Runner) (err error) {
	defer func() {
		p := recover()
		if p != nil {
			err = errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on %s init", r.Name()))
		}
	}()
	r.initCtx()
	r.Info("init")
	err = r.Init()
	if err != nil {
		r.Cancel()
		return
	}
	r.Info("init done")
	for _, child := range r.Children() {
		child.Inherit(r)
		child.SetParent(r)
		err = Init(child)
		if err != nil {
			r.Cancel()
			return
		}
	}
	return
}

// Start a runner and wait it done.
func Start(r Runner) {
	if !r.markStarted() {
		r.Info("already started")
		return
	}

	defer func() {
		err := recover()
		if err != nil {
			r.AppendError(errs.PanicToErrWithMsg(err, fmt.Sprintf("panic on %s running", r.Name())))
		}

		if r.markStopping() {
			// At this point
			// 1. stopping or canceled before call runner.Start(r)
			// 2. done before call runner.Stop(r)
			// both need to markStopDone
			r.markStopDone()
		}
		r.WaitChildrenDone()
		<-r.StopDone()
		r.Info("done")
		r.markDone()
		r.Cancel()
		if r.Parent() != nil {
			func() {
				defer func() {
					err = recover()
					if err != nil {
						r.Parent().AppendError(errs.PanicToErrWithMsg(err, fmt.Sprintf("panic on %s OnChildDone: %s", r.Parent().Name(), r.Name())))
					}
				}()
				r.Parent().AppendError(r.Parent().OnChildDone(r))
			}()
		}
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
			// 当r.Start()立即返回的情况下，上方的defer函数里的r.markStopping()和r.Cancel()可能会在此goroutine执行前就全部执行结束
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

// StartBG start a runner and its children in the background.
func StartBG(r Runner) {
	go Start(r)
	for _, c := range r.Children() {
		go Start(c)
	}
}

// Stop a runner, in most scenario, Stop is a notification action, used to notify the Runner to stop.
func Stop(r Runner) {
	stopRunnerAndChildren(r, false)
}

// StopAndWait notify Runner to stop, and notify the Runner's children
// in reverse order to stop and wait for the children to finish.
func StopAndWait(r Runner) {
	stopRunnerAndChildren(r, true)
}

func stopRunnerAndChildren(r Runner, wait bool) {
	stop(r, false)
	if len(r.Children()) > 0 {
		for i := len(r.Children()) - 1; i >= 0; i-- {
			stopRunnerAndChildren(r.Children()[i], wait)
			if wait {
				<-r.Children()[i].Done()
			}
		}
	}

	if wait {
		<-r.Done()
	}
}

func stop(r Runner, wait bool) {
	if !r.markStopping() {
		r.Info("already stopping")
		return
	}

	r.Info("stopping")

	select {
	case <-r.Started():
		safeStop(r)
		r.Info("stop done")
		r.markStopDone()
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
		r.markStopDone()
		r.markDone()
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

	ctx          context.Context
	cancel       context.CancelFunc
	err          error
	parent       Runner
	started      chan struct{}
	childrenMap  map[string]Runner
	done         chan struct{}
	stopDone     chan struct{}
	stopping     chan struct{}
	name         string
	children     []Runner
	startedOnce  sync.Once
	doneOnce     sync.Once
	stopDoneOnce sync.Once
	stoppingOnce sync.Once
	cancelOnce   sync.Once
	errMu        sync.Mutex
}

func newBase(name string) Runner {
	br := &baseRunner{
		Logger:      log.NewNopLogger(),
		name:        name,
		started:     make(chan struct{}),
		stopping:    make(chan struct{}),
		stopDone:    make(chan struct{}),
		done:        make(chan struct{}),
		childrenMap: make(map[string]Runner),
		NoErrKVS:    kvs.NewMapKVS(),
	}
	return br
}

func (br *baseRunner) SetName(n string) {
	br.name = n
}

func (br *baseRunner) Name() string {
	return br.name
}

func (br *baseRunner) Init() error {
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
	if br.childrenMap == nil {
		br.childrenMap = make(map[string]Runner)
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
	br.WithLoggerFrom(r)
	if br.ctx == nil {
		br.SetCtx(r.Ctx())
	}
}

func (br *baseRunner) SetCtx(ctx context.Context) {
	if ctx == nil {
		panic("nil context")
	}
	br.ctx = ctx
}

func (br *baseRunner) Cancel() {
	br.cancelOnce.Do(func() {
		if br.cancel != nil {
			br.Debug("cancel")
			br.cancel()
		}
	})
}

func (br *baseRunner) initCtx() {
	if br.ctx == nil {
		br.ctx = context.Background()
	}
	br.ctx, br.cancel = context.WithCancel(br.ctx)
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

func (br *baseRunner) markStarted() bool {
	marked := false
	br.startedOnce.Do(func() {
		close(br.started)
		marked = true
	})
	return marked
}

func (br *baseRunner) markStopping() bool {
	marked := false
	br.stoppingOnce.Do(func() {
		close(br.stopping)
		marked = true
	})
	return marked
}

func (br *baseRunner) markStopDone() bool {
	marked := false
	br.stopDoneOnce.Do(func() {
		close(br.stopDone)
		marked = true
	})
	return marked
}

func (br *baseRunner) markDone() bool {
	marked := false
	br.doneOnce.Do(func() {
		close(br.done)
		marked = true
	})
	return marked
}

func (br *baseRunner) GetChild(name string) Runner {
	return br.childrenMap[name]
}

func (br *baseRunner) Children() []Runner {
	return br.children
}

func (br *baseRunner) WaitChildrenDone() {
	for _, child := range br.Children() {
		select {
		case <-child.Started():
			<-child.Done()
		default:
			continue
		}
	}
}

func (br *baseRunner) SetParent(r Runner) {
	br.parent = r
}

func (br *baseRunner) Parent() Runner {
	return br.parent
}

func (br *baseRunner) OnChildDone(_ Runner) error {
	return nil
}

func (br *baseRunner) AppendRunner(r Runner) bool {
	if r == nil {
		return false
	}

	if _, exists := br.childrenMap[r.Name()]; exists {
		return false
	}
	br.childrenMap[r.Name()] = r
	br.children = append(br.children, r)
	return true
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
	if len(br.Children()) == 0 {
		return br.SelfErr()
	}
	return errors.Join(br.SelfErr(), br.ChildrenErr())
}

func (br *baseRunner) ChildrenErr() error {
	if len(br.Children()) == 0 {
		return nil
	}
	var err error
	for _, child := range br.Children() {
		err = errors.Join(err, child.Err())
	}
	return err
}

func (br *baseRunner) SelfErr() error {
	br.errMu.Lock()
	defer br.errMu.Unlock()
	return br.err
}
