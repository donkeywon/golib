package pipeline

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/ratelimit"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/bytespool"
	"github.com/donkeywon/golib/util/conv"
	"github.com/donkeywon/golib/util/reflects"
	"github.com/zeebo/xxh3"
)

var (
	ErrStoppedManually  = errors.New("RW stopped manually")
	ErrChecksumNotMatch = errors.New("checksum not match")
)

const (
	RWRoleStarter RWRole = "starter"
	RWRoleReader  RWRole = "reader"
	RWRoleWriter  RWRole = "writer"

	defaultHashAlgo = "xxh3"
)

type RWCfg struct {
	Type      RWType       `json:"type"    validate:"required" yaml:"type"`
	Cfg       interface{}  `json:"cfg"       validate:"required" yaml:"cfg"`
	CommonCfg *RWCommonCfg `json:"commonCfg" yaml:"commonCfg"`
	Role      RWRole       `json:"role"      validate:"required" yaml:"role"`
}

type RWCommonCfg struct {
	RateLimiterCfg     *ratelimit.RateLimiterCfg `json:"rateLimiterCfg"     yaml:"rateLimiterCfg"`
	BufSize            int                       `json:"bufSize"            yaml:"bufSize"`
	Deadline           int                       `json:"deadline"           yaml:"deadline"`
	AsyncChanBufSize   int                       `json:"asyncChanBufSize"   yaml:"asyncChanBufSize"`
	EnableMonitorSpeed bool                      `json:"enableMonitorSpeed" yaml:"enableMonitorSpeed"`
	EnableCalcHash     bool                      `json:"enableCalcHash"     yaml:"enableCalcHash"`
	EnableRateLimit    bool                      `json:"enableRateLimit"    yaml:"enableRateLimit"`
	EnableAsync        bool                      `json:"enableAsync"        yaml:"enableAsync"`
	Checksum           string                    `json:"checksum"           yaml:"checksum"`
	HashAlgo           string                    `json:"hashAlgo"           yaml:"hashAlgo"`
}

type RWType string
type RWRole string

func Create(role RWRole, typ RWType, cfg interface{}, commonCfg *RWCommonCfg) (RW, error) {
	rwCfg := plugin.CreateCfg(typ)
	err := conv.ConvertOrMerge(rwCfg, cfg)
	if err != nil {
		return nil, errs.Wrapf(err, "invalid rw(%s) cfg", typ)
	}

	rw := plugin.CreateWithCfg(typ, rwCfg).(RW)
	if commonCfg == nil {
		commonCfg = &RWCommonCfg{}
	}
	rw.As(role)
	ApplyCommonCfgToRW(rw, commonCfg)
	return rw, nil
}

type onceError struct {
	err error
	sync.Mutex
}

func (a *onceError) Store(err error) bool {
	a.Lock()
	defer a.Unlock()
	if a.err != nil {
		return false
	}
	a.err = err
	return true
}

func (a *onceError) Load() error {
	a.Lock()
	defer a.Unlock()
	return a.err
}

type ReadHook func(n int, bs []byte, err error, cost int64, misc ...interface{}) error
type WriteHook func(n int, bs []byte, err error, cost int64, misc ...interface{}) error

type RW interface {
	runner.Runner
	io.ReadWriteCloser
	plugin.Plugin

	NestReader(io.ReadCloser) error
	NestWriter(io.WriteCloser) error
	Reader() io.ReadCloser
	Writer() io.WriteCloser

	Flush() error

	RegisterReadHook(...ReadHook)
	RegisterWriteHook(...WriteHook)

	Nwrite() uint64
	Nread() uint64

	Hash() string

	EnableMonitorSpeed()
	EnableCalcHash(hashAlgo string)
	EnableChecksum(checksum string, hashAlgo string)
	EnableRateLimit(ratelimit.RxTxRateLimiter)
	EnableWriteBuf(bufSize int, deadline int, async bool, asyncChanBufSize int)
	EnableReadBuf(bufSize int, async bool, asyncChanBufSize int)

	IsAsyncOrDeadline() bool
	AsyncChanLen() int
	AsyncChanCap() int

	AsStarter()
	IsStarter() bool
	AsReader()
	IsReader() bool
	AsWriter()
	IsWriter() bool
	As(RWRole)
	Is(RWRole) bool
	Role() RWRole
}

