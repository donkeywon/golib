package aio

import (
	"io"

	"github.com/donkeywon/golib/util/bytespool"
	"github.com/donkeywon/golib/util/iou"
)

type buf struct {
	b      *bytespool.Bytes
	l      int
	offset int
}

func newBuf(n int) *buf {
	return &buf{
		b: bytespool.GetN(n),
	}
}

func (b *buf) readFrom(p []byte) int {
	nc := copy(b.b.B()[b.l:], p)
	b.l += nc
	return nc
}

func (b *buf) readFill(r io.Reader) (int, error) {
	nr, err := iou.ReadFill(r, b.b.B()[b.l:])
	b.l += nr
	return nr, err
}

func (b *buf) writeToBytes(p []byte) int {
	n := copy(p, b.bytes())
	b.offset += n
	return n
}

func (b *buf) writeTo(w io.Writer) (int, error) {
	n, err := w.Write(b.bytes())
	b.offset += n
	return n, err
}

func (b *buf) bytes() []byte {
	return b.b.B()[b.offset:b.l]
}

func (b *buf) isFull() bool {
	return b.l == b.b.Len()
}

func (b *buf) isEmpty() bool {
	return b.offset == b.b.Len()
}

func (b *buf) free() {
	b.b.Free()
}
