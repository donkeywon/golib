package tail

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tail.log")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// readWithTimeout runs Read in a goroutine and fails the test if it does not
// return within the timeout (e.g. when an fsnotify event is expected).
func readWithTimeout(t *testing.T, r *Reader, buf []byte, timeout time.Duration) (int, error) {
	t.Helper()
	type result struct {
		n   int
		err error
	}
	ch := make(chan result, 1)
	go func() {
		n, err := r.Read(buf)
		ch <- result{n, err}
	}()
	select {
	case res := <-ch:
		return res.n, res.err
	case <-time.After(timeout):
		_ = r.Close() // unblock the reader goroutine
		t.Fatalf("read timed out after %s", timeout)
		return 0, nil
	}
}

func TestNewReader(t *testing.T) {
	path := newTestFile(t, "hello\n")
	r, err := NewReader(path, 0)
	require.NoError(t, err)
	require.NoError(t, r.Close())

	assert.Equal(t, int64(-1), r.Len())
	assert.NotNil(t, r.File())
}

func TestRead_AppendData(t *testing.T) {
	path := newTestFile(t, "hello\n")
	r, err := NewReader(path, 0)
	require.NoError(t, err)
	defer r.Close()

	buf := make([]byte, 64)
	n, err := r.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", string(buf[:n]))

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	require.NoError(t, err)
	_, err = f.WriteString("world\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	n, err = readWithTimeout(t, r, buf, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "world\n", string(buf[:n]))
	assert.Equal(t, int64(12), r.Offset())
}

func TestRead_FromOffset(t *testing.T) {
	path := newTestFile(t, "hello\nworld\n")
	r, err := NewReader(path, 6)
	require.NoError(t, err)
	defer r.Close()

	buf := make([]byte, 64)
	n, err := r.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "world\n", string(buf[:n]))
	assert.Equal(t, int64(12), r.Offset())
}

func TestTruncate_RestartsFromBeginning(t *testing.T) {
	path := newTestFile(t, "hello\nworld\n")
	r, err := NewReader(path, 0)
	require.NoError(t, err)
	defer r.Close()

	buf := make([]byte, 64)
	n, err := r.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "hello\nworld\n", string(buf[:n]))
	assert.Equal(t, int64(12), r.Offset())

	// os.WriteFile opens with O_TRUNC, simulating logrotate copytruncate:
	// the file is cleared and new content starts from offset 0.
	require.NoError(t, os.WriteFile(path, []byte("new data\n"), 0o644))

	n, err = readWithTimeout(t, r, buf, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "new data\n", string(buf[:n]))
	assert.Equal(t, int64(9), r.Offset())
}

func TestClose_Twice(t *testing.T) {
	path := newTestFile(t, "hello\n")
	r, err := NewReader(path, 0)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	require.NoError(t, r.Close()) // must be idempotent, must not panic
}

func TestRead_AfterClose(t *testing.T) {
	path := newTestFile(t, "hello\n")
	r, err := NewReader(path, 0)
	require.NoError(t, err)
	require.NoError(t, r.Close())

	buf := make([]byte, 64)
	n, err := r.Read(buf)
	assert.Zero(t, n)
	assert.ErrorIs(t, err, io.EOF)
}

func TestClose_WhileReadBlocked(t *testing.T) {
	path := newTestFile(t, "")
	r, err := NewReader(path, 0)
	require.NoError(t, err)

	type result struct {
		n   int
		err error
	}
	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, 64)
		n, err := r.Read(buf)
		ch <- result{n, err}
	}()

	// Give the reader goroutine time to block in wait().
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, r.Close())

	select {
	case res := <-ch:
		assert.Zero(t, res.n)
		assert.ErrorIs(t, res.err, io.EOF)
	case <-time.After(5 * time.Second):
		t.Fatal("read did not return after close")
	}
}

func TestFileRemoved(t *testing.T) {
	path := newTestFile(t, "")
	r, err := NewReader(path, 0)
	require.NoError(t, err)
	defer r.Close()

	require.NoError(t, os.Remove(path))

	buf := make([]byte, 64)
	_, err = readWithTimeout(t, r, buf, 5*time.Second)
	assert.ErrorIs(t, err, ErrFileRemoved)
}

func TestFileRenamed(t *testing.T) {
	path := newTestFile(t, "")
	r, err := NewReader(path, 0)
	require.NoError(t, err)
	defer r.Close()

	newPath := path + ".1"
	require.NoError(t, os.Rename(path, newPath))
	t.Cleanup(func() { _ = os.Remove(newPath) })

	buf := make([]byte, 64)
	_, err = readWithTimeout(t, r, buf, 5*time.Second)
	assert.ErrorIs(t, err, ErrFileRenamed)
}
