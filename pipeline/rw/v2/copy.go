package v2

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/bytespool"
	"io"
)

const TypeCopy WorkerType = "copy"

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
		Worker:  CreateWorker(string(TypeCopy)),
		CopyCfg: NewCopyCfg(),
	}
}

func (c *Copy) Init() error {
	return c.Worker.Init()
}

func (c *Copy) Start() error {
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
	if rr, ok := c.Reader().(runner.Runner); ok {
		rr.Cancel()
		return nil
	}
	return c.Reader().Close()
}

func (c *Copy) Type() any {
	return TypeCopy
}

func (c *Copy) GetCfg() any {
	return c.CopyCfg
}
