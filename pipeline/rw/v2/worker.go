package v2

import (
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"io"
)

type WorkerType string

type Worker interface {
	runner.Runner
	plugin.Plugin
}

type BaseWorker struct {
	runner.Runner

	r io.ReadCloser
	w io.WriteCloser
}

func NewBaseWorker(name string) Worker {
	return &BaseWorker{
		Runner: runner.Create(name),
	}
}

func (b *BaseWorker) WrapReader(r io.ReadCloser) {
	if b.r == nil {
		b.r = r
		return
	}

	if rr, ok := r.(Reader); ok {
		rr.Wrap(b.r)
		b.r = rr
		return
	}

	panic(ErrCannotWrap)
}

func (b *BaseWorker) WrapWriter(w io.WriteCloser) {
	if b.w == nil {
		b.w = w
		return
	}

	if ww, ok := w.(Writer); ok {
		ww.Wrap(b.w)
		b.w = ww
		return
	}

	panic(ErrCannotWrap)
}
