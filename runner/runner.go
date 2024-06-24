package runner

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/log"
	"github.com/donkeywon/golib/util"
	"go.uber.org/zap"
)

type Runner interface {
	Init() error
	Start() error
	Stop() error

	Name() string
	SetName(string)
	Ctx() context.Context
	SetCtx(context.Context)

	Logger() *zap.Logger
	WithLogger(l *zap.Logger, kvs ...any)
	WithLoggerNoName(l *zap.Logger, kvs ...any)

	Debug(msg string, kvs ...any)
	Info(msg string, kvs ...any)
	Warn(msg string, kvs ...any)
	Err(msg string, err error, kvs ...any)

	Store(k string, v any)
	DelKey(k string)
	HasKey(k string) bool
	BoolValue(k string) bool
	StringValue(k string) string
	StringValueOr(k string, d string) string
	IntValue(k string) int
	IntValueOr(k string, d int) int
	Int64Value(k string) int64
	Int64ValueOr(k string, d int64) int64
	FloatValue(k string) float64
	FloatValueOr(k string, d float64) float64
	ValueTo(k string, to any) error
	Collect() map[string]string
	CollectF(func(map[string]string))
	StoreValues(map[string]string)

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
	ChildrenError() error
	AppendRunner(Runner) bool
	WaitChildrenDone()
	SetParent(Runner)
	Parent() Runner
	OnChildDone(Runner) error

	AppendError(err ...error)
	Error() error
	SelfError() error
}

