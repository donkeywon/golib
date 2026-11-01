package pipeline

import (
	"context"
	"errors"
	"io"

	"github.com/donkeywon/golib/errs"
)

type copyWorker struct {
	BaseWorker

	bufSize int
}

func NewCopyWorker(bufSize int) Worker {
	return &copyWorker{
		bufSize: bufSize,
	}
}

type forceCloseResult struct {
	err    error
	closed bool
}

func (c *copyWorker) Run(ctx context.Context) (err error) {
	if c.bufSize <= 0 {
		c.bufSize = 32 * 1024
	}
	bs := make([]byte, c.bufSize)

	done := make(chan struct{})
	errCh := make(chan forceCloseResult, 1)
	go func() {
		defer close(errCh)

		select {
		case <-ctx.Done():
			errCh <- forceCloseResult{err: c.Close(true), closed: true}
		case <-done:
			errCh <- forceCloseResult{}
		}
	}()

	defer func() {
		close(done)

		res := <-errCh
		if !res.closed {
			closeErr := c.Close(false)
			if closeErr != nil {
				err = errors.Join(err, errs.Wrap(closeErr, "close failed"))
			}
		} else {
			err = errors.Join(err, errs.Wrap(res.err, "close failed"))
		}
	}()

	_, err = io.CopyBuffer(c.Writer(), c.Reader(), bs)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return errs.Wrap(err, "copy canceled")
		}
		return errs.Wrap(err, "copy failed")
	}

	return nil
}
