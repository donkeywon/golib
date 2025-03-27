package rw

import (
	"context"
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
	"github.com/donkeywon/golib/util/jsons"
	"github.com/donkeywon/golib/util/reflects"
	"github.com/donkeywon/golib/util/yamls"
	"github.com/tidwall/gjson"
	"github.com/zeebo/xxh3"
)

var (
	ErrChecksumNotMatch = errors.New("checksum not match")

	CreateBase = newBase
)

const (
	RoleStarter Role = "starter"
	RoleReader  Role = "reader"
	RoleWriter  Role = "writer"

	defaultHashAlgo = "xxh3"
)

type Cfg struct {
	Type     Type      `json:"type"      validate:"required" yaml:"type"`
	Cfg      any       `json:"cfg"       validate:"required" yaml:"cfg"`
	ExtraCfg *ExtraCfg `json:"extraCfg"  yaml:"extraCfg"`
	Role     Role      `json:"role"      validate:"required" yaml:"role"`
}

type cfgWithoutType struct {
	Cfg      any       `json:"cfg"      yaml:"cfg"`
	ExtraCfg *ExtraCfg `json:"extraCfg" yaml:"extraCfg"`
	Role     Role      `json:"role"     yaml:"role"`
}

func (c *Cfg) UnmarshalJSON(data []byte) error {
	return c.customUnmarshal(data, jsons.Unmarshal)
}

func (c *Cfg) UnmarshalYAML(data []byte) error {
	return c.customUnmarshal(data, yamls.Unmarshal)
}

func (c *Cfg) customUnmarshal(data []byte, unmarshaler func([]byte, any) error) error {
	typ := gjson.GetBytes(data, "type")
	if !typ.Exists() {
		return errs.Errorf("rw type is not present")
	}
	if typ.Type != gjson.String {
		return errs.Errorf("invalid rw type")
	}
	c.Type = Type(typ.Str)

	cv := cfgWithoutType{}
	cv.Cfg = plugin.CreateCfg(c.Type)
	if cv.Cfg == nil {
		return errs.Errorf("created rw cfg is nil: %s", c.Type)
	}
	err := unmarshaler(data, &cv)
	if err != nil {
		return err
	}
	c.Cfg = cv.Cfg
	c.ExtraCfg = cv.ExtraCfg
	c.Role = cv.Role
	return nil
}

