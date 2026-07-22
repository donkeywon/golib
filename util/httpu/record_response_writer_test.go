package httpu

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecordResponseWriter_WriteHeader(t *testing.T) {
	rw := NewRecordResponseWriter(httptest.NewRecorder())
	assert.Equal(t, http.StatusOK, rw.Status())

	rw.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, rw.Status())
}

func TestRecordResponseWriter_WriteHeaderPanicsOnNegative(t *testing.T) {
	rw := NewRecordResponseWriter(httptest.NewRecorder())
	assert.Panics(t, func() {
		rw.WriteHeader(-1)
	})
}

func TestRecordResponseWriter_WriteHeaderZeroPanics(t *testing.T) {
	rw := NewRecordResponseWriter(httptest.NewRecorder())
	assert.Panics(t, func() {
		rw.WriteHeader(0)
	})
}

func TestRecordResponseWriter_Write(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewRecordResponseWriter(rec)

	n, err := rw.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	n, err = rw.Write([]byte(" world"))
	assert.NoError(t, err)
	assert.Equal(t, 6, n)

	assert.Equal(t, "hello world", rec.Body.String())
	assert.Equal(t, 11, rw.Size())
}

func TestRecordResponseWriter_Written(t *testing.T) {
	rw := NewRecordResponseWriter(httptest.NewRecorder())
	assert.False(t, rw.Written())

	rw.Write([]byte("x"))
	assert.True(t, rw.Written())
}

func TestRecordResponseWriter_Size(t *testing.T) {
	t.Run("not written returns 0", func(t *testing.T) {
		rw := NewRecordResponseWriter(httptest.NewRecorder())
		assert.Equal(t, 0, rw.Size())
	})

	t.Run("after write returns correct size", func(t *testing.T) {
		rw := NewRecordResponseWriter(httptest.NewRecorder())
		rw.Write([]byte("abcdef"))
		assert.Equal(t, 6, rw.Size())
	})
}

func TestRecordResponseWriter_Status(t *testing.T) {
	rw := NewRecordResponseWriter(httptest.NewRecorder())
	assert.Equal(t, http.StatusOK, rw.Status())

	rw.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, rw.Status())
}

func TestRecordResponseWriter_Flush(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewRecordResponseWriter(rec)

	rw.Flush()
	assert.True(t, rw.Written())
}

func TestRecordResponseWriter_Hijack(t *testing.T) {
	rw := NewRecordResponseWriter(httptest.NewRecorder())

	// httptest.NewRecorder does not implement Hijacker
	_, _, err := rw.Hijack()
	assert.ErrorIs(t, err, http.ErrNotSupported)
}

func TestRecordResponseWriter_HijackAfterWrite(t *testing.T) {
	rw := NewRecordResponseWriter(httptest.NewRecorder())
	rw.Write([]byte("data"))

	// httptest.NewRecorder still doesn't implement Hijacker
	_, _, err := rw.Hijack()
	assert.ErrorIs(t, err, http.ErrNotSupported)
}

func TestRecordResponseWriter_HijackSuccess(t *testing.T) {
	rw := NewRecordResponseWriter(&hijackableWriter{})
	conn, rw2, err := rw.Hijack()
	assert.NoError(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, rw2)
	assert.True(t, rw.Written())
}

func TestRecordResponseWriter_HijackSuccessAfterWrite(t *testing.T) {
	rw := NewRecordResponseWriter(&hijackableWriter{})
	rw.Write([]byte("data"))

	conn, rw2, err := rw.Hijack()
	assert.NoError(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, rw2)
}

// hijackableWriter implements http.ResponseWriter and http.Hijacker.
type hijackableWriter struct{}

func (h *hijackableWriter) Header() http.Header                          { return http.Header{} }
func (h *hijackableWriter) Write([]byte) (int, error)                    { return 0, nil }
func (h *hijackableWriter) WriteHeader(statusCode int)                   {}
func (h *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) { return nil, nil, nil }

func TestRecordResponseWriter_WriteAfterWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewRecordResponseWriter(rec)
	rw.WriteHeader(http.StatusAccepted)

	n, err := rw.Write([]byte("body"))
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "body", rec.Body.String())
}

func TestRecordResponseWriter_MultipleWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := NewRecordResponseWriter(rec)

	rw.Write([]byte("part1-"))
	rw.Write([]byte("part2-"))
	rw.Write([]byte("part3"))

	assert.Equal(t, "part1-part2-part3", rec.Body.String())
	assert.Equal(t, 17, rw.Size())
}
