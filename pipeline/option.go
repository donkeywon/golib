package pipeline

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"time"

	"github.com/donkeywon/golib/aio"
	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/ratelimit"
	"github.com/donkeywon/golib/runner"
	"github.com/zeebo/xxh3"
)

type ReaderWrapFunc func(io.Reader) io.Reader

type WriterWrapFunc func(io.Writer) io.Writer

type itemSetter interface {
	Set(Common)
}

type Option interface {
	apply(*option)
}

type optionFunc func(*option)

func (f optionFunc) apply(o *option) {
	f(o)
}

type multiOption []Option

func (mo multiOption) apply(opt *option) {
	for _, o := range mo {
		o.apply(opt)
	}
}

type option struct {
	tees            []io.Writer
	ws              []io.Writer
	cs              []closeFunc
	readerWrapFuncs []ReaderWrapFunc
	writerWrapFuncs []WriterWrapFunc
}

func newOption() *option {
	return &option{}
}

func (o *option) with(opts ...Option) {
	for _, opt := range opts {
		opt.apply(o)
	}
}

func (o *option) onclose() error {
	return doAllClose(o.cs)
}

func doAllClose(closes []closeFunc) error {
	var err []error
	for _, c := range closes {
		func(c func() error) {
			defer func() {
				p := recover()
				if p != nil {
					err = append(err, errs.PanicToErrWithMsg(p, "panic on close"))
				}
			}()

			e := c()
			if e != nil {
				err = append(err, errs.Wrap(e, "err on close"))
			}
		}(c)
	}
	if len(err) == 0 {
		return nil
	}
	if len(err) == 1 {
		return err[0]
	}

	return errors.Join(err...)
}

func EnableBufRead(bufSize int) Option {
	return WrapReader(func(r io.Reader) io.Reader {
		return bufio.NewReaderSize(r, bufSize)
	})
}

func EnableBufWrite(bufSize int) Option {
	return WrapWriter(func(w io.Writer) io.Writer {
		return bufio.NewWriterSize(w, bufSize)
	})
}

func EnableAsyncRead(bufSize int, queueSize int) Option {
	return WrapReader(func(r io.Reader) io.Reader {
		return aio.NewAsyncReader(r, aio.BufSize(bufSize), aio.QueueSize(queueSize))
	})
}

func EnableAsyncWrite(bufSize int, queueSize int, deadline time.Duration) Option {
	return WrapWriter(func(w io.Writer) io.Writer {
		return aio.NewAsyncWriter(w, aio.BufSize(bufSize), aio.QueueSize(queueSize), aio.Deadline(deadline))
	})
}

func WrapReader(f ReaderWrapFunc) Option {
	return optionFunc(func(o *option) {
		o.readerWrapFuncs = append(o.readerWrapFuncs, f)
	})
}

func WrapWriter(f WriterWrapFunc) Option {
	return optionFunc(func(o *option) {
		o.writerWrapFuncs = append(o.writerWrapFuncs, f)
	})
}

// Tee only for Reader
func Tee(w ...io.Writer) Option {
	return optionFunc(func(o *option) {
		o.tees = append(o.tees, w...)
	})
}

// MultiWrite only for Writer
func MultiWrite(w ...io.Writer) Option {
	return optionFunc(func(o *option) {
		o.ws = append(o.ws, w...)
	})
}

func OnClose(c ...closeFunc) Option {
	return optionFunc(func(o *option) {
		o.cs = append(o.cs, c...)
	})
}

type logger struct {
	Common

	msg string
}

func (l *logger) Write(p []byte) (int, error) {
	l.Info(l.msg, "len", len(p))
	return len(p), nil
}

func (l *logger) Set(c Common) {
	l.Common = c
}

func LogWrite() Option {
	return MultiWrite(&logger{msg: "write"})
}

func LogRead() Option {
	return Tee(&logger{msg: "read"})
}

type hasher struct {
	c        Common
	h        hash.Hash
	checksum string
}

func (h *hasher) Write(p []byte) (int, error) {
	return h.h.Write(p)
}

func (h *hasher) Set(c Common) {
	h.c = c
}

func (h *hasher) Close() error {
	var bs []byte
	switch hs := h.h.(type) {
	case *xxh3.Hasher:
		bbs := hs.Sum128().Bytes()
		bs = bbs[:]
	default:
		bs = hs.Sum(nil)
	}
	hash := hex.EncodeToString(bs)
	if len(h.checksum) > 0 {
		if h.checksum != hash {
			return errs.Errorf("checksum not match, wanted: %s, actual: %s", h.checksum, hash)
		}
	}
	h.c.Store(consts.FieldHash, hash)
	return nil
}

func Hash(h hash.Hash) Option {
	hs := &hasher{h: h}
	return multiOption{MultiWrite(hs), OnClose(hs.Close)}
}