// BaseRW implement RW interface
// werr: async或deadline只要写失败就存werr，如果werr!=nil，之后的async写都skip掉
// rerr: async或buf的err
type BaseRW struct {
	rl ratelimit.RxTxRateLimiter
	runner.Runner
	w                  io.WriteCloser
	r                  io.ReadCloser
	buf                *bytespool.Bytes
	hashAlgo           string
	hash               hash.Hash
	closed             chan struct{}
	asyncDone          chan struct{}
	asyncChan          chan *bytespool.Bytes
	werr               onceError
	rerr               onceError
	checksum           string
	role               RWRole
	readHooks          []ReadHook
	writeHooks         []WriteHook
	offset             int
	wDeadline          int
	nw                 uint64
	bufSize            int
	nr                 uint64
	lastFlushTS        int64
	asyncChanBufSize   int
	closeOnce          sync.Once
	mu                 sync.Mutex
	enableRateLimit    bool
	enableMonitorSpeed bool
	enableCalcHash     bool
	async              bool
}

func NewBaseRW(name string) *BaseRW {
	return &BaseRW{
		Runner: runner.Create(name),
		closed: make(chan struct{}),
	}
}

func (b *BaseRW) Init() (err error) {
	defer func() {
		if err != nil {
			b.Warn("close after RW init fail", "err", err, "close_err", b.Close())
		}
	}()

	if !b.IsStarter() && b.Reader() != nil && b.Writer() != nil {
		return errs.New("RW cannot has nested reader and writer at the same time")
	}

	if b.Reader() == nil && b.Writer() == nil {
		return errs.New("RW has no nested reader and writer")
	}

	b.RegisterWriteHook(b.hookIncWritten, b.hookDebugLogWrite)
	b.RegisterReadHook(b.hookIncReadn, b.hookDebugLogRead)

	if b.checksum != "" {
		b.EnableCalcHash(b.hashAlgo)
		b.RegisterReadHook(b.hookChecksum)
	}

	if b.enableCalcHash {
		if b.hashAlgo == "" {
			b.hashAlgo = defaultHashAlgo
		}
		b.initHash()
		b.RegisterWriteHook(b.hookHash)
		b.RegisterReadHook(b.hookHash)
	}

	if b.enableMonitorSpeed {
		go b.monitorSpeed()
	}

	if b.enableRateLimit {
		b.rl.Inherit(b)
		err = runner.Init(b.rl)
		if err != nil {
			return errs.Wrap(err, "init rate limiter fail")
		}
		b.RegisterWriteHook(b.hookWriteRateLimit)
		b.RegisterReadHook(b.hookReadRateLimit)
	}

	if b.bufSize > 0 {
		if b.wDeadline > 0 && b.IsWriter() {
			go b.deadlineFlush()
		}
	}

	if b.async {
		if b.asyncChanBufSize < 0 {
			b.asyncChanBufSize = 0
		}
		b.asyncChan = make(chan *bytespool.Bytes, b.asyncChanBufSize)
		b.asyncDone = make(chan struct{})
		if b.IsReader() {
			go b.asyncRead()
		} else if b.IsWriter() {
			go b.asyncWrite()
		}
	}

	if !b.IsReader() && !b.IsWriter() && !b.IsStarter() {
		return errs.New("RW must be Reader|Writer|Starter")
	}

	if b.IsReader() && b.IsWriter() || b.IsWriter() && b.IsStarter() || b.IsStarter() && b.IsReader() {
		return errs.New("RW can only be one of the roles: Reader|Writer|Starter")
	}

	if b.IsStarter() {
		b.RegisterReadHook(b.hookManuallyStop)
		b.RegisterWriteHook(b.hookManuallyStop)
	}

	return b.Runner.Init()
}

// Start 如果当前RW可以作为Starter，那么需要实现Start方法.
func (b *BaseRW) Start() error {
	// for example

	// read from b and write to b.Writer()
	// such as io.Copy(b.Writer(), b)

	// 执行到这里，说明要么读到了io.EOF，要么被Stop
	// 如果被Stop的话，b.read()方法会返回一个ErrStoppedManually
	// if errors.Is(err, ErrStoppedManually) {
	// 	   err = nil
	// }

	// closeErr = b.Close() // 也可以放到defer里
	// if closeErr != nil {
	// 	   err = errors.Join(err, closeErr)
	// }
	// return err

	return errs.New("non-runnable RW type")
}

