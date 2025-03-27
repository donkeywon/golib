package v2

import (
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/ftp"
)

const (
	TypeFtpReader ReaderType = "ftp"
	TypeFtpWriter ReaderType = "ftp"
)

type FtpCfg struct {
	*ftp.Cfg
	Path string `json:"path" yaml:"path" validate:"required"`
}

func NewFtpCfg() *FtpCfg {
	return &FtpCfg{
		Cfg: &ftp.Cfg{},
	}
}

type FtpReader struct {
	Reader
	*FtpCfg
}

func (f *FtpReader) Init() error {
	r, err := createFtpReader(f.FtpCfg)
	if err != nil {
		return errs.Wrap(err, "create ftp reader failed")
	}

	f.Wrap(r)
	return f.Reader.Init()
}

func (f *FtpReader) Type() any {
	return TypeFtpReader
}

func (f *FtpReader) GetCfg() any {
	return f.FtpCfg
}

type FtpWriter struct {
	Writer
	*FtpCfg
}

func (f *FtpWriter) Init() error {
	w, err := createFtpWriter(f.FtpCfg)
	if err != nil {
		return errs.Wrap(err, "create ftp writer failed")
	}
	f.Wrap(w)
	return f.Writer.Init()
}

func (f *FtpWriter) Type() any {
	return TypeFtpWriter
}

func (f *FtpWriter) GetCfg() any {
	return f.FtpCfg
}

func createFtpReader(ftpCfg *FtpCfg) (*ftp.Reader, error) {
	r := ftp.NewReader()
	r.Cfg = ftpCfg.Cfg
	r.Path = ftpCfg.Path

	err := r.Init()
	if err != nil {
		return nil, errs.Wrap(err, "init ftp reader failed")
	}

	return r, nil
}

func createFtpWriter(ftpCfg *FtpCfg) (*ftp.Writer, error) {
	w := ftp.NewWriter()
	w.Cfg = ftpCfg.Cfg
	w.Path = ftpCfg.Path

	err := w.Init()
	if err != nil {
		return nil, errs.Wrap(err, "init ftp writer failed")
	}

	return w, nil
}
