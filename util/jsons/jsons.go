package jsons

import (
	"encoding/json"
	"io"
	"unsafe"
)

type JsonDecoder interface {
	Decode(v any) error
	UseNumber()
	DisallowUnknownFields()
	Buffered() io.Reader
	More() bool
}

type JsonEncoder interface {
	Encode(v any) error
	SetIndent(prefix, indent string)
	SetEscapeHTML(on bool)
}

var (
	Unmarshal     = json.Unmarshal
	Marshal       = json.Marshal
	MarshalIndent = json.MarshalIndent

	UnmarshalString = func(buf string, val interface{}) error { return json.Unmarshal(toBytes(buf), val) }
	MarshalString   = func(val interface{}) (string, error) {
		bs, err := json.Marshal(val)
		if err != nil {
			return "", err
		}
		return toString(bs), nil
	}

	NewEncoder = func(w io.Writer) JsonEncoder {
		return json.NewEncoder(w)
	}
	NewDecoder = func(r io.Reader) JsonDecoder {
		return json.NewDecoder(r)
	}
)

func toString(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}

func toBytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}
