package conv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBytes2String(t *testing.T) {
	bs := []byte("hello world")
	s := Bytes2String(bs)
	assert.Equal(t, "hello world", s)
	assert.Equal(t, len(bs), len(s))
}

func TestString2Bytes(t *testing.T) {
	s := "hello world"
	bs := String2Bytes(s)
	assert.Equal(t, []byte("hello world"), bs)
	assert.Equal(t, len(s), len(bs))
}

func TestBytes2StringRoundTrip(t *testing.T) {
	original := "round-trip test string"
	bs := String2Bytes(original)
	s := Bytes2String(bs)
	assert.Equal(t, original, s)
}

func TestBytes2StringEmpty(t *testing.T) {
	bs := []byte{}
	s := Bytes2String(bs)
	assert.Equal(t, "", s)
	assert.Equal(t, 0, len(s))
}

func TestString2BytesEmpty(t *testing.T) {
	bs := String2Bytes("")
	assert.Len(t, bs, 0)
}
