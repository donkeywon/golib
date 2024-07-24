package json

import (
	"github.com/bytedance/sonic"
	"github.com/donkeywon/golib/util"
)

func Unmarshal(buf []byte, val interface{}) error {
	return sonic.Unmarshal(buf, val)
}

func Marshal(val interface{}) ([]byte, error) {
	return sonic.Marshal(val)
}

func UnmarshalString(s string, val interface{}) error {
	return Unmarshal(util.String2Bytes(s), val)
}

func MarshalString(val interface{}) (string, error) {
	bs, err := Marshal(val)
	return util.Bytes2String(bs), err
}
