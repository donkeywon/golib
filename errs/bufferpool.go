package errs

import (
	"bytes"
	"sync"
)

// for zero dep.
var _bufferPool = &sync.Pool{
	New: func() interface{} {
		return &buffer{
			Buffer: &bytes.Buffer{},
		}
	}}

type buffer struct {
	*bytes.Buffer
	p *sync.Pool
}

func getBuffer() *buffer {
	b, _ := _bufferPool.Get().(*buffer)
	b.p = _bufferPool
	b.Reset()
	return b
}

func (b *buffer) free() {
	b.p.Put(b)
}
