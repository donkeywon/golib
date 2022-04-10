package pipeline

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/donkeywon/golib/util/yamls"
)

var CreateWorker = newBaseWorker

type WorkerResult struct {
	Data        map[string]any   `json:"data" yaml:"data"`
	ReadersData []map[string]any `json:"readersData" yaml:"readersData"`
	WritersData []map[string]any `json:"writersData" yaml:"writersData"`
}

type WorkerCfg struct {
	CommonCfgWithOption
	Readers []ReaderCfg `json:"readers" yaml:"readers"`
	Writers []WriterCfg `json:"writers" yaml:"writers"`
}

func (wc *WorkerCfg) WriteTo(typ Type, cfg any, opt CommonOption) *WorkerCfg {
	wc.Writers = append(wc.Writers, WriterCfg{
		CommonCfgWithOption: CommonCfgWithOption{
			CommonCfg: CommonCfg{
				Type: typ,
				Cfg:  cfg,
			},
			CommonOption: opt,
		},
	})

	return wc
}

func (wc *WorkerCfg) ReadFrom(typ Type, cfg any, opt CommonOption) *WorkerCfg {
	wc.Readers = append(wc.Readers, ReaderCfg{
		CommonCfgWithOption: CommonCfgWithOption{
			CommonCfg: CommonCfg{
				Type: typ,
				Cfg:  cfg,
			},
			CommonOption: opt,
		},
	})

	return wc
}

func (wc *WorkerCfg) WriteToWriter(c CommonCfgWithOption) *WorkerCfg {
	wc.Writers = append(wc.Writers, WriterCfg{c})
	return wc
}

func (wc *WorkerCfg) ReadFromReader(c CommonCfgWithOption) *WorkerCfg {
	wc.Readers = append(wc.Readers, ReaderCfg{CommonCfgWithOption: c})
	return wc
}

type workerCfgWithoutCommonCfg struct {
	Readers []ReaderCfg `json:"readers" yaml:"readers"`
	Writers []WriterCfg `json:"writers" yaml:"writers"`
}

func (wc *WorkerCfg) UnmarshalJSON(data []byte) error {
	err := wc.CommonCfgWithOption.UnmarshalJSON(data)
	if err != nil {
		return err
	}
	return wc.customUnmarshal(data, jsons.Unmarshal)
}

func (wc *WorkerCfg) UnmarshalYAML(data []byte) error {
	err := wc.CommonCfgWithOption.UnmarshalYAML(data)
	if err != nil {
		return err
	}
	return wc.customUnmarshal(data, yamls.Unmarshal)
}

func (wc *WorkerCfg) customUnmarshal(data []byte, unmarshal func([]byte, any) error) error {
	wcc := workerCfgWithoutCommonCfg{}
	err := unmarshal(data, &wcc)
	if err != nil {
		return err
	}
	wc.Readers = wcc.Readers
	wc.Writers = wcc.Writers
	return nil
}

func (wc *WorkerCfg) build() Worker {
	worker := plugin.CreateWithCfg[Type, Worker](wc.Type, wc.Cfg)

	for _, readerCfg := range wc.Readers {
		worker.ReadFrom(readerCfg.build())
	}
	for _, writerCfg := range wc.Writers {
		worker.WriteTo(writerCfg.build())
	}

	return worker
}

type Worker interface {
	Common

	WriteTo(...io.Writer)
	ReadFrom(...io.Reader)

	Writers() []io.Writer
	Readers() []io.Reader
	LastWriter() io.Writer
	LastReader() io.Reader

	Reader() io.Reader
	Writer() io.Writer

	Result() *WorkerResult
}

type BaseWorker struct {
	runner.Runner

	r io.Reader
	w io.Writer

	ws []io.Writer
	rs []io.Reader
}

func newBaseWorker(name string) Worker {
	return &BaseWorker{
		Runner: runner.Create(name),
	}
}

func (b *BaseWorker) Init() error {
	for i := len(b.ws) - 2; i >= 0; i-- {
		if ww, ok := b.ws[i].(writerWrapper); !ok {
			b.Error("writer is not WriterWrapper", nil, "writer", reflect.TypeOf(b.ws[i]))
			panic(ErrNotWrapper)
		} else {
			ww.WrapWriter(b.ws[i+1])
		}
	}
	if len(b.ws) > 0 {
		b.w = b.ws[0]
	}

	for i := len(b.rs) - 2; i >= 0; i-- {
		if rr, ok := b.rs[i].(readerWrapper); !ok {
			b.Error("reader is not ReaderWrapper", nil, "reader", reflect.TypeOf(b.rs[i]))
			panic(ErrNotWrapper)
		} else {
			rr.WrapReader(b.rs[i+1])
		}
	}
	if len(b.rs) > 0 {
		b.r = b.rs[0]
	}

	var err error
	for i := len(b.ws) - 1; i >= 0; i-- {
		if ww, ok := b.ws[i].(runner.Runner); ok {
			ww.Inherit(b)
			err = runner.Init(ww)
			if err != nil {
				return errs.Wrapf(err, "init writer failed: %s", reflect.TypeOf(b.ws[i]).String())
			}
		}
	}
	for i := len(b.rs) - 1; i >= 0; i-- {
		if rr, ok := b.rs[i].(runner.Runner); ok {
			rr.Inherit(b)
			err = runner.Init(rr)
			if err != nil {
				return errs.Wrapf(err, "init reader failed: %s", reflect.TypeOf(b.rs[i]).String())
			}
		}
	}

	return b.Runner.Init()
}

