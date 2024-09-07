package sonicloader

import (
	"io"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/decoder"
	"github.com/bytedance/sonic/encoder"
	"github.com/donkeywon/golib/util/jsons"
)

func init() {
	Load()
}

func Load() {
	jsons.Unmarshal = sonic.Unmarshal
	jsons.Marshal = sonic.Marshal
	jsons.UnmarshalString = sonic.UnmarshalString
	jsons.MarshalString = sonic.MarshalString
	jsons.MarshalIndent = sonic.MarshalIndent
	jsons.NewEncoder = func(w io.Writer) jsons.JsonEncoder {
		return encoder.NewStreamEncoder(w)
	}
	jsons.NewDecoder = func(r io.Reader) jsons.JsonDecoder {
		return decoder.NewStreamDecoder(r)
	}
}
