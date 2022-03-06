package gojsonloader

import (
	"io"

	"github.com/donkeywon/golib/util/conv"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/goccy/go-json"
)

func init() {
	Load()
}

func Load() {
	jsons.Marshal = json.Marshal
	jsons.Unmarshal = json.Unmarshal
	jsons.UnmarshalString = func(buf string, val any) error {
		return json.Unmarshal(conv.String2Bytes(buf), val)
	}
	jsons.MarshalString = func(val any) (string, error) {
		bs, err := json.Marshal(val)
		if err != nil {
			return "", err
		}
		return conv.Bytes2String(bs), nil
	}
	jsons.MarshalIndent = json.MarshalIndent
	jsons.NewEncoder = func(w io.Writer) jsons.JSONEncoder {
		return json.NewEncoder(w)
	}
	jsons.NewDecoder = func(r io.Reader) jsons.JSONDecoder {
		return json.NewDecoder(r)
	}
}