type ExtraCfg struct {
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

type Type string
type Role string

func CreateRW(rwCfg *Cfg) RW {
	rw := plugin.CreateWithCfg(rwCfg.Type, rwCfg.Cfg).(RW)
	if rwCfg.ExtraCfg == nil {
		rwCfg.ExtraCfg = &ExtraCfg{}
	}
	rw.As(rwCfg.Role)
	ApplyCommonCfgToRW(rw, rwCfg.ExtraCfg)
	return rw
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

type ReadHook func(n int, bs []byte, err error, cost int64, misc ...any) error
type WriteHook func(n int, bs []byte, err error, cost int64, misc ...any) error

type RW interface {
	runner.Runner
	io.ReadWriteCloser

	NestReader(io.ReadCloser)
	NestWriter(io.WriteCloser)
	Reader() io.ReadCloser
	Writer() io.WriteCloser

	Flush() error

	HookRead(...ReadHook)
	HookWrite(...WriteHook)

	Nwrite() uint64
	Nread() uint64

	Hash() string

	// TODO EnableProgressLog
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
	As(Role)
	Is(Role) bool
	Role() Role
}

// baseRW implement RW interface
// werr: async或deadline只要写失败就存werr，如果werr!=nil，之后的async写都skip掉
// rerr: async或buf的err
type baseRW struct {
	rl ratelimit.RxTxRateLimiter
	runner.Runner
	cancel             context.CancelFunc
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
	role               Role
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

func newBase(name string) RW {
	return &baseRW{
		Runner: runner.Create(name),
		closed: make(chan struct{}),
	}
}

func (b *baseRW) Init() (err error) {
	if !b.IsStarter() && b.Reader() != nil && b.Writer() != nil {
		return errs.New("RW cannot has nested reader and writer at the same time")
	}

	if b.Reader() == nil && b.Writer() == nil {
		return errs.New("RW has no nested reader and writer")
	}

	b.HookWrite(b.hookIncWritten, b.hookDebugLogWrite, b.hookCancel)
	b.HookRead(b.hookIncReadn, b.hookDebugLogRead, b.hookCancel)

	if b.checksum != "" {
		b.EnableCalcHash(b.hashAlgo)
		b.HookRead(b.hookChecksum)
	}

	if b.enableCalcHash {
		if b.hashAlgo == "" {
			b.hashAlgo = defaultHashAlgo
		}
		b.initHash()
		b.HookWrite(b.hookHash)
		b.HookRead(b.hookHash)
	}

	if b.enableMonitorSpeed {
		go b.monitorSpeed()
	}

	if b.enableRateLimit {
		b.rl.Inherit(b)
		err = runner.Init(b.rl)
		if err != nil {
			return errs.Wrap(err, "init rate limiter failed")
		}
		b.HookWrite(b.hookWriteRateLimit)
		b.HookRead(b.hookReadRateLimit)
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

	if b.Ctx() == nil {
		b.SetCtx(context.Background())
	}
	ctx, cancel := context.WithCancel(b.Ctx())
	b.cancel = cancel
	b.SetCtx(ctx)

	return b.Runner.Init()
}

// Start 如果当前RW可以作为Starter，那么需要实现Start方法.
func (b *baseRW) Start() error {
	return errs.New("non-runnable RW type")
}

func (b *baseRW) Stop() error {
	return b.Runner.Stop()
}

func (b *baseRW) Type() any {
	panic("method RW.Kind not implemented")
}

func (b *baseRW) GetCfg() any {
	panic("method RW.GetCfg not implemented")
}

func (b *baseRW) NestReader(r io.ReadCloser) {
	if rw, ok := r.(RW); ok && b.Reader() != nil {
		rw.NestReader(b.Reader())
		b.r = rw
	} else {
		b.r = r
	}
}

func (b *baseRW) NestWriter(w io.WriteCloser) {
	if rw, ok := w.(RW); ok && b.Writer() != nil {
		rw.NestWriter(b.Writer())
		b.w = rw
	} else {
		b.w = w
	}
}

func (b *baseRW) Reader() io.ReadCloser {
	return b.r
}

func (b *baseRW) Writer() io.WriteCloser {
	return b.w
}

func (b *baseRW) AsStarter() {
	b.As(RoleStarter)
}

func (b *baseRW) IsStarter() bool {
	return b.Is(RoleStarter)
}

func (b *baseRW) AsReader() {
	b.As(RoleReader)
}

func (b *baseRW) IsReader() bool {
	return b.Is(RoleReader)
}

func (b *baseRW) AsWriter() {
	b.As(RoleWriter)
}

func (b *baseRW) IsWriter() bool {
	return b.Is(RoleWriter)
}

func (b *baseRW) As(role Role) {
	b.role = role
}

func (b *baseRW) Is(role Role) bool {
	return b.role == role
}

func (b *baseRW) Role() Role {
	return b.role
}

func (b *baseRW) HookRead(rh ...ReadHook) {
	for _, h := range rh {
		if h == nil {
			panic("read hook is nil")
		}
		b.readHooks = append(b.readHooks, h)
	}
}

func (b *baseRW) HookWrite(wh ...WriteHook) {
	for _, h := range wh {
		if h == nil {
			panic("write hook is nil")
		}
		b.writeHooks = append(b.writeHooks, h)
	}
}

func (b *baseRW) Read(p []byte) (int, error) {
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

func (b *baseRW) Write(p []byte) (int, error) {
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

func (b *baseRW) Cancel() {
	if b.cancel != nil {
		b.cancel()
	}
}

func (b *baseRW) Close() error {
	defer b.Cancel()
	return errors.Join(b.closeReader(), b.closeWriter())
}

func (b *baseRW) IsAsyncOrDeadline() bool {
	return b.async || b.bufSize > 0 && b.wDeadline > 0
}

func (b *baseRW) EnableMonitorSpeed() {
	b.enableMonitorSpeed = true
}

func (b *baseRW) EnableCalcHash(algo string) {
	b.enableCalcHash = true
	b.hashAlgo = algo
}

func (b *baseRW) EnableRateLimit(rl ratelimit.RxTxRateLimiter) {
	b.rl = rl
	b.enableRateLimit = true
}

func (b *baseRW) EnableWriteBuf(bufSize int, deadline int, async bool, asyncChanBufSize int) {
	b.bufSize = bufSize
	b.async = async
	if async {
		b.asyncChanBufSize = asyncChanBufSize
	}
	b.wDeadline = deadline
}

func (b *baseRW) EnableReadBuf(bufSize int, async bool, asyncChanBufSize int) {
	b.bufSize = bufSize
	b.async = async
	if async {
		b.asyncChanBufSize = asyncChanBufSize
	}
}

func (b *baseRW) EnableChecksum(checksum string, algo string) {
	b.checksum = checksum
	b.hashAlgo = algo
}

func (b *baseRW) Nwrite() uint64 {
	return atomic.LoadUint64(&b.nw)
}

func (b *baseRW) Nread() uint64 {
	return atomic.LoadUint64(&b.nr)
}

func (b *baseRW) AsyncChanLen() int {
	return len(b.asyncChan)
}

func (b *baseRW) AsyncChanCap() int {
	return cap(b.asyncChan)
}

func (b *baseRW) Hash() string {
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

func (b *baseRW) Flush() error {
	if b.bufSize <= 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	return b.flushNoLock()
}

func (b *baseRW) initHash() {
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

func (b *baseRW) getBuf() *bytespool.Bytes {
	return bytespool.GetN(b.bufSize)
}

func (b *baseRW) getBufN(n int) *bytespool.Bytes {
	return bytespool.GetN(n)
}

func (b *baseRW) read(p []byte) (nr int, err error) {
	startTS := time.Now().UnixMilli()
	defer func() {
		endTS := time.Now().UnixMilli()
		for i := len(b.readHooks) - 1; i >= 0; i-- {
			h := b.readHooks[i]
			hookErr := h(nr, p, err, endTS-startTS)
			if hookErr != nil {
				err = errors.Join(err, errs.Wrapf(hookErr, "read hook(%d) %s failed", i, reflects.GetFuncName(h)))
			}
		}
	}()

	return b.Reader().Read(p)
}

func (b *baseRW) write(p []byte) (nw int, err error) {
	startTS := time.Now().UnixMilli()
	defer func() {
		endTS := time.Now().UnixMilli()
		for i := len(b.writeHooks) - 1; i >= 0; i-- {
			h := b.writeHooks[i]
			hookErr := h(nw, p, err, endTS-startTS)
			if hookErr != nil {
				err = errors.Join(err, errs.Wrapf(hookErr, "write hook(%d) %s failed", i, reflects.GetFuncName(h)))
			}
		}
	}()

	return b.Writer().Write(p)
}

func (b *baseRW) readBuf(p []byte) (int, error) {
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

func (b *baseRW) prepareReadBuf() error {
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
			err = errs.Wrap(err, "read to buf failed")
		}
		b.rerr.Store(err)
	}
	if nr > 0 {
		return nil
	}
	return err
}

func (b *baseRW) writeBuf(p []byte) (int, error) {
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
			b.Debug("write buf returned caused by flush failed")
			return 0, err
		}
	}

	b.Debug("write buf done", "nw", nw)
	return nw, err
}

func (b *baseRW) flushNoLock() error {
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
		return errs.Wrap(err, "flush write failed")
	}
	return nil
}

func (b *baseRW) deadlineFlush() {
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
				err = errs.Wrap(err, "deadline flush failed")

				ok := b.werr.Store(err)
				if !ok {
					b.Warn("deadline flush err store failed", "err", err)
				}
			}
			b.mu.Unlock()
		}
	}
}

func (b *baseRW) asyncRead() {
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

		bs := bytespool.GetN(b.bufSize)
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
			b.rerr.Store(errs.Wrap(err, "async read failed"))
			close(b.asyncChan)
			b.Error("async read finished caused by error occurred", err)
			return
		}
	}
}

