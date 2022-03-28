package v2

import (
	"io"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/ftp"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(ReaderFtp, func() *Ftp { return NewFtp(ReaderFtp) }, NewFtpCfg)
	plugin.RegWithCfg(WriterFtp, func() *Ftp { return NewFtp(WriterFtp) }, NewFtpCfg)
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

type Ftp struct {
	Common
	Reader
	Writer

	c   *FtpCfg
	typ Type
}

func NewFtp(typ Type) *Ftp {
	f := &Ftp{
		typ: typ,
		c:   NewFtpCfg(),
	}

	if typ == ReaderFtp {
		r := CreateReader(string(typ))
		f.Common = r
		f.Reader = r
	} else {
		w := CreateWriter(string(typ))
		f.Common = w
		f.Writer = w
	}

	return f
}

func (f *Ftp) Type() Type {
	return f.typ
}

func (f *Ftp) Init() error {
	if f.typ == ReaderFtp {
		r, err := createFtpReader(f.c)
		if err != nil {
			return errs.Wrap(err, "create ftp reader failed")
		}
		f.Common.(Reader).WrapReader(r)
	} else {
		w, err := createFtpWriter(f.c)
		if err != nil {
			return errs.Wrap(err, "create ftp writer failed")
		}
		f.Common.(Writer).WrapWriter(w)
	}

	return f.Common.Init()
}

func (f *Ftp) WrapReader(io.ReadCloser) {
	panic(ErrInvalidWrap)
}

func (f *Ftp) WrapWriter(io.WriteCloser) {
	panic(ErrInvalidWrap)
}

func (f *Ftp) SetCfg(cfg *FtpCfg) {
	f.c = cfg
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
