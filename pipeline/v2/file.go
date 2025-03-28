package v2

import (
	"io"
	"os"
	"strconv"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(ReaderFile, func() *File { return NewFile(ReaderFile, os.O_RDONLY) }, NewFileCfg)
	plugin.RegWithCfg(WriterFile, func() *File { return NewFile(WriterFile, os.O_CREATE|os.O_WRONLY) }, NewFileCfg)
}

const (
	ReaderFile Type = "rfile"
	WriterFile Type = "wfile"
)

type FileCfg struct {
	Path string `json:"path" yaml:"path"`
	Perm uint32 `json:"perm" yaml:"perm"`
}

func NewFileCfg() *FileCfg {
	return &FileCfg{}
}

type File struct {
	Common
	Reader
	Writer

	c          *FileCfg
	f          *os.File
	typ        Type
	parsedPerm int64
	flag       int
}

func NewFile(typ Type, flag int) *File {
	f := &File{
		c:    NewFileCfg(),
		flag: flag,
		typ:  typ,
	}

	if typ == ReaderFile {
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

func (f *File) Init() error {
	var err error

	if f.c.Perm == 0 {
		f.c.Perm = 644
	}

	f.parsedPerm, err = strconv.ParseInt(strconv.Itoa(int(f.c.Perm)), 8, 32)
	if err != nil {
		return errs.Wrapf(err, "invalid file perm: %d", f.c.Perm)
	}

	f.f, err = os.OpenFile(f.c.Path, f.flag, os.FileMode(f.c.Perm))
	if err != nil {
		return errs.Wrapf(err, "failed to open file: %s", f.c.Path)
	}

	if f.typ == ReaderFile {
		f.Common.(Reader).WrapReader(f.f)
	} else {
		f.Common.(Writer).WrapWriter(f.f)
	}

	return f.Common.Init()
}

func (f *File) Type() Type {
	return f.typ
}

func (f *File) WrapReader(io.ReadCloser) {
	panic(ErrInvalidWrap)
}

func (f *File) WrapWriter(io.WriteCloser) {
	panic(ErrInvalidWrap)
}

func (f *File) SetCfg(c *FileCfg) {
	f.c = c
}