func (b *baseRW) asyncWrite() {
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
		err = errs.Wrap(err, "async write failed")
		ok = b.werr.Store(err)
		if !ok {
			b.Warn("async write fail err store failed", "err", err)
		}
	}
}

func (b *baseRW) closeReader() error {
	if b.Reader() == nil {
		return nil
	}

	var err error
	b.Info("close nested reader")
	err = b.Reader().Close()

	b.closeOnce.Do(func() {
		close(b.closed)
	})

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

	if err != nil {
		return errs.Wrapf(err, "%s close nested reader failed", b.Name())
	}
	return nil
}

func (b *baseRW) closeWriter() error {
	if b.Writer() == nil {
		return nil
	}

	var (
		err      error
		flushErr error
	)
	b.closeOnce.Do(func() {
		close(b.closed)
	})

	if b.bufSize > 0 && b.buf != nil {
		b.Info("close-flush")

		b.mu.Lock()
		flushErr = b.flushNoLock()
		if flushErr != nil {
			flushErr = errs.Wrap(flushErr, "close-flush failed")
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

	b.Info("close nested writer")
	err = b.Writer().Close()
	if err != nil {
		err = errs.Wrapf(err, "%s close nested writer failed", b.Name())
	}
	if flushErr != nil && err != nil {
		errors.Join(flushErr, err)
	}
	if err != nil {
		return err
	}
	if flushErr != nil {
		return flushErr
	}
	return nil
}

func (b *baseRW) hookCancel(_ int, _ []byte, _ error, _ int64, _ ...any) error {
	select {
	case <-b.Ctx().Done():
		return b.Ctx().Err()
	default:
		return nil
	}
}

func (b *baseRW) hookIncWritten(n int, _ []byte, _ error, _ int64, _ ...any) error {
	atomic.AddUint64(&b.nw, uint64(n))
	return nil
}

func (b *baseRW) hookIncReadn(n int, _ []byte, _ error, _ int64, _ ...any) error {
	atomic.AddUint64(&b.nr, uint64(n))
	return nil
}

func (b *baseRW) hookHash(n int, bs []byte, _ error, _ int64, _ ...any) error {
	_, _ = b.hash.Write(bs[:n])
	return nil
}

func (b *baseRW) hookDebugLogRead(n int, _ []byte, err error, cost int64, misc ...any) error {
	b.Debug("read hook log", "n", n, "err", err, "cost", fmt.Sprintf("%d ms", cost), "misc", misc)
	return nil
}

func (b *baseRW) hookDebugLogWrite(n int, _ []byte, err error, cost int64, misc ...any) error {
	b.Debug("write hook log", "n", n, "err", err, "cost", fmt.Sprintf("%d ms", cost), "misc", misc)
	return nil
}

func (b *baseRW) hookReadRateLimit(n int, _ []byte, _ error, _ int64, _ ...any) error {
	if n == 0 || b.rl == nil {
		return nil
	}
	e := b.rl.RxWaitN(n, 0)
	if e != nil {
		b.Warn("read rate limit failed", "n", n, "err", e.Error())
	}
	return nil
}

func (b *baseRW) hookWriteRateLimit(n int, _ []byte, _ error, _ int64, _ ...any) error {
	if n == 0 || b.rl == nil {
		return nil
	}
	e := b.rl.TxWaitN(n, 0)
	if e != nil {
		b.Warn("write rate limit failed", "n", n, "err", e.Error())
	}
	return nil
}

func (b *baseRW) hookChecksum(_ int, _ []byte, err error, _ int64, _ ...any) error {
	if !errors.Is(err, io.EOF) {
		return nil
	}

	checksum := b.Hash()
	if b.checksum != checksum {
		return errs.Wrapf(err, "checksum expected: %s, actual: %s", b.checksum, checksum)
	}

	return nil
}

func (b *baseRW) logSpeed(sub uint64, interval int64) {
	switch {
	case sub < 1024:
		b.Info("monitor", "speed", fmt.Sprintf("%.1fB/s", float64(sub)/float64(interval)))
	case sub >= 1024 && sub < 1024*1024:
		b.Info("monitor", "speed", fmt.Sprintf("%.3fKB/s", float64(sub)/1024/float64(interval)))
	default:
		b.Info("monitor", "speed", fmt.Sprintf("%.3fMB/s", float64(sub)/1024/1024/float64(interval)))
	}
}

func (b *baseRW) monitorSpeed() {
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

func ApplyCommonCfgToRW(rw RW, cfg *ExtraCfg) {
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
		case RoleReader:
			rw.EnableReadBuf(cfg.BufSize, cfg.EnableAsync, cfg.AsyncChanBufSize)
		case RoleWriter:
			rw.EnableWriteBuf(cfg.BufSize, cfg.Deadline, cfg.EnableAsync, cfg.AsyncChanBufSize)
		case RoleStarter:
			if rw.Writer() != nil {
				rw.EnableWriteBuf(cfg.BufSize, cfg.Deadline, cfg.EnableAsync, cfg.AsyncChanBufSize)
			} else {
				rw.EnableReadBuf(cfg.BufSize, cfg.EnableAsync, cfg.AsyncChanBufSize)
			}
		}
	}
}