// Stop 通知RW停止
// 不在Stop方法中做CloseReader和CloseWriter的操作的原因是想做到优雅关闭
// 由Starter检测到stop信号后停止read，然后再进行Close，参考Start方法中的example
//
// 存在一种特殊情况，例如
// b.Reader()是TailRW或者其他类似的ReadCloser，当执行Read方法的时候，会存在阻塞的情况
// 如果不执行Close方法且一直没有新内容的话，那么Read会一直阻塞住
// 所以对b.Reader()和b.Writer()也执行一次runner.Stop通知，由TailRW自己在Stop方法中做Close操作.
func (b *BaseRW) Stop() error {
	if b.Reader() != nil {
		if r, ok := b.Reader().(RW); ok {
			runner.Stop(r)
		}
	}
	if b.Writer() != nil {
		if w, ok := b.Writer().(RW); ok {
			runner.Stop(w)
		}
	}
	return b.Runner.Stop()
}

func (b *BaseRW) Type() interface{} {
	panic("method RW.Type not implemented")
}

func (b *BaseRW) GetCfg() interface{} {
	panic("method RW.GetCfg not implemented")
}

func (b *BaseRW) NestReader(r io.ReadCloser) error {
	if rw, ok := r.(RW); ok && b.Reader() != nil {
		err := rw.NestReader(b.Reader())
		if err != nil {
			return errs.Wrapf(err, "rw(%s) nest reader fail", rw.Type())
		}
		b.r = rw
	} else {
		b.r = r
	}
	return nil
}

func (b *BaseRW) NestWriter(w io.WriteCloser) error {
	if rw, ok := w.(RW); ok && b.Writer() != nil {
		err := rw.NestWriter(b.Writer())
		if err != nil {
			return errs.Wrapf(err, "rw(%s) nest writer fail", rw.Type())
		}
		b.w = rw
	} else {
		b.w = w
	}
	return nil
}

func (b *BaseRW) Reader() io.ReadCloser {
	return b.r
}

func (b *BaseRW) Writer() io.WriteCloser {
	return b.w
}

func (b *BaseRW) AsStarter() {
	b.role = RWRoleStarter
}

func (b *BaseRW) IsStarter() bool {
	return b.role == RWRoleStarter
}

func (b *BaseRW) AsReader() {
	b.role = RWRoleReader
}

func (b *BaseRW) IsReader() bool {
	return b.role == RWRoleReader
}

func (b *BaseRW) AsWriter() {
	b.role = RWRoleWriter
}

func (b *BaseRW) IsWriter() bool {
	return b.role == RWRoleWriter
}

func (b *BaseRW) As(role RWRole) {
	b.role = role
}

func (b *BaseRW) Is(role RWRole) bool {
	return b.role == role
}

func (b *BaseRW) Role() RWRole {
	return b.role
}

func (b *BaseRW) RegisterReadHook(rh ...ReadHook) {
	for _, h := range rh {
		if h == nil {
			panic("read hook is nil")
		}
		b.readHooks = append(b.readHooks, h)
	}
}

func (b *BaseRW) RegisterWriteHook(wh ...WriteHook) {
	for _, h := range wh {
		if h == nil {
			panic("write hook is nil")
		}
		b.writeHooks = append(b.writeHooks, h)
	}
}

func (b *BaseRW) Read(p []byte) (int, error) {
	if b.Reader() == nil {
		return 0, errs.New("RW is not reader")
	}

	if len(p) == 0 {
		return 0, nil
	}

	var (
		nr  int
		err error
	)
	if b.bufSize > 0 {
		nr, err = b.readBuf(p)
	} else {
		nr, err = b.read(p)
	}
	return nr, err
}

func (b *BaseRW) Write(p []byte) (int, error) {
	if b.Writer() == nil {
		return 0, errs.New("RW is not writer")
	}

	if len(p) == 0 {
		return 0, nil
	}

	var (
		nw  int
		err error
	)

	if b.bufSize > 0 || b.async {
		nw, err = b.writeBuf(p)
	} else {
		nw, err = b.write(p)
	}
	return nw, err
}

func (b *BaseRW) Close() error {
	return errors.Join(b.closeReader(), b.closeWriter())
}

