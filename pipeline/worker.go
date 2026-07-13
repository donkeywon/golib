package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/reflects"
)

type Worker interface {
	runner.Runner

	WriteTo(io.Writer, ...WriterWrapFunc)
	ReadFrom(io.Reader, ...ReaderWrapFunc)
}

type BaseWorker struct {
	runner.Base

	r io.Reader
	w io.Writer

	rs []io.Reader
	ws []io.Writer

	wwrappers []WriterWrapFunc
	rwrappers []ReaderWrapFunc

	wcloses []func() error
	rcloses []func() error
}

func (wk *BaseWorker) Init(ctx context.Context) error {
	if wk.r == nil || wk.w == nil {
		panic("nil reader or writer")
	}

	wk.wrapWriters()
	wk.wrapReaders()

	return nil
}

func (wk *BaseWorker) Start(ctx context.Context) error {
	panic("not implemented")
}

func (wk *BaseWorker) Stop(ctx context.Context) error {
	panic("not implemented")
}

func (wk *BaseWorker) Close() error {
	return errors.Join(closes(wk.rcloses), closes(wk.wcloses))
}

func closes(cs []func() error) error {
	var allErr []error
	for _, c := range cs {
		func(c func() error) {
			defer func() {
				p := recover()
				if p != nil {
					allErr = append(allErr, errs.PanicToErrWithMsg(p, fmt.Sprintf("panic on close %s", reflects.GetFuncName(c))))
				}
			}()

			err := c()
			if err != nil {
				allErr = append(allErr, errs.Wrapf(err, "failed close %s", reflects.GetFuncName(c)))
			}
		}(c)
	}
	if len(allErr) == 0 {
		return nil
	}
	if len(allErr) == 1 {
		return allErr[0]
	}
	return errors.Join(allErr...)
}

func (wk *BaseWorker) wrapWriters() {
	wk.ws = append(wk.ws, wk.w)
	wk.appendCloseWriter(wk.w)
	for _, wrapper := range wk.wwrappers {
		wk.w = wrapper(wk.w)
		wk.ws = append(wk.ws, wk.w)
		wk.appendCloseWriter(wk.w)
	}
	slices.Reverse(wk.wcloses)
}

func (wk *BaseWorker) wrapReaders() {
	wk.rs = append(wk.rs, wk.r)
	wk.appendCloseReader(wk.r)
	for _, wrapper := range wk.rwrappers {
		wk.r = wrapper(wk.r)
		wk.rs = append(wk.rs, wk.r)
		wk.appendCloseReader(wk.r)
	}
}

type noerrCloser interface {
	Close()
}

func (wk *BaseWorker) appendCloseWriter(w io.Writer) {
	switch c := w.(type) {
	case io.Closer:
		wk.wcloses = append(wk.wcloses, c.Close)
	case noerrCloser:
		wk.wcloses = append(wk.wcloses, func() error { c.Close(); return nil })
	}
}

func (wk *BaseWorker) appendCloseReader(r io.Reader) {
	switch c := r.(type) {
	case io.Closer:
		wk.rcloses = append(wk.rcloses, c.Close)
	case noerrCloser:
		wk.rcloses = append(wk.rcloses, func() error { c.Close(); return nil })
	}
}

func (wk *BaseWorker) WriteTo(w io.Writer, wrappers ...WriterWrapFunc) {
	wk.w = w
	wk.wwrappers = wrappers
}

func (wk *BaseWorker) ReaderFrom(r io.Reader, wrappers ...ReaderWrapFunc) {
	wk.r = r
	wk.rwrappers = wrappers
}
