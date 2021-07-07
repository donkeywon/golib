package runner

import (
	"context"
	"errors"
	"sync"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/log"
	"go.uber.org/zap"
)

type Runner interface {
	kvs

	Init() error
	Start() error
	Stop() error

	Name() string
	SetName(string)
	Ctx() context.Context
	SetCtx(context.Context)
	Cancel()

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

	getLogger() *zap.Logger
	WithLoggerFrom(r Runner, kvs ...any)
	WithLoggerFields(kvs ...any)
	Debug(msg string, kvs ...any)
	Info(msg string, kvs ...any)
	Warn(msg string, kvs ...any)
	Error(msg string, err error, kvs ...any)
}

func Inherit(to Runner, from Runner) {
	to.WithLoggerFrom(from)
	to.SetCtx(from.Ctx())
}

func Init(r Runner) error {
	var err error
	r.Info("init")
	if r.Ctx() == nil {
		r.SetCtx(context.Background())
	}
	err = r.Init()
	if err != nil {
		return err
	}
	r.Info("init done")
	for _, child := range r.Children() {
		Inherit(child, r)
		child.SetParent(r)
		err = Init(child)
		if err != nil {
			return err
		}
	}
	return nil
}

func Start(r Runner) {
	defer func() {
		err := recover()
		if err != nil {
			r.AppendError(errs.Errorf("panic when running: %+v", err))
		}

		if r.markStopping() {
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
						r.Parent().AppendError(errs.Errorf("panic when OnChildDone: %+v", err))
					}
				}()
				r.Parent().AppendError(r.Parent().OnChildDone(r))
			}()
		}
	}()
	go func() {
		select {
		case <-r.Stopping():
			return
		case <-r.Ctx().Done():
			r.Info("received cancel, stopping")
			stop(r)
		}
	}()

	r.Info("start")
	r.markStarted()
	select {
	case <-r.Stopping():
		r.Info("already stopping before start")
		return
	default:
	}
	r.AppendError(r.Start())
}

func StartBG(r Runner) {
	go Start(r)
	for _, c := range r.Children() {
		go Start(c)
	}
}

func Stop(r Runner) {
	if len(r.Children()) > 0 {
		for i := len(r.Children()) - 1; i >= 0; i-- {
			stop(r.Children()[i])
		}
	}
	stop(r)
}

func stop(r Runner) {
	select {
	case <-r.Started():
	default:
		// 这里不直接return的原因是：
		// 有些struct组合了Runner接口，但是并不需要Start，只是依赖Runner的一些公共方法
		// 例如Log相关方法、Value相关方法，Ctx和事件通知相关方法
		// 所以这里支持在Runner没有Start的情况下做Stop操作
		// 这种情况下必须要调用runner.Init执行初始化，Init、Start、Stop都可以不用实现.
		r.Info("stopping before started")
		r.markStopping()
		safeStop(r)
		r.Info("stop done before started")
		r.markStopDone()
		r.markDone()
		return
	}

	if !r.markStopping() {
		r.Info("already stopping")
		return
	}

	r.Info("stopping")
	safeStop(r)
	r.Info("stop done")
	r.markStopDone()
	<-r.Done()
}

func safeStop(r Runner) {
	defer func() {
		err := recover()
		if err != nil {
			r.AppendError(errs.Errorf("panic when stopping: %+v", err))
		}
	}()
	r.AppendError(r.Stop())
}

type baseRunner struct {
	kvs
	ctx          context.Context
	cancel       context.CancelFunc
	err          error
	parent       Runner
	started      chan struct{}
	childrenMap  map[string]Runner
	Logger       *zap.Logger
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

func NewBase(name string) Runner {
	br := &baseRunner{
		Logger:      zap.NewNop(),
		name:        name,
		ctx:         context.Background(),
		started:     make(chan struct{}),
		stopping:    make(chan struct{}),
		stopDone:    make(chan struct{}),
		done:        make(chan struct{}),
		childrenMap: make(map[string]Runner),
		kvs:         newSimpleInMemKvs(),
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
		br.Logger = zap.NewNop()
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
	if br.kvs == nil {
		br.kvs = newSimpleInMemKvs()
	}
	if br.Ctx() == nil {
		br.SetCtx(context.Background())
	}
	if br.cancel == nil {
		br.ctx, br.cancel = context.WithCancel(br.Ctx())
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

func (br *baseRunner) SetCtx(ctx context.Context) {
	br.ctx, br.cancel = context.WithCancel(ctx)
}

func (br *baseRunner) Cancel() {
	br.cancelOnce.Do(func() {
		br.cancel()
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

func (br *baseRunner) getLogger() *zap.Logger {
	return br.Logger
}

func (br *baseRunner) WithLoggerFrom(r Runner, kvs ...any) {
	br.Logger = r.getLogger().Named(br.Name()).With(log.HandleZapFields(kvs)...)
}

func (br *baseRunner) WithLoggerFields(kvs ...any) {
	br.Logger = br.Logger.With(log.HandleZapFields(kvs)...)
}

func (br *baseRunner) Debug(msg string, kvs ...any) {
	br.Logger.Debug(msg, log.HandleZapFields(kvs)...)
}

func (br *baseRunner) Info(msg string, kvs ...any) {
	br.Logger.Info(msg, log.HandleZapFields(kvs)...)
}

func (br *baseRunner) Warn(msg string, kvs ...any) {
	br.Logger.Warn(msg, log.HandleZapFields(kvs)...)
}

func (br *baseRunner) Error(msg string, err error, kvs ...any) {
	br.Logger.Error(msg, log.HandleZapFields(kvs, zap.Error(err))...)
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
