package v2

import (
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/bytespool"
)

func init() {
	plugin.RegWithCfg(WorkerCopy, func() any { return NewCopy() }, func() any { return NewCopyCfg() })
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
	*CopyCfg
}

func NewCopy() *Copy {
	return &Copy{
		Worker:  CreateWorker(string(WorkerCopy)),
		CopyCfg: NewCopyCfg(),
	}
}

func (c *Copy) Init() error {
	return c.Worker.Init()
}

func (c *Copy) Start() error {
	defer c.Close()

	bufSize := c.CopyCfg.BufSize
	if bufSize <= 0 {
		bufSize = 32 * 1024
	}
	buf := bytespool.GetN(bufSize)
	defer buf.Free()

	_, err := io.CopyBuffer(c.Writer(), c.Reader(), buf.B())
	if err != nil {
		return errs.Wrap(err, "copy failed")
	}

	return nil
}

func (c *Copy) Stop() error {
	if rc, ok := c.Reader().(canceler); ok {
		rc.Cancel()
		return nil
	}
	return c.Reader().Close()
}

func (c *Copy) Type() Type {
	return WorkerCopy
}

func (c *Copy) GetCfg() *CopyCfg {
	return c.CopyCfg
}
