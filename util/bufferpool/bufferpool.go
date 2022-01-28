package bufferpool

import (
	"bufio"
	"bytes"
	"sync"
)

var _bufferPool = &sync.Pool{
	New: func() any {
		return &Buffer{
			Buffer: &bytes.Buffer{},
		}
	}}

type Buffer struct {
	*bytes.Buffer
	p *sync.Pool
}

func GetBuffer() *Buffer {
	b, _ := _bufferPool.Get().(*Buffer)
	b.p = _bufferPool
	b.Reset()
	return b
}

func (b *Buffer) Free() {
	b.p.Put(b)
}

func (b *Buffer) Lines() []string {
	var lines []string

	s := bufio.NewScanner(b.Buffer)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	return lines
}

func (b *Buffer) JustFree() {}