func (b *BaseRW) IsAsyncOrDeadline() bool {
	return b.async || b.bufSize > 0 && b.wDeadline > 0
}

func (b *BaseRW) EnableMonitorSpeed() {
	b.enableMonitorSpeed = true
}

func (b *BaseRW) EnableCalcHash(algo string) {
	b.enableCalcHash = true
	b.hashAlgo = algo
}

func (b *BaseRW) EnableRateLimit(rl ratelimit.RxTxRateLimiter) {
	b.rl = rl
	b.enableRateLimit = true
}

func (b *BaseRW) EnableWriteBuf(bufSize int, deadline int, async bool, asyncChanBufSize int) {
	b.bufSize = bufSize
	b.async = async
	if async {
		b.asyncChanBufSize = asyncChanBufSize
	}
	b.wDeadline = deadline
}

func (b *BaseRW) EnableReadBuf(bufSize int, async bool, asyncChanBufSize int) {
	b.bufSize = bufSize
	b.async = async
	if async {
		b.asyncChanBufSize = asyncChanBufSize
	}
}

func (b *BaseRW) EnableChecksum(checksum string, algo string) {
	b.checksum = checksum
	b.hashAlgo = algo
}

func (b *BaseRW) Nwrite() uint64 {
	return atomic.LoadUint64(&b.nw)
}

func (b *BaseRW) Nread() uint64 {
	return atomic.LoadUint64(&b.nr)
}

func (b *BaseRW) AsyncChanLen() int {
	return len(b.asyncChan)
}

func (b *BaseRW) AsyncChanCap() int {
	return cap(b.asyncChan)
}

func (b *BaseRW) Hash() string {
	if !b.enableCalcHash || b.hash == nil {
		return ""
	}
	var bs []byte
	switch b.hashAlgo {
	case "xxh3":
		bbs := b.hash.(*xxh3.Hasher).Sum128().Bytes()
		bs = bbs[:]
	default:
		bs = b.hash.Sum(nil)
	}
	return hex.EncodeToString(bs)
}

func (b *BaseRW) Flush() error {
	if b.bufSize <= 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	return b.flushNoLock()
}

func (b *BaseRW) initHash() {
	switch b.hashAlgo {
	case "sha1":
		b.hash = sha1.New()
	case "md5":
		b.hash = md5.New()
	case "sha256":
		b.hash = sha256.New()
	case "crc32":
		b.hash = crc32.New(crc32.IEEETable)
	case "xxh3":
		b.hash = xxh3.New()
	}
}

func (b *BaseRW) hookManuallyStop(_ int, _ []byte, _ error, _ int64, _ ...interface{}) error {
	select {
	case <-b.Stopping():
		return ErrStoppedManually
	default:
		return nil
	}
}

func (b *BaseRW) getBuf() *bytespool.Bytes {
	return bytespool.GetBytesN(b.bufSize)
}

func (b *BaseRW) getBufN(n int) *bytespool.Bytes {
	return bytespool.GetBytesN(n)
}

func (b *BaseRW) read(p []byte) (nr int, err error) {
	startTS := time.Now().UnixMilli()
	defer func() {
		endTS := time.Now().UnixMilli()
		for i := len(b.readHooks) - 1; i >= 0; i-- {
			h := b.readHooks[i]
			hookErr := h(nr, p, err, endTS-startTS)
			if hookErr != nil {
				err = errors.Join(err, errs.Wrapf(hookErr, "read hook(%d) %s fail", i, reflects.GetFuncName(h)))
			}
		}
	}()

	return b.Reader().Read(p)
}

func (b *BaseRW) write(p []byte) (nw int, err error) {
	startTS := time.Now().UnixMilli()
	defer func() {
		endTS := time.Now().UnixMilli()
		for i := len(b.writeHooks) - 1; i >= 0; i-- {
			h := b.writeHooks[i]
			hookErr := h(nw, p, err, endTS-startTS)
			if hookErr != nil {
				err = errors.Join(err, errs.Wrapf(hookErr, "write hook(%d) %s fail", i, reflects.GetFuncName(h)))
			}
		}
	}()

	return b.Writer().Write(p)
}

