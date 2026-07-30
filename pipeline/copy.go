package pipeline

import (
	"context"
	"errors"
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/rs/zerolog"
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

func (c *copyWorker) Start(ctx context.Context) (err error) {
	if c.bufSize <= 0 {
		c.bufSize = 32 * 1024
	}
	bs := make([]byte, c.bufSize)

	defer func() {
		closeErr := c.Close(false)
		if closeErr != nil {
			err = errors.Join(err, errs.Wrap(closeErr, "close failed"))
		}
	}()

	_, err = io.CopyBuffer(c.Writer(), c.Reader(), bs)
	select {
	case <-c.Stopping():
		if err != nil {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("copy stopped manually")
			err = nil
		}
	default:
	}
	if err != nil {
		return errs.Wrap(err, "copy failed")
	}

	return nil
}

func (c *copyWorker) Stop(ctx context.Context) error {
	return c.Close(true)
}