func (b *BaseWorker) Start() error {
	panic("not implemented")
}

func (b *BaseWorker) Stop() error {
	panic("not implemented")
}

func (b *BaseWorker) WriteTo(w ...io.Writer) {
	b.ws = append(b.ws, w...)
}

func (b *BaseWorker) ReadFrom(r ...io.Reader) {
	b.rs = append(b.rs, r...)
}

func (b *BaseWorker) Readers() []io.Reader {
	return b.rs
}

func (b *BaseWorker) Writers() []io.Writer {
	return b.ws
}

func (b *BaseWorker) Reader() io.Reader {
	return b.r
}

func (b *BaseWorker) Writer() io.Writer {
	return b.w
}

func (b *BaseWorker) LastWriter() io.Writer {
	if len(b.ws) > 0 {
		return b.ws[len(b.ws)-1]
	}
	return nil
}

func (b *BaseWorker) LastReader() io.Reader {
	if len(b.rs) > 0 {
		return b.rs[len(b.rs)-1]
	}
	return nil
}

func (b *BaseWorker) Close() error {
	defer b.Cancel()
	err := errors.Join(b.closeReaders(), b.closeWriters())
	if err != nil {
		b.AppendError(errs.Wrap(err, "close failed"))
	}
	return nil
}

func (b *BaseWorker) WithOptions(...Option) {}

func (b *BaseWorker) Result() *WorkerResult {
	d := &WorkerResult{
		Data:        b.LoadAll(),
		ReadersData: make([]map[string]any, len(b.rs)),
		WritersData: make([]map[string]any, len(b.ws)),
	}

	for i, r := range b.rs {
		if c, ok := r.(Common); ok {
			d.ReadersData[i] = c.LoadAll()
		}
	}
	for i, w := range b.ws {
		if c, ok := w.(Common); ok {
			d.WritersData[i] = c.LoadAll()
		}
	}
	return d
}

func (b *BaseWorker) closeReaders() error {
	var err []error
	for i, r := range b.rs {
		e := closeReader(i, r)
		if e != nil {
			err = append(err, e)
		}
	}

	if len(err) == 0 {
		return nil
	}
	if len(err) == 1 {
		return err[0]
	}
	return errors.Join(err...)
}

func closeReader(idx int, r io.Reader) (err error) {
	defer func() {
		p := recover()
		if p != nil {
			err = errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on close reader(%d) %s", idx, getName(r)))
		}
	}()

	if c, ok := r.(io.Closer); ok {
		e := c.Close()
		if e != nil {
			err = errs.Wrapf(e, "err on close reader(%d) %s", idx, getName(r))
		}
	}
	return
}

func closeWriter(idx int, w io.Writer) (err error) {
	defer func() {
		p := recover()
		if p != nil {
			err = errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on close writer(%d) %s", idx, getName(w)))
		}
	}()

	if c, ok := w.(io.Closer); ok {
		e := c.Close()
		if e != nil {
			err = errs.Wrapf(e, "err on close writer(%d) %s", idx, getName(w))
		}
	}
	return
}

func flushWriter(idx int, w io.Writer) (err error) {
	defer func() {
		p := recover()
		if p != nil {
			err = errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on flush writer(%d) %s", idx, getName(w)))
		}
	}()

	switch f := w.(type) {
	case flusher:
		e := f.Flush()
		if e != nil {
			err = errs.Wrapf(e, "err on flush writer(%d) %s", idx, getName(w))
		}
	case flusher2:
		f.Flush()
	}
	return
}

func (b *BaseWorker) closeWriters() error {
	var err []error
	for i, w := range b.ws {
		e := flushWriter(i, w)
		if e != nil {
			err = append(err, e)
		}
		e = closeWriter(i, w)
		if e != nil {
			err = append(err, e)
		}
	}

	if len(err) == 0 {
		return nil
	}
	if len(err) == 1 {
		return err[0]
	}
	return errors.Join(err...)
}

type hasName interface {
	Name() string
}

func getName(v any) string {
	switch vv := v.(type) {
	case hasName:
		return vv.Name()
	default:
		return reflect.TypeOf(v).String()
	}
}
