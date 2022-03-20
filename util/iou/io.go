package iou

import "io"

// ReadFill read from reader as much as possible.
// The return value n is the number of bytes read. Any error encountered during r.Read is also returned.
// n == len(buf) or nil error means buf is full.
func ReadFill(r io.Reader, buf []byte) (n int, err error) {
	var (
		l  = len(buf)
		nn int
	)
	for n < l && err == nil {
		nn, err = r.Read(buf[n:])
		if nn < 0 {
			panic("iou.ReadFill: reader returned negative count from Read")
		}
		n += nn
	}
	return
}