func Init(r Runner) error {
	var err error
	r.Info("init")
	err = util.V.Struct(r)
	if err != nil {
		return errs.Wrap(err, "validate fail")
	}
	if r.Ctx() == nil {
		r.SetCtx(context.Background())
	}
	err = r.Init()
	if err != nil {
		return err
	}
	r.Info("init done")
	for _, child := range r.Children() {
		child.WithLogger(r.Logger())
		child.SetCtx(r.Ctx())
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
		if r.Parent() != nil {
			r.Parent().AppendError(r.Parent().OnChildDone(r))
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
		// 例如Log相关方法、Value相关方法，Ctx和信号通知相关方法
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

	select {
	case <-r.Stopping():
		r.Info("already stopping")
		return
	default:
	}

	if r.markStopping() {
		r.Info("stopping")
		safeStop(r)
		r.Info("stop done")
		r.markStopDone()
		<-r.Done()
	}
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

type BaseRunner struct {
	ctx          context.Context
	err          error
	parent       Runner
	started      chan struct{}
	kvs          map[string]string
	childrenMap  map[string]Runner
	l            *zap.Logger
	done         chan struct{}
	stopDone     chan struct{}
	stopping     chan struct{}
	name         string
	children     []Runner
	startedOnce  sync.Once
	doneOnce     sync.Once
	stopDoneOnce sync.Once
	stoppingOnce sync.Once
	errMu        sync.Mutex
	kvsMu        sync.Mutex
}

func NewBase(name string) *BaseRunner {
	return &BaseRunner{
		l:           zap.NewNop(),
		name:        name,
		ctx:         context.Background(),
		started:     make(chan struct{}),
		stopping:    make(chan struct{}),
		stopDone:    make(chan struct{}),
		done:        make(chan struct{}),
		childrenMap: make(map[string]Runner, 0),
		kvs:         make(map[string]string, 0),
	}
}

func (br *BaseRunner) SetName(n string) {
	br.name = n
}

func (br *BaseRunner) Name() string {
	return br.name
}

func (br *BaseRunner) Init() error {
	if br.l == nil {
		br.l = zap.NewNop()
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
		br.childrenMap = make(map[string]Runner, 0)
	}
	if br.kvs == nil {
		br.kvs = make(map[string]string, 0)
	}
	return nil
}

func (br *BaseRunner) Start() error {
	<-br.Stopping()
	return nil
}

func (br *BaseRunner) Stop() error {
	return nil
}

func (br *BaseRunner) SetCtx(ctx context.Context) {
	br.ctx = ctx
}

func (br *BaseRunner) Ctx() context.Context {
	return br.ctx
}

func (br *BaseRunner) Store(k string, v any) {
	var vs string

	switch vv := v.(type) {
	case uint8:
		vs = strconv.FormatUint(uint64(vv), 10)
	case uint16:
		vs = strconv.FormatUint(uint64(vv), 10)
	case uint32:
		vs = strconv.FormatUint(uint64(vv), 10)
	case uint64:
		vs = strconv.FormatUint(vv, 10)
	case int8:
		vs = strconv.FormatInt(int64(vv), 10)
	case int16:
		vs = strconv.FormatInt(int64(vv), 10)
	case int32:
		vs = strconv.FormatInt(int64(vv), 10)
	case int64:
		vs = strconv.FormatInt(vv, 10)
	case float32:
		vs = strconv.FormatFloat(float64(vv), 'f', 10, 64)
	case uint:
		vs = strconv.FormatUint(uint64(vv), 10)
	case float64:
		vs = strconv.FormatFloat(float64(vv), 'f', 10, 64)
	case string:
		vs = vv
	case int:
		vs = strconv.FormatInt(int64(vv), 10)
	case bool:
		vs = strconv.FormatBool(vv)
	default:
		vs, _ = sonic.MarshalString(vv)
	}

	br.kvsMu.Lock()
	defer br.kvsMu.Unlock()
	br.kvs[k] = vs
}

func (br *BaseRunner) HasKey(k string) bool {
	br.kvsMu.Lock()
	defer br.kvsMu.Unlock()
	_, exists := br.kvs[k]
	return exists
}

func (br *BaseRunner) DelKey(k string) {
	br.kvsMu.Lock()
	defer br.kvsMu.Unlock()
	delete(br.kvs, k)
}

func (br *BaseRunner) StringValue(k string) string {
	br.kvsMu.Lock()
	defer br.kvsMu.Unlock()
	return br.kvs[k]
}

func (br *BaseRunner) StringValueOr(k string, d string) string {
	v := br.StringValue(k)
	if v == "" {
		return d
	}
	return v
}

func (br *BaseRunner) BoolValue(k string) bool {
	s := br.StringValue(k)
	return s == "true"
}

func (br *BaseRunner) IntValue(k string) int {
	return br.IntValueOr(k, 0)
}

func (br *BaseRunner) IntValueOr(k string, d int) int {
	i, err := strconv.Atoi(br.StringValueOr(k, strconv.FormatInt(int64(d), 10)))
	if err != nil {
		panic(err)
	}
	return i
}

func (br *BaseRunner) Int64Value(k string) int64 {
	return br.Int64ValueOr(k, int64(0))
}

func (br *BaseRunner) Int64ValueOr(k string, d int64) int64 {
	return int64(br.IntValueOr(k, int(d)))
}

func (br *BaseRunner) FloatValue(k string) float64 {
	return br.FloatValueOr(k, 0.0)
}

func (br *BaseRunner) FloatValueOr(k string, d float64) float64 {
	f, err := strconv.ParseFloat(br.StringValueOr(k, strconv.FormatFloat(d, 'f', 10, 64)), 64)
	if err != nil {
		panic(err)
	}
	return f
}

func (br *BaseRunner) ValueTo(k string, to any) error {
	v := br.StringValue(k)
	if v == "" {
		return nil
	}

	return sonic.Unmarshal(util.String2Bytes(v), to)
}

func (br *BaseRunner) Collect() map[string]string {
	c := make(map[string]string)
	br.CollectF(func(m map[string]string) {
		for k, v := range m {
			c[k] = v
		}
	})
	return c
}

func (br *BaseRunner) CollectF(f func(map[string]string)) {
	br.kvsMu.Lock()
	defer br.kvsMu.Unlock()
	f(br.kvs)
}

func (br *BaseRunner) StoreValues(m map[string]string) {
	if m == nil {
		return
	}
	br.kvsMu.Lock()
	defer br.kvsMu.Unlock()
	br.kvs = m
}

func (br *BaseRunner) Debug(msg string, kvs ...any) {
	br.l.Debug(msg, log.HandleZapFields(kvs)...)
}

func (br *BaseRunner) Info(msg string, kvs ...any) {
	br.l.Info(msg, log.HandleZapFields(kvs)...)
}

func (br *BaseRunner) Warn(msg string, kvs ...any) {
	br.l.Warn(msg, log.HandleZapFields(kvs)...)
}

func (br *BaseRunner) Err(msg string, err error, kvs ...any) {
	br.l.Error(msg, log.HandleZapFields(kvs, zap.Error(err))...)
}

func (br *BaseRunner) Logger() *zap.Logger {
	return br.l
}

func (br *BaseRunner) WithLogger(l *zap.Logger, kvs ...any) {
	br.l = l.Named(br.Name()).With(log.HandleZapFields(kvs)...)
}

func (br *BaseRunner) WithLoggerNoName(l *zap.Logger, kvs ...any) {
	br.l = l.With(log.HandleZapFields(kvs)...)
}

func (br *BaseRunner) Started() <-chan struct{} {
	return br.started
}

func (br *BaseRunner) Stopping() <-chan struct{} {
	return br.stopping
}

func (br *BaseRunner) StopDone() <-chan struct{} {
	return br.stopDone
}

func (br *BaseRunner) Done() <-chan struct{} {
	return br.done
}

func (br *BaseRunner) markStarted() bool {
	marked := false
	br.startedOnce.Do(func() {
		close(br.started)
		marked = true
	})
	return marked
}

func (br *BaseRunner) markStopping() bool {
	marked := false
	br.stoppingOnce.Do(func() {
		close(br.stopping)
		marked = true
	})
	return marked
}

func (br *BaseRunner) markStopDone() bool {
	marked := false
	br.stopDoneOnce.Do(func() {
		close(br.stopDone)
		marked = true
	})
	return marked
}

func (br *BaseRunner) markDone() bool {
	marked := false
	br.doneOnce.Do(func() {
		close(br.done)
		marked = true
	})
	return marked
}

func (br *BaseRunner) GetChild(name string) Runner {
	return br.childrenMap[name]
}

func (br *BaseRunner) Children() []Runner {
	return br.children
}

func (br *BaseRunner) WaitChildrenDone() {
	for _, child := range br.Children() {
		select {
		case <-child.Started():
			<-child.Done()
		default:
			continue
		}
	}
}

func (br *BaseRunner) SetParent(r Runner) {
	br.parent = r
}

func (br *BaseRunner) Parent() Runner {
	return br.parent
}

func (br *BaseRunner) OnChildDone(_ Runner) error {
	return nil
}

func (br *BaseRunner) AppendRunner(r Runner) bool {
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

func (br *BaseRunner) AppendError(err ...error) {
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

func (br *BaseRunner) Error() error {
	return errors.Join(br.SelfError(), br.ChildrenError())
}

func (br *BaseRunner) ChildrenError() error {
	var err error
	for _, child := range br.Children() {
		err = errors.Join(err, child.Error())
	}
	return err
}

func (br *BaseRunner) SelfError() error {
	br.errMu.Lock()
	defer br.errMu.Unlock()
	return br.err
}
