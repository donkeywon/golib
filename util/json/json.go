package json

import (
	"github.com/bytedance/sonic"
)

func Unmarshal(buf []byte, val interface{}) error {
	return sonic.Unmarshal(buf, val)
}

func Marshal(val interface{}) ([]byte, error) {
	return sonic.Marshal(val)
}

func UnmarshalString(s string, val interface{}) error {
	return sonic.UnmarshalString(s, val)
}

func MarshalString(val interface{}) (string, error) {
	return sonic.MarshalString(val)
}
