package rw

import (
	"os"
	"strconv"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(TypeFile, func() RW { return NewFile() }, func() any { return NewFileCfg() })
}

const TypeFile Type = "file"

type FileCfg struct {
	Path string `json:"path" yaml:"path"`
	Perm uint32 `json:"perm" yaml:"perm"`
}

func NewFileCfg() *FileCfg {
	return &FileCfg{}
}

type File struct {
	RW
	*FileCfg

	f          *os.File
	parsedPerm int64
}

func NewFile() *File {
	return &File{
		RW: CreateBase(string(TypeFile)),
	}
}

func (f *File) Init() error {
	var err error

	if f.Perm == 0 {
		f.Perm = 644
	}
	f.parsedPerm, err = strconv.ParseInt(strconv.Itoa(int(f.Perm)), 8, 32)
	if err != nil {
		return errs.Wrapf(err, "invalid perm: %d", f.Perm)
	}

	if f.IsStarter() {
		return errs.New("file cannot be Starter")
	}

	if f.IsReader() {
		f.f, err = os.OpenFile(f.Path, os.O_RDONLY, os.FileMode(f.parsedPerm))
		f.NestReader(f.f)
	} else {
		f.f, err = os.OpenFile(f.Path, os.O_WRONLY|os.O_CREATE, os.FileMode(f.parsedPerm))
		f.NestWriter(f.f)
	}

	if err != nil {
		return errs.Wrap(err, "open file failed")
	}

	return f.RW.Init()
}

func (f *File) Type() Type {
	return TypeFile
}