func (b *BaseRW) readBuf(p []byte) (int, error) {
	b.Debug("start read buf", "len", len(p))

	b.mu.Lock()
	defer b.mu.Unlock()

	var (
		nr  int
		err error
	)
	for nr < len(p) && err == nil {
		err = b.prepareReadBuf()
		if err == nil {
			nc := copy(p[nr:], b.buf.B()[b.offset:])
			nr += nc
			b.offset += nc
			b.Debug("read buf", "nc", nc, "nr", nr, "offset", b.offset)
		}
	}

	b.Debug("read buf done", "nr", nr, "err", err)

	return nr, err
}

func (b *BaseRW) prepareReadBuf() error {
	if b.buf != nil && b.offset < b.buf.Len() {
		// 当前buf还有剩余没读
		b.Debug("cur buf remains unread bytes", "buf_len", b.buf.Len(), "offset", b.offset)
		return nil
	}

	if b.async {
		buf := <-b.asyncChan
		if buf != nil {
			// 还有剩余的buf在asyncChan里没读
			if b.buf != nil {
				b.buf.Free()
			}
			b.offset = 0
			b.buf = buf
			b.Debug("receive buf from async chan", "buf_len", b.buf.Len())
			return nil
		}

		return b.rerr.Load()
	}

	err := b.rerr.Load()
	if err != nil {
		return err
	}

	if b.buf == nil {
		b.buf = b.getBuf()
	}

	b.offset = 0
	nr, err := b.read(b.buf.B())
	b.Debug("receive buf from reader", "nr", nr, "buf_len", b.buf.Len(), "err", err)
	b.buf.Shrink(nr)
	if err != nil {
		if err != io.EOF {
			err = errs.Wrap(err, "read to buf fail")
		}
		b.rerr.Store(err)
	}
	if nr > 0 {
		return nil
	}
	return err
}

func (b *BaseRW) writeBuf(p []byte) (int, error) {
	err := b.werr.Load()
	if err != nil {
		return 0, err
	}

	b.Debug("start write buf", "len", len(p))

	b.mu.Lock()
	defer b.mu.Unlock()

	var nw int

	for len(p) > 0 {
		if b.buf == nil {
			if b.bufSize <= 0 {
				b.buf = b.getBufN(len(p))
			} else {
				b.buf = b.getBuf()
			}
		}

		nc := copy(b.buf.B()[b.offset:], p)
		p = p[nc:]
		b.offset += nc
		nw += nc

		b.Debug("write buf", "nc", nc, "nw", nw, "buf_len", b.buf.Len(), "offset", b.offset)
		if b.offset < b.bufSize {
			continue
		}

		b.Debug("buf full, flush")
		err = b.flushNoLock()
		if err != nil {
			b.Debug("write buf returned caused by flush fail")
			return 0, err
		}
	}

	b.Debug("write buf done", "nw", nw)
	return nw, err
}

func (b *BaseRW) flushNoLock() error {
	if b.buf == nil {
		b.Debug("flush skipped caused by buf is nil")
		return nil
	}
	if b.offset == 0 {
		b.Debug("flush skipped caused by buf empty", "buf_len", b.buf.Len(), "offset", b.offset)
		return nil
	}

	var err error

	b.lastFlushTS = time.Now().Unix()
	b.Debug("flush", "len", b.buf.Len(), "offset", b.offset)
	if b.async {
		b.buf.Shrink(b.offset)
		b.asyncChan <- b.buf
		b.Debug("send to write async channel", "chan_len", len(b.asyncChan), "chan_cap", cap(b.asyncChan))
		b.buf = nil
	} else {
		_, err = b.write(b.buf.B()[:b.offset])
	}

	b.offset = 0
	if err != nil {
		return errs.Wrap(err, "flush write fail")
	}
	return nil
}

func (b *BaseRW) deadlineFlush() {
	t := time.NewTicker(time.Second * time.Duration(b.wDeadline))
	defer t.Stop()
	for {
		select {
		case <-b.closed:
			b.Debug("deadline flush routine finished")
			return
		case <-t.C:
			if b.werr.Load() != nil {
				b.Info("deadline flush skipped caused by write err")
				continue
			}

			b.mu.Lock()
			if time.Now().Unix()-b.lastFlushTS < int64(b.wDeadline) {
				b.mu.Unlock()
				continue
			}
			b.Debug("deadline flush")
			err := b.flushNoLock()
			if err != nil {
				err = errs.Wrap(err, "deadline flush fail")

				ok := b.werr.Store(err)
				if !ok {
					b.Warn("deadline flush err store fail", "err", err)
				}
			}
			b.mu.Unlock()
		}
	}
}

