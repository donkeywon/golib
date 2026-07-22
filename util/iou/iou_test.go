package iou

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadFill(t *testing.T) {
	t.Run("full read exact size", func(t *testing.T) {
		r := strings.NewReader("hello")
		buf := make([]byte, 5)
		n, err := ReadFill(buf, r)
		assert.NoError(t, err)
		assert.Equal(t, 5, n)
		assert.Equal(t, "hello", string(buf))
	})

	t.Run("full read buffer larger than reader", func(t *testing.T) {
		r := strings.NewReader("hi")
		buf := make([]byte, 10)
		n, err := ReadFill(buf, r)
		assert.ErrorIs(t, err, io.EOF)
		assert.Equal(t, 2, n)
		assert.Equal(t, "hi", string(buf[:2]))
	})

	t.Run("partial read buffer smaller than reader", func(t *testing.T) {
		r := strings.NewReader("hello world")
		buf := make([]byte, 5)
		n, err := ReadFill(buf, r)
		assert.NoError(t, err)
		assert.Equal(t, 5, n)
		assert.Equal(t, "hello", string(buf))
	})

	t.Run("empty reader", func(t *testing.T) {
		r := strings.NewReader("")
		buf := make([]byte, 5)
		n, err := ReadFill(buf, r)
		assert.ErrorIs(t, err, io.EOF)
		assert.Equal(t, 0, n)
	})

	t.Run("zero length buffer", func(t *testing.T) {
		r := strings.NewReader("hello")
		buf := make([]byte, 0)
		n, err := ReadFill(buf, r)
		assert.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("multiple read cycles", func(t *testing.T) {
		// Use a reader that returns small chunks
		r := &chunkedReader{data: []byte("hello world"), chunkSize: 2}
		buf := make([]byte, 11)
		n, err := ReadFill(buf, r)
		assert.NoError(t, err)
		assert.Equal(t, 11, n)
		assert.Equal(t, "hello world", string(buf))
	})
}

func TestReadFillError(t *testing.T) {
	t.Run("reader returns error", func(t *testing.T) {
		r := &errorReader{data: []byte("abc"), errAfter: 2}
		buf := make([]byte, 10)
		n, err := ReadFill(buf, r)
		assert.Error(t, err)
		assert.Equal(t, "test error", err.Error())
		assert.Equal(t, 2, n)
	})
}

// negativeReader is a malicious reader that violates the io.Reader contract
// by returning a negative count. ReadFill must panic when it detects this.
type negativeReader struct{}

func (r *negativeReader) Read(p []byte) (int, error) {
	return -1, nil
}

func TestReadFill_NegativeCount(t *testing.T) {
	assert.PanicsWithValue(t, "iou.ReadFill: reader returned negative count from Read", func() {
		ReadFill(make([]byte, 10), &negativeReader{})
	})
}

// chunkedReader returns data in small chunks to simulate multiple read cycles.
type chunkedReader struct {
	data      []byte
	offset    int
	chunkSize int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	end := r.offset + r.chunkSize
	if end > len(r.data) {
		end = len(r.data)
	}
	n := copy(p, r.data[r.offset:end])
	r.offset += n
	return n, nil
}

// errorReader returns data up to a point then returns an error.
type errorReader struct {
	data     []byte
	offset   int
	errAfter int
}

func (r *errorReader) Read(p []byte) (int, error) {
	if r.offset >= r.errAfter {
		return 0, errors.New("test error")
	}
	n := copy(p, r.data[r.offset:r.errAfter])
	r.offset += n
	return n, nil
}
