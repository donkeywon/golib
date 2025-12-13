package pipeline

import (
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/ftp"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.Reg(ReaderFtp, func() Reader { return NewFtpReader() }, func() any { return NewFtpCfg() })
	plugin.Reg(WriterFtp, func() Writer { return NewFtpWriter() }, func() any { return NewFtpCfg() })
}

const (
	ReaderFtp Type = "rftp"
	WriterFtp Type = "wftp"
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

func NewFtpReader() *FtpReader {
	return &FtpReader{
		Reader: CreateReader(string(ReaderFtp)),
		FtpCfg: NewFtpCfg(),
	}
}

func (f *FtpReader) Init() error {
	r, err := createFtpReader(f.FtpCfg)
	if err != nil {
		return errs.Wrap(err, "create ftp reader failed")
	}

	f.Reader.WrapReader(r)
	return f.Reader.Init()
}

func (f *FtpReader) SetCfg(c any) {
	f.FtpCfg = c.(*FtpCfg)
}

func (f *FtpReader) WrapReader(io.Reader) {
	panic(ErrInvalidWrap)
}

type FtpWriter struct {
	Writer
	*FtpCfg
}

func NewFtpWriter() *FtpWriter {
	return &FtpWriter{
		Writer: CreateWriter(string(WriterFtp)),
		FtpCfg: NewFtpCfg(),
	}
}

func (f *FtpWriter) Init() error {
	w, err := createFtpWriter(f.FtpCfg)
	if err != nil {
		return errs.Wrap(err, "create ftp writer failed")
	}
	f.Writer.WrapWriter(w)
	return f.Writer.Init()
}

func (f *FtpWriter) WrapWriter(io.Writer) {
	panic(ErrInvalidWrap)
}

func (f *FtpWriter) SetCfg(c any) {
	f.FtpCfg = c.(*FtpCfg)
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
