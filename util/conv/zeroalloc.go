package conv

import "unsafe"

func Bytes2String(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}

func String2BytesNoCap(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}

func String2Bytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}

func Uint64ToInt64(val uint64) int64 {
	return *(*int64)(unsafe.Pointer(&val))
}

func Uint64ToFloat64(val uint64) float64 {
	return *(*float64)(unsafe.Pointer(&val))
}

func Int64ToUint64(val int64) uint64 {
	return *(*uint64)(unsafe.Pointer(&val))
}

func Float64ToUint64(val float64) uint64 {
	return *(*uint64)(unsafe.Pointer(&val))
}
