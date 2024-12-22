package pipeline

import (
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(WorkerCopy, func() Worker { return NewCopy() }, func() any { return NewCopyCfg() })
}

const WorkerCopy Type = "copy"

type CopyCfg struct {
	BufSize int `json:"bufSize" yaml:"bufSize"`
}

func NewCopyCfg() *CopyCfg {
	return &CopyCfg{}
}

type Copy struct {
	Worker

	c *CopyCfg
}

func NewCopy() *Copy {
	return &Copy{
		Worker: CreateWorker(string(WorkerCopy)),
		c:      NewCopyCfg(),
	}
}

func (c *Copy) Init() error {
	return c.Worker.Init()
}

func (c *Copy) Start() error {
	defer c.Close()

	bufSize := c.c.BufSize
	if bufSize <= 0 {
		bufSize = 32 * 1024
	}
	bs := make([]byte, bufSize)

	w := c.Writer()
	if w == nil {
		return errs.New("writer is nil")
	}
	if ww, ok := w.(Writer); ok {
		w = ww.DirectWriter()
	}
	r := c.Reader()
	if r == nil {
		return errs.New("reader is nil")
	}
	if rr, ok := r.(Reader); ok {
		r = rr.DirectReader()
	}

	_, err := io.CopyBuffer(w, r, bs)
	select {
	case <-c.Stopping():
		// stop before copy done
		if err != nil {
			c.Warn("copy stopped manually before done", "err", err)
			err = nil
		}
	default:
	}
	if err != nil {
		return errs.Wrap(err, "copy failed")
	}

	return nil
}

func (c *Copy) Stop() error {
	defer c.Cancel()
	switch rc := c.Reader().(type) {
	case io.Closer:
		return rc.Close()
	case canceler:
		rc.Cancel()
	}
	return nil
}

func (c *Copy) SetCfg(cfg any) {
	c.c = cfg.(*CopyCfg)
}
