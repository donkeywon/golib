package jsons

import (
	"encoding/json"
	"io"
	"unsafe"
)

type JSONDecoder interface {
	Decode(v any) error
	UseNumber()
	DisallowUnknownFields()
	Buffered() io.Reader
	More() bool
}

type JSONEncoder interface {
	Encode(v any) error
	SetIndent(prefix, indent string)
	SetEscapeHTML(on bool)
}

var (
	Unmarshal     = json.Unmarshal
	Marshal       = json.Marshal
	MarshalIndent = json.MarshalIndent

	UnmarshalString = func(s string, val any) error { return Unmarshal(string2Bytes(s), val) }
	MarshalString   = func(val any) (string, error) {
		bs, err := Marshal(val)
		if err != nil {
			return "", err
		}
		return bytes2String(bs), nil
	}

	NewEncoder = func(w io.Writer) JSONEncoder {
		return json.NewEncoder(w)
	}
	NewDecoder = func(r io.Reader) JSONDecoder {
		return json.NewDecoder(r)
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
	MustMarshalIndent = func(v any, prefix, indent string) []byte {
		bs, err := MarshalIndent(v, prefix, indent)
		if err != nil {
			panic(err)
		}
		return bs
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

// for zero dep.
func bytes2String(bs []byte) string {
	return unsafe.String(unsafe.SliceData(bs), len(bs))
}

func string2Bytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