func Checksum(checksum string, h hash.Hash) Option {
	hs := &hasher{h: h, checksum: checksum}
	return multiOption{MultiWrite(hs), OnClose(hs.Close)}
}

type progressLogger struct {
	Common

	t *time.Ticker

	sizeGetter    func() int64
	size          int64
	offset        int64
	lastLogOffset int64
	lastLogAt     int64
}

func newProgressLogger(interval time.Duration) *progressLogger {
	return &progressLogger{
		t:         time.NewTicker(interval),
		lastLogAt: time.Now().UnixNano(),
	}
}

type hasSize interface {
	Size() int64
}

type hasSize2 interface {
	Size() int
}

func (p *progressLogger) Write(b []byte) (n int, err error) {
	n = len(b)
	p.offset += int64(n)
	select {
	case <-p.t.C:
		p.logProgress()
	default:
	}

	return
}

func (p *progressLogger) logProgress() {
	if p.sizeGetter == nil {
		p.Warn("cannot get size, skip log progress")
		return
	}

	if p.size <= 0 {
		p.size = p.sizeGetter()
		if p.size <= 0 {
			p.Warn("size le zero, skip log progress")
			return
		}
	}

	inc := p.offset - p.lastLogOffset
	if inc <= 0 {
		return
	}

	interval := time.Now().UnixNano() - p.lastLogAt
	if interval <= 0 {
		return
	}

	percent := fmt.Sprintf("%.3f%%", float64(p.offset)/float64(p.size)*100)
	speed := float64(inc) / float64(interval) * 1000000000
	switch {
	case speed < 1024:
		p.Info("progress", "offset", p.offset, "size", p.size, "percent", percent, "avgSpeed", fmt.Sprintf("%.1fB/s", speed))
	case speed >= 1024 && speed < 1024*1024:
		p.Info("progress", "offset", p.offset, "size", p.size, "percent", percent, "avgSpeed", fmt.Sprintf("%.3fKB/s", speed/1024))
	default:
		p.Info("progress", "offset", p.offset, "size", p.size, "percent", percent, "avgSpeed", fmt.Sprintf("%.3fMB/s", speed/1048576))
	}
	p.lastLogOffset = p.offset
	p.lastLogAt = time.Now().UnixNano()
}

func (p *progressLogger) Close() error {
	p.t.Stop()
	p.logProgress()
	return nil
}

func (p *progressLogger) Set(c Common) {
	p.Common = c
	switch s := c.(type) {
	case hasSize:
		p.sizeGetter = s.Size
	case hasSize2:
		p.sizeGetter = func() int64 {
			return int64(s.Size())
		}
	}
}

func ProgressLog(interval time.Duration) Option {
	p := newProgressLogger(interval)
	return multiOption{Tee(p), OnClose(p.Close)}
}

type rateLimit struct {
	Common

	cfg   *ratelimit.Cfg
	rl    ratelimit.RxTxRateLimiter
	write bool
}

func newRateLimit(cfg *ratelimit.Cfg) *rateLimit {
	return &rateLimit{
		cfg: cfg,
	}
}

func (rl *rateLimit) Write(p []byte) (n int, err error) {
	n = len(p)
	if n == 0 || rl.rl == nil {
		return
	}

	var e error
	if rl.write {
		e = rl.rl.TxWaitN(n, 0)
		if e != nil {
			rl.Warn("write rate limit failed", "n", n, "err", err)
		}
	} else {
		e = rl.rl.RxWaitN(n, 0)
		if e != nil {
			rl.Warn("read rate limit failed", "n", n, "err", err)
		}
	}
	return
}

func (rl *rateLimit) Close() error {
	runner.Stop(rl.rl)
	return nil
}

func (rl *rateLimit) Set(c Common) {
	rl.Common = c
	ratelimiter := plugin.CreateWithCfg[ratelimit.Type, ratelimit.RxTxRateLimiter](rl.cfg.Type, rl.cfg.Cfg)
	ratelimiter.Inherit(c)
	err := runner.Init(ratelimiter)
	if err != nil {
		rl.Error("init ratelimiter failed", err, "type", rl.cfg.Type, "cfg", rl.cfg.Cfg)
		return
	}
	if _, ok := c.(Writer); ok {
		rl.write = true
	}
	rl.rl = ratelimiter
}

func RateLimit(cfg *ratelimit.Cfg) Option {
	return MultiWrite(newRateLimit(cfg))
}

func setToTeesAndMultiWriters(c Common) Option {
	return optionFunc(func(o *option) {
		for _, w := range o.ws {
			if setter, ok := w.(itemSetter); ok {
				setter.Set(c)
			}
		}
		for _, w := range o.tees {
			if setter, ok := w.(itemSetter); ok {
				setter.Set(c)
			}
		}
	})
}
