package bytespool

import "sync"

type Bytes struct {
	p *sync.Pool
	b []byte
}

var (
	_bytesPool = &sync.Pool{New: func() interface{} {
		return &Bytes{
			b: make([]byte, 0),
		}
	}}
)

func GetBytes() *Bytes {
	b, _ := _bytesPool.Get().(*Bytes)
	b.p = _bytesPool
	return b
}

func GetBytesN(n int) *Bytes {
	b := GetBytes()
	b.Grow(n)
	return b
}

func (b *Bytes) Free() {
	b.p.Put(b)
}

func (b *Bytes) JustFree() {}

func (b *Bytes) Len() int {
	return len(b.b)
}

func (b *Bytes) Cap() int {
	return cap(b.b)
}

func (b *Bytes) Grow(n int) {
	if n <= 0 {
		return
	}
	if b.Cap() >= n {
		b.b = b.b[:n]
		return
	}

	b.b = make([]byte, n)
}

func (b *Bytes) Shrink(n int) {
	if b.Len() <= n {
		return
	}

	if n <= 0 {
		n = 0
	}

	b.b = b.b[:n]
}

func (b *Bytes) B() []byte {
	return b.b
}
