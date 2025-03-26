package v2

import (
	"io"
	"reflect"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

var CreateWorker = newBaseWorker

type WorkerType string

type Worker interface {
	runner.Runner
	plugin.Plugin

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
			b.w = b.ws[i]
		}
	}

	for i := len(b.rs) - 1; i >= 0; i-- {
		if rr, ok := b.rs[i].(ReaderWrapper); !ok {
			b.Error("reader is not ReaderWrapper", nil, "reader", reflect.TypeOf(b.rs[i]))
			panic(ErrNotWrapper)
		} else {
			rr.Wrap(b.rs[i+1])
			b.r = b.rs[i]
		}
	}

	var err error
	for i := len(b.ws) - 1; i >= 0; i-- {
		if ww, ok := b.ws[i].(runner.Runner); !ok {
			err = runner.Init(ww)
			if err != nil {
				return errs.Wrapf(err, "init writer failed: %s", reflect.TypeOf(b.ws[i]).String())
			}
		}
	}
	for i := len(b.rs) - 1; i >= 0; i-- {
		if rr, ok := b.rs[i].(runner.Runner); !ok {
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

func (b *BaseWorker) Type() any { panic("not implemented") }

func (b *BaseWorker) GetCfg() any { panic("not implemented") }

func (b *BaseWorker) WriteTo(w ...io.WriteCloser) {
	b.ws = append(b.ws, w...)
}

func (b *BaseWorker) ReadFrom(r ...io.ReadCloser) {
	b.rs = append(b.rs, r...)
}

func (b *BaseWorker) Reader() io.ReadCloser {
	return b.r
}

func (b *BaseWorker) Writer() io.WriteCloser {
	return b.w
}
