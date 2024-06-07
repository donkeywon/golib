package pipeline

import (
	"errors"
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/bytespool"
)

func init() {
	plugin.Register(RWTypeCopy, func() interface{} { return NewCopyRW() })
	plugin.RegisterCfg(RWTypeCopy, func() interface{} { return NewCopyRWCfg() })
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
		RW: NewBaseRW("copy"),
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
	defer func() {
		err := recover()
		if err != nil {
			cp.AppendError(errs.Errorf("copy panic: %+v", err))
		}

		closeErr := cp.Close()
		if closeErr != nil {
			cp.AppendError(errs.Wrap(closeErr, "copy RW close fail"))
		}
	}()

	buf := bytespool.GetBytesN(cp.BufSize)
	defer buf.Free()

	_, err := io.CopyBuffer(cp.Writer(), cp, buf.B())
	if errors.Is(err, ErrStoppedManually) {
		cp.Info("stopped manually", "err", err)
		err = nil
	}

	return err
}

func (cp *CopyRW) Type() interface{} {
	return RWTypeCopy
}

func (cp *CopyRW) GetCfg() interface{} {
	return cp.CopyRWCfg
}
