package pipeline

import "github.com/donkeywon/golib/plugin"

func init() {
	plugin.RegWithCfg(ReaderNop, func() Reader { return NewNopReader() }, func() any { return NewNopCfg() })
	plugin.RegWithCfg(WriterNop, func() Writer { return NewNopWriter() }, func() any { return NewNopCfg() })
}

const (
	ReaderNop Type = "rnop"
	WriterNop Type = "wnop"
)

type NopCfg struct {
}

func NewNopCfg() *NopCfg {
	return &NopCfg{}
}

type NopReader struct {
	Reader
	*NopCfg
}

func NewNopReader() *NopReader {
	return &NopReader{
		Reader: CreateReader("r"),
		NopCfg: NewNopCfg(),
	}
}

type NopWriter struct {
	Writer
	*NopCfg
}

func NewNopWriter() *NopWriter {
	return &NopWriter{
		Writer: CreateWriter("w"),
		NopCfg: NewNopCfg(),
	}
}
