package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/runner"
)

type Pipeline struct {
	runner.Base

	ws []Worker
}

func (p *Pipeline) Add(ws ...Worker) {
	p.ws = append(p.ws, ws...)
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

func (p *Pipeline) Start(ctx context.Context) error {
	for i, w := range p.ws {
		err := runner.Init(ctx, w)
		if err != nil {
			return errs.Wrapf(err, "init worker failed: %T(%d)", w, i)
		}
	}

	allErr := make([]error, len(p.ws))

	wg := &sync.WaitGroup{}
	for i, w := range p.ws {
		wg.Go(func() {
			err := runner.Start(ctx, w)
			if err != nil {
				allErr[i] = errs.Wrapf(err, "worker failed: %T(%d)", w, i)
			}
		})
	}

	wg.Wait()
	return errors.Join(allErr...)
}

func (p *Pipeline) Stop(ctx context.Context) error {
	return runner.Stop(ctx, p.ws[0])
}
