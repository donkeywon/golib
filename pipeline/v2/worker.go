package v2

import (
	"errors"
	"io"
	"reflect"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/runner"
)

var CreateWorker = newBaseWorker

type Worker interface {
	Common

	WriteTo(...io.WriteCloser)
	ReadFrom(...io.ReadCloser)

	Writers() []io.WriteCloser
	Readers() []io.ReadCloser
	LastWriter() io.WriteCloser
	LastReader() io.ReadCloser

	Reader() io.ReadCloser
	Writer() io.WriteCloser
}

type BaseWorker struct {
	runner.Runner

	r io.ReadCloser
	w io.WriteCloser

	ws []io.WriteCloser
	rs []io.ReadCloser
}

func newBaseWorker(name string) Worker {
	return &BaseWorker{
		Runner: runner.Create(name),
	}
}

func (b *BaseWorker) Init() error {
	for i := len(b.ws) - 2; i >= 0; i-- {
		if ww, ok := b.ws[i].(WriterWrapper); !ok {
			b.Error("writer is not WriterWrapper", nil, "writer", reflect.TypeOf(b.ws[i]))
			panic(ErrNotWrapper)
		} else {
			ww.Wrap(b.ws[i+1])
		}
	}
	b.w = b.ws[0]

	for i := len(b.rs) - 2; i >= 0; i-- {
		if rr, ok := b.rs[i].(ReaderWrapper); !ok {
			b.Error("reader is not ReaderWrapper", nil, "reader", reflect.TypeOf(b.rs[i]))
			panic(ErrNotWrapper)
		} else {
			rr.Wrap(b.rs[i+1])
		}
	}
	b.r = b.rs[0]

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

func (b *BaseWorker) Type() Type { panic("not implemented") }

func (b *BaseWorker) WriteTo(w ...io.WriteCloser) {
	b.ws = append(b.ws, w...)
}

func (b *BaseWorker) ReadFrom(r ...io.ReadCloser) {
	b.rs = append(b.rs, r...)
}

func (b *BaseWorker) Readers() []io.ReadCloser {
	return b.rs
}

func (b *BaseWorker) Writers() []io.WriteCloser {
	return b.ws
}

func (b *BaseWorker) Reader() io.ReadCloser {
	return b.r
}

func (b *BaseWorker) Writer() io.WriteCloser {
	return b.w
}

func (b *BaseWorker) LastWriter() io.WriteCloser {
	if len(b.ws) > 0 {
		return b.ws[len(b.ws)-1]
	}
	return nil
}

func (b *BaseWorker) LastReader() io.ReadCloser {
	if len(b.rs) > 0 {
		return b.rs[len(b.rs)-1]
	}
	return nil
}

func (b *BaseWorker) Close() error {
	defer b.Cancel()
	defer func() {
		err := recover()
		if err != nil {
			b.AppendError(errs.PanicToErrWithMsg(err, "panic on closing"))
		}
	}()
	var err error
	if b.Reader() != nil {
		err = b.Reader().Close()
	}
	if b.Writer() != nil {
		err = errors.Join(err, b.Writer().Close())
	}
	if err != nil {
		b.AppendError(errs.Wrap(err, "close failed"))
	}
	return nil
}

func (b *BaseWorker) WithOptions(...Option) {

}
