package pipeline

import (
	"os"
	"strconv"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
)

func init() {
	plugin.RegWithCfg(RWTypeFile, func() any { return NewFileRW() }, func() any { return NewFileRWCfg() })
}

const RWTypeFile RWType = "file"

type FileRWCfg struct {
	Path string `json:"path" yaml:"path"`
	Perm uint32 `json:"perm" yaml:"perm"`
}

func NewFileRWCfg() *FileRWCfg {
	return &FileRWCfg{}
}

type FileRW struct {
	RW
	*FileRWCfg

	f          *os.File
	parsedPerm int64
}

func NewFileRW() *FileRW {
	return &FileRW{
		RW: CreateBaseRW(string(RWTypeFile)),
	}
}

func (f *FileRW) Init() error {
	var err error

	if f.Perm == 0 {
		f.Perm = 644
	}
	f.parsedPerm, err = strconv.ParseInt(strconv.Itoa(int(f.Perm)), 8, 32)
	if err != nil {
		return errs.Wrapf(err, "invalid perm: %d", f.Perm)
	}

	if f.IsStarter() {
		return errs.New("fileRW cannot be Starter")
	}

	if f.IsReader() {
		f.f, err = os.OpenFile(f.Path, os.O_RDONLY, os.FileMode(f.parsedPerm))
		_ = f.NestReader(f.f)
	} else {
		f.f, err = os.OpenFile(f.Path, os.O_WRONLY|os.O_CREATE, os.FileMode(f.parsedPerm))
		_ = f.NestWriter(f.f)
	}

	if err != nil {
		return errs.Wrap(err, "open file failed")
	}

	return f.RW.Init()
}

func (f *FileRW) Type() any {
	return RWTypeFile
}

func (f *FileRW) GetCfg() any {
	return f.FileRWCfg
}
