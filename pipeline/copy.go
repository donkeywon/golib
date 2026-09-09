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

func (c *copyWorker) Run(ctx context.Context) (err error) {
	if c.bufSize <= 0 {
		c.bufSize = 32 * 1024
	}
	bs := make([]byte, c.bufSize)

	done := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)

		select {
		case <-ctx.Done():
			closeErr := c.Close(true)
			if closeErr != nil {
				errCh <- closeErr
			}
		case <-done:
		}
	}()

	defer func() {
		close(done)

		closeErr, ok := <-errCh
		if !ok {
			closeErr = c.Close(false)
		}
		if closeErr != nil {
			err = errors.Join(err, errs.Wrap(closeErr, "close failed"))
		}
	}()

	_, err = io.CopyBuffer(c.Writer(), c.Reader(), bs)
	if err != nil {
		return errs.Wrap(err, "copy failed")
	}

	return nil
}
