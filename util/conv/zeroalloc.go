package conv

import (
	"unsafe"
)

// Bytes2String converts a byte slice to a string without copying.
// The returned string shares the underlying memory with bs.
//
// WARNING: bs must not be modified after this call. Mutating bs would
// silently corrupt the returned string, violating Go's string-immutability
// contract and causing undefined behaviour.
func Bytes2String(bs []byte) string {
	return unsafe.String(unsafe.SliceData(bs), len(bs))
}

// String2Bytes converts a string to a byte slice without copying.
// The returned slice shares the underlying memory with s.
//
// WARNING: the returned bytes must never be written to. Writing to them would
// corrupt the original string and cause undefined behaviour.
// Use this only for read-only operations (e.g. passing to functions that
// accept []byte but do not modify it).
func String2Bytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
