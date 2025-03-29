package rw

import (
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/bytespool"
)

const defaultBufSize = 32 * 1024

func init() {
	plugin.RegWithCfg(TypeCopy, func() RW { return NewCopy() }, func() any { return NewCopyCfg() })
}

const TypeCopy Type = "copy"

type CopyCfg struct {
	BufSize int `json:"bufSize" yaml:"bufSize"`
}

func NewCopyCfg() *CopyCfg {
	return &CopyCfg{}
}

type Copy struct {
	RW
	*CopyCfg
}

func NewCopy() *Copy {
	return &Copy{
		RW: CreateBase(string(TypeCopy)),
	}
}

func (cp *Copy) Init() error {
	if !cp.IsStarter() {
		return errs.New("copy must be Starter")
	}

	if cp.Reader() == nil || cp.Writer() == nil {
		return errs.New("copy must nest both Reader and Writer")
	}

	if cp.CopyCfg.BufSize <= 0 {
		cp.CopyCfg.BufSize = defaultBufSize
	}

	return cp.RW.Init()
}

func (cp *Copy) Start() error {
	buf := bytespool.GetN(cp.BufSize)
	defer buf.Free()

	_, err := io.CopyBuffer(cp.Writer(), cp, buf.B())
	if err != nil {
		return errs.Wrap(err, "io copy failed")
	}

	return nil
}

func (cp *Copy) Stop() error {
	cp.Cancel()
	return cp.RW.Stop()
}

func (cp *Copy) Type() Type {
	return TypeCopy
}
