package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/donkeywon/golib/errs"
)

type Pipeline struct {
	ws []Worker
}

func New() *Pipeline {
	return &Pipeline{}
}

func (p *Pipeline) Add(ws ...Worker) *Pipeline {
	p.ws = append(p.ws, ws...)
	return p
}

func (p *Pipeline) Init(ctx context.Context) error {
	if len(p.ws) == 0 {
		panic("no workers")
	}

	var (
		pr  io.ReadCloser
		pw  io.WriteCloser
		err error
	)

	for i := 0; i < len(p.ws)-1; i++ {
		if p.ws[i].Writer() != nil || p.ws[i+1].Reader() != nil {
			panic(fmt.Sprintf("writer or reader already exists between workers: %T(%d) and %T(%d)", p.ws[i], i, p.ws[i+1], i+1))
		}

		if p.ws[i].SupportZeroCopy() && p.ws[i+1].SupportZeroCopy() && !p.ws[i].WriterWrapped() && !p.ws[i+1].ReaderWrapped() {
			pr, pw, err = os.Pipe()
			if err != nil {
				return errs.Wrap(err, "create os pipe failed")
			}
		} else {
			pr, pw = io.Pipe()
		}

		p.ws[i].WriteToWriter(pw)
		p.ws[i+1].ReadFromReader(pr)
	}

	return nil
}

type initializer interface {
	Init(context.Context) error
}

func (p *Pipeline) Run(ctx context.Context) error {
	for i, w := range p.ws {
		if initer, ok := w.(initializer); ok {
			err := initer.Init(ctx)
			if err != nil {
				return errs.Wrapf(err, "init worker failed: %T(%d)", w, i)
			}
		}
	}

	allErr := make([]error, len(p.ws))

	wg := &sync.WaitGroup{}
	for i, w := range p.ws {
		wg.Go(func() {
			err := w.Run(ctx)
			if err != nil {
				allErr[i] = errs.Wrapf(err, "worker failed: %T(%d)", w, i)
			}
		})
	}

	wg.Wait()
	return errors.Join(allErr...)
}
