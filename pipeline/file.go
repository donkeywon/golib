package pipeline

import (
	"io"
	"os"
	"strconv"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(ReaderFile, func() Common { return NewFileReader() }, func() any { return NewFileCfg() })
	plugin.RegWithCfg(WriterFile, func() Common { return NewFileWriter() }, func() any { return NewFileCfg() })
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
	*os.File
	*FileCfg

	parsedPerm int64
	flag       int
}

func newFile(flag int) *File {
	return &File{
		FileCfg: NewFileCfg(),
		flag:    flag,
	}
}

func (f *File) init() error {
	var err error

	if f.Perm == 0 {
		f.Perm = 644
	}

	f.parsedPerm, err = strconv.ParseInt(strconv.Itoa(int(f.Perm)), 8, 32)
	if err != nil {
		return errs.Wrapf(err, "invalid file perm: %d", f.Perm)
	}

	f.File, err = os.OpenFile(f.Path, f.flag, os.FileMode(f.Perm))
	if err != nil {
		return errs.Wrapf(err, "failed to open file: %s", f.Path)
	}

	return nil
}

type FileReader struct {
	Reader

	f *File
}

func NewFileReader() *FileReader {
	return &FileReader{
		Reader: CreateReader(string(ReaderFile)),
		f:      newFile(os.O_RDONLY),
	}
}

func (f *FileReader) Init() error {
	err := f.f.init()
	if err != nil {
		return err
	}

	f.Reader.WrapReader(f.f)

	return f.Reader.Init()
}

func (f *FileReader) Wrap(io.ReadCloser) {
	panic(ErrInvalidWrap)
}

func (f *FileReader) Type() Type {
	return ReaderFile
}

func (f *FileReader) SetCfg(cfg any) {
	f.f.FileCfg = cfg.(*FileCfg)
}

type FileWriter struct {
	Writer

	f *File
}

func NewFileWriter() *FileWriter {
	return &FileWriter{
		Writer: CreateWriter(string(WriterFile)),
		f:      newFile(os.O_WRONLY | os.O_CREATE),
	}
}

func (f *FileWriter) Init() error {
	err := f.f.init()
	if err != nil {
		return err
	}

	f.Writer.WrapWriter(f.f)

	return f.Writer.Init()
}

func (f *FileWriter) Wrap(io.WriteCloser) {
	panic(ErrInvalidWrap)
}

func (f *FileWriter) Type() Type {
	return WriterFile
}

func (f *FileWriter) SetCfg(cfg any) {
	f.f.FileCfg = cfg.(*FileCfg)
}
