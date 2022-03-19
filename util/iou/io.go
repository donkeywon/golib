package iou

import "io"

// ReadFill read from reader as much as possible.
func ReadFill(r io.Reader, buf []byte) (n int, err error) {
	var (
		l  = len(buf)
		nn int
	)
	for n < l && err == nil {
		nn, err = r.Read(buf[n:])
		n += nn
	}
	return
}
