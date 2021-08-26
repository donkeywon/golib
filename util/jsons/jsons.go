package jsons

import (
	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/decoder"
	"github.com/bytedance/sonic/encoder"
)

var (
	Unmarshal       = sonic.Unmarshal
	Marshal         = sonic.Marshal
	UnmarshalString = sonic.UnmarshalString
	MarshalString   = sonic.MarshalString
	MarshalIndent   = sonic.MarshalIndent
	NewEncoder      = encoder.NewStreamEncoder
	NewDecoder      = decoder.NewStreamDecoder
)
