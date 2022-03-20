package buffer

import (
	"io"

	"github.com/donkeywon/golib/util/bytespool"
	"github.com/donkeywon/golib/util/iou"
)

type FixedBuffer struct {
	b    *bytespool.Bytes
	roff int
	woff int
}

func NewFixedBuffer(n int) *FixedBuffer {
	return &FixedBuffer{
		b: bytespool.GetN(n),
	}
}

func (b *FixedBuffer) Len() int {
	return b.woff
}

func (b *FixedBuffer) Cap() int {
	return b.b.Cap()
}

func (b *FixedBuffer) HasRemaining() bool {
	return b.woff > b.roff
}

// Write writes len(p) bytes from p to buf.
// It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
// io.ErrShortWrite means buf is full.
func (b *FixedBuffer) Write(p []byte) (n int, err error) {
	n = copy(b.wremain(), p)
	b.woff += n
	if n < len(p) {
		// buf full
		err = io.ErrShortWrite
	}

	return
}

// ReadFrom reads data from r until io.EOF or b is full, and appends it to the buffer.
// The return value n is the number of bytes read. Any error except io.EOF
// encountered during the read is also returned
// nil err means Reader io.EOF
// io.ErrShortWrite means buf full and Reader not io.EOF.
func (b *FixedBuffer) ReadFrom(r io.Reader) (int64, error) {
	nr, err := iou.ReadFill(r, b.wremain())
	b.woff += nr
	if err == nil {
		// buf full and not EOF
		err = io.ErrShortWrite
	}
	if err == io.EOF {
		err = nil
	}

	return int64(nr), err
}

// Read reads up to len(p) bytes into p. It returns the number of bytes
// read (0 <= n <= len(p)) and any error encountered.
// err is nil or io.EOF(buf all read).
func (b *FixedBuffer) Read(p []byte) (n int, err error) {
	if b.roff == b.woff {
		return 0, io.EOF
	}
	n = copy(p, b.rremain())
	b.roff += n
	if b.roff == b.woff {
		err = io.EOF
	}
	return
}

// WriteTo writes data to w until there's no more data to write or
// when an error occurs. The return value n is the number of bytes
// written. Any error encountered during the write is also returned.
func (b *FixedBuffer) WriteTo(w io.Writer) (n int64, err error) {
	if b.roff == b.woff {
		return 0, nil
	}

	var nw int
	remain := b.rremain()
	remainlen := len(remain)

	nw, err = w.Write(remain)
	if nw > remainlen {
		panic("buffer.FixedBuffer.WriteTo: invalid write count")
	}

	b.roff += nw
	n += int64(nw)
	if err != nil {
		return
	}

	if nw < remainlen {
		return n, io.ErrShortWrite
	}

	return
}

func (b *FixedBuffer) Free() {
	b.b.Free()
}

func (b *FixedBuffer) wremain() []byte {
	return b.b.B()[b.woff:]
}

func (b *FixedBuffer) rremain() []byte {
	return b.b.B()[b.roff:b.woff]
}