func (b *BaseRW) asyncRead() {
	b.Debug("start async read")
	for {
		select {
		case <-b.closed:
			b.Info("async read finished caused by closed")
			b.rerr.Store(io.EOF)
			close(b.asyncChan)
			return
		default:
		}

		bs := bytespool.GetBytesN(b.bufSize)
		nr, err := b.read(bs.B())
		b.Debug("async read", "nr", nr, "err", err)
		bs.Shrink(nr)
		b.Debug("send to read async channel", "chan_len", len(b.asyncChan), "chan_cap", cap(b.asyncChan), "bs_len", bs.Len())
		b.asyncChan <- bs
		if errors.Is(err, io.EOF) {
			b.rerr.Store(err)
			close(b.asyncChan)
			b.Info("async read finished caused by EOF")
			return
		}
		if err != nil {
			b.rerr.Store(errs.Wrap(err, "async read fail"))
			close(b.asyncChan)
			b.Error("async read finished caused by error occurred", err)
			return
		}
	}
}

func (b *BaseRW) asyncWrite() {
	b.Debug("start async write")
	for {
		bs, ok := <-b.asyncChan
		if bs == nil && !ok {
			b.Info("async write finished caused by closed")
			close(b.asyncDone)
			return
		}

		b.Debug("async write receive from chan", "chan_len", len(b.asyncChan), "chan_cap", cap(b.asyncChan))

		if b.werr.Load() != nil {
			b.Info("async write skipped caused by write err")
			bs.Free()
			continue
		}

		_, err := b.write(bs.B())
		b.Debug("async write", "len", bs.Len(), "err", err)
		bs.Free()
		if err == nil {
			continue
		}

		// 这里不能直接break或return，因为要消费asyncChan里剩余的buf并进行Free，否则会内存泄漏，消费完后才会return或break
		err = errs.Wrap(err, "async write fail")
		ok = b.werr.Store(err)
		if !ok {
			b.Warn("async write fail err store fail", "err", err)
		}
	}
}

func (b *BaseRW) closeReader() error {
	if b.Reader() == nil {
		return nil
	}

	b.Info("close nested reader")

	err := b.Reader().Close()
	b.closeOnce.Do(func() {
		close(b.closed)

		if b.bufSize > 0 && b.buf != nil {
			b.mu.Lock()
			b.buf.Free()
			b.mu.Unlock()
		}

		if b.async {
			for bs := range b.asyncChan {
				if bs == nil {
					break
				}
				b.Info("consume remaining buf in async channel", "len", bs.Len())
				bs.Free()
			}
		}
	})

	if err == nil {
		return nil
	}
	return errs.Wrapf(err, "%s close nested reader fail", b.Name())
}

func (b *BaseRW) closeWriter() error {
	if b.Writer() == nil {
		return nil
	}

	b.Info("close nested writer")

	var err error
	b.closeOnce.Do(func() {
		close(b.closed)

		if b.bufSize > 0 && b.buf != nil {
			b.mu.Lock()
			err = b.flushNoLock()
			if err != nil {
				err = errs.Wrap(err, "close flush fail")
			}
			if b.buf != nil {
				b.buf.Free()
				b.buf = nil
			}
			b.mu.Unlock()
		}
		if b.async {
			close(b.asyncChan)
			// 等async线程写完
			<-b.asyncDone
		}
	})
	closeErr := b.Writer().Close()
	if closeErr == nil {
		return err
	}
	return errors.Join(err, errs.Wrapf(closeErr, "%s close nested writer fail", b.Name()))
}

func (b *BaseRW) hookIncWritten(n int, _ []byte, _ error, _ int64, _ ...interface{}) error {
	atomic.AddUint64(&b.nw, uint64(n))
	return nil
}

func (b *BaseRW) hookIncReadn(n int, _ []byte, _ error, _ int64, _ ...interface{}) error {
	atomic.AddUint64(&b.nr, uint64(n))
	return nil
}

func (b *BaseRW) hookHash(n int, bs []byte, _ error, _ int64, _ ...interface{}) error {
	_, _ = b.hash.Write(bs[:n])
	return nil
}

