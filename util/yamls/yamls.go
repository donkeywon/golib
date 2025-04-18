package yamls

import (
	"io"
	"unsafe"

	"github.com/goccy/go-yaml"
)

type YAMLDecoder interface {
	Decode(any) error
}

type YAMLEncoder interface {
	Encode(any) error
}

var (
	Unmarshal = yaml.Unmarshal
	Marshal   = yaml.Marshal

	UnmarshalString = func(buf string, val any) error {
		return Unmarshal(string2Bytes(buf), val)
	}
	MarshalString = func(val any) (string, error) {
		bs, err := Marshal(val)
		if err != nil {
			return "", err
		}

		return bytes2String(bs), err
	}

	NewEncoder = func(w io.Writer) YAMLEncoder {
		return yaml.NewEncoder(w)
	}
	NewDecoder = func(r io.Reader) YAMLDecoder {
		return yaml.NewDecoder(r)
	}

	MustMarshal = func(v any) []byte {
		bs, err := Marshal(v)
		if err != nil {
			panic(err)
		}
		return bs
	}
	MustUnmarshal = func(bs []byte, v any) {
		err := Unmarshal(bs, v)
		if err != nil {
			panic(err)
		}
	}
	MustMarshalString = func(v any) string {
		bs, err := Marshal(v)
		if err != nil {
			panic(err)
		}
		return bytes2String(bs)
	}
	MustUnmarshalString = func(s string, v any) {
		err := Unmarshal(string2Bytes(s), v)
		if err != nil {
			panic(err)
		}
	}
)

func bytes2String(bs []byte) string {
	return unsafe.String(unsafe.SliceData(bs), len(bs))
}

func string2Bytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
