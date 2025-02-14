package pipeline

import (
	"errors"
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/bytespool"
)

func init() {
	plugin.RegWithCfg(RWTypeCopy, func() any { return NewCopyRW() }, func() any { return NewCopyRWCfg() })
}

const RWTypeCopy RWType = "copy"

type CopyRWCfg struct {
	BufSize int `json:"bufSize" yaml:"bufSize"`
}

func NewCopyRWCfg() *CopyRWCfg {
	return &CopyRWCfg{}
}

type CopyRW struct {
	RW
	*CopyRWCfg
}

func NewCopyRW() *CopyRW {
	return &CopyRW{
		RW: CreateBaseRW(string(RWTypeCopy)),
	}
}

func (cp *CopyRW) Init() error {
	if !cp.IsStarter() {
		return errs.New("copyRW must be Starter")
	}

	if cp.Reader() == nil || cp.Writer() == nil {
		return errs.New("copyRW must nest both Reader and Writer")
	}

	return cp.RW.Init()
}

func (cp *CopyRW) Start() error {
	buf := bytespool.GetBytesN(cp.BufSize)
	defer buf.Free()

	_, err := io.CopyBuffer(cp.Writer(), cp.Reader(), buf.B())
	if err != nil {
		select {
		case <-cp.Stopping():
			cp.Warn("copy stopped manually", "err", err)
			err = nil
		default:
		}
	}

	closeErr := cp.Close()
	if closeErr != nil {
		cp.Error("close rw failed", closeErr)
	}

	return errs.Wrap(err, "io copy failed")
}

func (cp *CopyRW) Stop() error {
	return errors.Join(cp.Close(), cp.RW.Stop())
}

func (cp *CopyRW) Type() any {
	return RWTypeCopy
}

func (cp *CopyRW) GetCfg() any {
	return cp.CopyRWCfg
}