func (b *BaseRW) hookDebugLogRead(n int, _ []byte, err error, cost int64, misc ...interface{}) error {
	b.Debug("read hook log", "n", n, "err", err, "cost", fmt.Sprintf("%d ms", cost), "misc", misc)
	return nil
}

func (b *BaseRW) hookDebugLogWrite(n int, _ []byte, err error, cost int64, misc ...interface{}) error {
	b.Debug("write hook log", "n", n, "err", err, "cost", fmt.Sprintf("%d ms", cost), "misc", misc)
	return nil
}

func (b *BaseRW) hookReadRateLimit(n int, _ []byte, _ error, _ int64, _ ...interface{}) error {
	if n == 0 || b.rl == nil {
		return nil
	}
	e := b.rl.RxWaitN(n, 0)
	if e != nil {
		b.Warn("read rate limit fail", "n", n, "err", e.Error())
	}
	return nil
}

func (b *BaseRW) hookWriteRateLimit(n int, _ []byte, _ error, _ int64, _ ...interface{}) error {
	if n == 0 || b.rl == nil {
		return nil
	}
	e := b.rl.TxWaitN(n, 0)
	if e != nil {
		b.Warn("write rate limit fail", "n", n, "err", e.Error())
	}
	return nil
}

func (b *BaseRW) hookChecksum(_ int, _ []byte, err error, _ int64, _ ...interface{}) error {
	if !errors.Is(err, io.EOF) {
		return nil
	}

	checksum := b.Hash()
	if b.checksum != checksum {
		return errs.Wrapf(err, "checksum expected: %s, actual: %s", b.checksum, checksum)
	}

	return nil
}

func (b *BaseRW) logSpeed(sub uint64, interval int64) {
	switch {
	case sub < 1024:
		b.Info("monitor", "speed", fmt.Sprintf("%.1fB/s", float64(sub)/float64(interval)))
	case sub >= 1024 && sub < 1024*1024:
		b.Info("monitor", "speed", fmt.Sprintf("%.3fKB/s", float64(sub)/1024/float64(interval)))
	default:
		b.Info("monitor", "speed", fmt.Sprintf("%.3fMB/s", float64(sub)/1024/1024/float64(interval)))
	}
}

func (b *BaseRW) monitorSpeed() {
	var last, now uint64
	var interval int64 = 5
	t := time.NewTicker(time.Second * time.Duration(interval))
	defer t.Stop()
	for {
		select {
		case <-b.closed:
			return
		case <-t.C:
		}

		if b.IsWriter() {
			now = b.Nwrite()
		} else {
			now = b.Nread()
		}

		if now == last {
			continue
		}
		sub := now - last
		b.logSpeed(sub, interval)
		last = now
	}
}

func ApplyCommonCfgToRW(rw RW, cfg *RWCommonCfg) {
	if cfg == nil {
		return
	}
	if cfg.EnableMonitorSpeed {
		rw.EnableMonitorSpeed()
	}
	if cfg.EnableCalcHash {
		rw.EnableCalcHash(cfg.HashAlgo)
	}
	if cfg.EnableRateLimit {
		rw.EnableRateLimit(plugin.CreateWithCfg(cfg.RateLimiterCfg.Type, cfg.RateLimiterCfg.Cfg).(ratelimit.RxTxRateLimiter))
	}
	if cfg.Checksum != "" {
		rw.EnableChecksum(cfg.Checksum, cfg.HashAlgo)
	}
	if cfg.BufSize > 0 {
		switch rw.Role() {
		case RWRoleReader:
			rw.EnableReadBuf(cfg.BufSize, cfg.EnableAsync, cfg.AsyncChanBufSize)
		case RWRoleWriter:
			rw.EnableWriteBuf(cfg.BufSize, cfg.Deadline, cfg.EnableAsync, cfg.AsyncChanBufSize)
		case RWRoleStarter:
			if rw.Writer() != nil {
				rw.EnableWriteBuf(cfg.BufSize, cfg.Deadline, cfg.EnableAsync, cfg.AsyncChanBufSize)
			} else {
				rw.EnableReadBuf(cfg.BufSize, cfg.EnableAsync, cfg.AsyncChanBufSize)
			}
		}
	}
}
