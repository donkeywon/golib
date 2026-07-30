package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/runner"
)

type Worker interface {
	runner.Runner

	Writer() io.Writer
	Reader() io.Reader
	WriteToWriter(io.Writer, ...WriterWrapFunc)
	ReadFromReader(io.Reader, ...ReaderWrapFunc)
	WithWriterWrappers(...WriterWrapFunc)
	WithReaderWrappers(...ReaderWrapFunc)
	WriterWrapped() bool
	ReaderWrapped() bool
	SupportZeroCopy() bool
}

type BaseWorker struct {
	runner.Base

	r io.Reader
	w io.Writer

	rs []io.Reader
	ws []io.Writer

	wwrappers []WriterWrapFunc
	rwrappers []ReaderWrapFunc

	forceCloseOnce func() error
	closeOnce      func() error
}

func (wk *BaseWorker) Init(ctx context.Context) error {
	wk.wrapWriters()
	wk.wrapReaders()

	wk.forceCloseOnce = sync.OnceValue(func() error {
		return errors.Join(closeReaders(wk.rs), closeWriters(wk.ws))
	})
	wk.closeOnce = sync.OnceValue(func() error {
		ws := slices.Clone(wk.ws)
		slices.Reverse(ws)
		return errors.Join(closeReaders(wk.rs), closeWriters(ws))
	})

	return nil
}

func (wk *BaseWorker) Start(ctx context.Context) error {
	panic("not implemented")
}

func (wk *BaseWorker) Stop(ctx context.Context) error {
	panic("not implemented")
}

func (wk *BaseWorker) Close(force bool) error {
	if force {
		return wk.forceCloseOnce()
	}

	return wk.closeOnce()
}

type flusher interface {
	Flush() error
}

type flusher2 interface {
	Flush()
}

func closeWriters(ws []io.Writer) error {
	allErr := make([]error, 0, len(ws))
	for _, w := range ws {
		err := closeWriter(w)
		if err != nil {
			allErr = append(allErr, err)
		}
	}
	return errors.Join(allErr...)
}

func closeReaders(rs []io.Reader) error {
	allErr := make([]error, 0, len(rs))
	for _, r := range rs {
		err := closeReader(r)
		if err != nil {
			allErr = append(allErr, err)
		}
	}
	return errors.Join(allErr...)
}

func closeWriter(w io.Writer) (err error) {
	defer func() {
		p := recover()
		if p != nil {
			err = errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on close writer: %T", w))
		}
	}()

	switch c := w.(type) {
	case io.Closer:
		e := c.Close()
		if e != nil {
			err = errs.Wrapf(e, "close writer failed: %T", w)
		}
	case flusher:
		e := c.Flush()
		if e != nil {
			err = errs.Wrapf(e, "flush writer failed: %T", w)
		}
	case flusher2:
		c.Flush()
	}

	return err
}

func closeReader(r io.Reader) (err error) {
	defer func() {
		p := recover()
		if p != nil {
			err = errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on close reader: %T", r))
		}
	}()

	if c, ok := r.(io.Closer); ok {
		e := c.Close()
		if e != nil {
			err = errs.Wrapf(e, "close reader failed: %T", r)
		}
	}
	return err
}

func (wk *BaseWorker) wrapWriters() {
	if wk.w == nil {
		return
	}
	wk.ws = append(wk.ws, wk.w)
	for _, wrapper := range wk.wwrappers {
		wk.w = wrapper(wk.w)
		wk.ws = append(wk.ws, wk.w)
	}
}

func (wk *BaseWorker) wrapReaders() {
	if wk.r == nil {
		return
	}
	wk.rs = append(wk.rs, wk.r)
	for _, wrapper := range wk.rwrappers {
		wk.r = wrapper(wk.r)
		wk.rs = append(wk.rs, wk.r)
	}
}

func (wk *BaseWorker) Writer() io.Writer {
	return wk.w
}

func (wk *BaseWorker) Reader() io.Reader {
	return wk.r
}

func (wk *BaseWorker) WriteToWriter(w io.Writer, wrappers ...WriterWrapFunc) {
	wk.w = w
	wk.wwrappers = append(wk.wwrappers, wrappers...)
}

func (wk *BaseWorker) ReadFromReader(r io.Reader, wrappers ...ReaderWrapFunc) {
	wk.r = r
	wk.rwrappers = append(wk.rwrappers, wrappers...)
}

func (wk *BaseWorker) WithWriterWrappers(wrappers ...WriterWrapFunc) {
	wk.wwrappers = append(wk.wwrappers, wrappers...)
}

func (wk *BaseWorker) WithReaderWrappers(wrappers ...ReaderWrapFunc) {
	wk.rwrappers = append(wk.rwrappers, wrappers...)
}

func (wk *BaseWorker) WriterWrapped() bool {
	return len(wk.wwrappers) > 0
}

func (wk *BaseWorker) ReaderWrapped() bool {
	return len(wk.rwrappers) > 0
}

func (wk *BaseWorker) SupportZeroCopy() bool {
	return false
}
