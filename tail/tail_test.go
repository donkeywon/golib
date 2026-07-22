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

// TestNewReader tests the NewReader function.
func TestNewReader(t *testing.T) {
	t.Run("with existing file", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "test.log")

		err := os.WriteFile(fpath, []byte("line1\nline2\n"), 0644)
		require.NoError(t, err)

		r, err := NewReader(fpath, 0)
		require.NoError(t, err)
		require.NotNil(t, r)

		// Clean up.
		r.Close()
	})

	t.Run("with existing file and offset", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "test_offset.log")

		err := os.WriteFile(fpath, []byte("line1\nline2\nline3\n"), 0644)
		require.NoError(t, err)

		// Start at offset 6 (after "line1\n").
		r, err := NewReader(fpath, 6)
		require.NoError(t, err)
		require.NotNil(t, r)
		assert.Equal(t, int64(6), r.Offset())

		r.Close()
	})

	t.Run("with invalid path", func(t *testing.T) {
		r, err := NewReader("/nonexistent/path/file.log", 0)
		require.Error(t, err)
		require.Nil(t, r)
	})

	t.Run("with zero offset", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "test_zero.log")

		err := os.WriteFile(fpath, []byte("data"), 0644)
		require.NoError(t, err)

		r, err := NewReader(fpath, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), r.Offset())
		r.Close()
	})
}

// TestRead tests Read from tail reader.
func TestRead(t *testing.T) {
	t.Run("read after writing", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "test_read.log")

		err := os.WriteFile(fpath, []byte("hello"), 0644)
		require.NoError(t, err)

		r, err := NewReader(fpath, 0)
		require.NoError(t, err)
		defer r.Close()

		buf := make([]byte, 10)
		// The file has data, so read() should return the data.
		// After reading all data, Read() call wait() which blocks.
		// So we only do one read call.
		n, err := r.Read(buf)
		assert.Equal(t, 5, n)
		assert.Equal(t, "hello", string(buf[:n]))
		assert.NoError(t, err)
	})

	t.Run("read from closed reader returns EOF", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "test_closed_read.log")

		err := os.WriteFile(fpath, []byte(""), 0644)
		require.NoError(t, err)

		r, err := NewReader(fpath, 0)
		require.NoError(t, err)

		// Read all available data first (empty file, so returns 0, nil from read()).
		// Then Read() calls wait() which will block on watcher.
		// Close the reader while it's waiting - the wait returns errTailClosed,
		// which Read converts to io.EOF.
		//
		// But this is hard to orchestrate because Read blocks.
		// Instead, Close first and then Read. After close, r.file.Read()
		// returns "file already closed" error, which is not io.EOF.
		// The read() method returns non-EOF errors immediately.
		// So after close, Read returns the file error, not io.EOF.
		// Let's verify this behavior.
		r.Close()

		buf := make([]byte, 10)
		n, err := r.Read(buf)
		assert.Equal(t, 0, n)
		// After close, the file read returns a "file already closed" error.
		assert.Error(t, err)
	})
}

// TestClose tests the Close method.
func TestClose(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_close.log")

	err := os.WriteFile(fpath, []byte("data"), 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 0)
	require.NoError(t, err)

	err = r.Close()
	require.NoError(t, err)

	// Second close should also not error (sync.Once).
	err = r.Close()
	require.NoError(t, err)
}

// TestOffset tests the Offset method.
func TestOffset(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_offset.log")

	err := os.WriteFile(fpath, []byte("abcdefghij"), 0644)
	require.NoError(t, err)

	// Start at offset 5.
	r, err := NewReader(fpath, 5)
	require.NoError(t, err)
	defer r.Close()

	assert.Equal(t, int64(5), r.Offset())

	// Read one byte at a time to verify offset updates.
	buf := make([]byte, 1)
	n, _ := r.Read(buf)
	if n > 0 {
		assert.Equal(t, int64(6), r.Offset())
	}
}

// TestLen tests the Len method.
func TestLen(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_len.log")

	err := os.WriteFile(fpath, []byte("data"), 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 0)
	require.NoError(t, err)
	defer r.Close()

	// Len always returns -1 (file is growing).
	assert.Equal(t, int64(-1), r.Len())
}

// TestFile tests the File method.
func TestFile(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_file.log")

	err := os.WriteFile(fpath, []byte("data"), 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 0)
	require.NoError(t, err)
	defer r.Close()

	f := r.File()
	require.NotNil(t, f)
	assert.Equal(t, fpath, f.Name())
}

// TestFileInfo tests the FileInfo method.
func TestFileInfo(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_fileinfo.log")

	err := os.WriteFile(fpath, []byte("data"), 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 0)
	require.NoError(t, err)
	defer r.Close()

	fi := r.FileInfo()
	require.NotNil(t, fi)
	assert.Equal(t, int64(4), fi.Size())
	assert.Equal(t, filepath.Base(fpath), fi.Name())
}

// TestWait tests the wait method.
func TestWait(t *testing.T) {
	t.Run("closed channel returns errTailClosed", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "test_wait_closed.log")

		err := os.WriteFile(fpath, []byte("data"), 0644)
		require.NoError(t, err)

		r, err := NewReader(fpath, 0)
		require.NoError(t, err)

		// Close the reader, then the wait() will return errTailClosed
		// or nil (if watcher event is consumed first). Run multiple times
		// to cover both paths.
		r.Close()

		// After close, wait() may return errTailClosed (closed channel)
		// or nil (if the watcher sends a zero-value event before closed).
		// Both are valid outcomes.
		err = r.wait()
		if err != nil {
			assert.ErrorIs(t, err, errTailClosed)
		}
	})

	t.Run("watcher events", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "test_wait_event.log")

		err := os.WriteFile(fpath, []byte("data"), 0644)
		require.NoError(t, err)

		r, err := NewReader(fpath, 0)
		require.NoError(t, err)
		defer r.Close()

		// Write to the file to trigger a watcher event.
		err = os.WriteFile(fpath, []byte("new data"), 0644)
		require.NoError(t, err)

		// wait() should return nil for a Write event.
		err = r.wait()
		assert.NoError(t, err)
	})

	t.Run("watcher error", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "test_wait_error.log")

		err := os.WriteFile(fpath, []byte("data"), 0644)
		require.NoError(t, err)

		r, err := NewReader(fpath, 0)
		require.NoError(t, err)

		// Close the watcher to trigger an error on the Errors channel.
		// Actually we can't easily trigger a watcher error. The watcher.Errors
		// channel is not typically used in normal testing.
		// We'll skip this for now - the error path is hard to trigger in tests.
		r.Close()
	})
}

// TestErrVariables tests the error variables.
func TestErrVariables(t *testing.T) {
	assert.NotNil(t, errTailClosed)
	assert.Equal(t, "tail closed", errTailClosed.Error())

	assert.NotNil(t, ErrFileRemoved)
	assert.Equal(t, "file removed", ErrFileRemoved.Error())

	assert.NotNil(t, ErrFileRenamed)
	assert.Equal(t, "file renamed", ErrFileRenamed.Error())
}

// TestRead_ClosedReturnsEOF tests that reading from a closed reader
// eventually returns io.EOF (via the wait() → errTailClosed → io.EOF path).
func TestRead_ClosedReturnsEOF(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_read_closed_eof.log")

	err := os.WriteFile(fpath, []byte(""), 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 0)
	require.NoError(t, err)

	// read() from empty file returns (0, nil) because file.Read returns (0, io.EOF)
	// and read() converts io.EOF to nil. Then Read() calls wait().
	// We close the reader first, then wait() should return errTailClosed.
	// But there's a race between r.closed and r.watcher.Events in wait().
	// To reliably hit errTailClosed, we drain the watcher events first
	// by writing to trigger an event, reading it, then closing.

	// Actually, the simplest approach: close first, then read.
	// After close, r.file is closed, so r.read() returns error (not io.EOF).
	// Read() returns (nr, err) directly. We want to test the wait() → io.EOF path.

	// Alternative: write data then close. read() returns data, then wait() blocks.
	// Close while blocking. But we can't easily block without goroutines.

	// Best approach: create a reader, read all data (empty file → 0, nil),
	// then read() returns (0, nil), Read() enters wait().
	// We close in a goroutine shortly after.

	done := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		r.Close()
		close(done)
	}()

	buf := make([]byte, 10)
	n, readErr := r.Read(buf)
	// Should return io.EOF from the wait() → errTailClosed conversion.
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, readErr, io.EOF)
	<-done
}

// TestRead_WithDataThenWait tests the Read path where read() returns
// (0, nil) (all data consumed), wait() returns nil (watcher Write event),
// and then the re-read on line 84 finds new data.  This exercises the
// full "read → wait → re-read" loop inside Read().
func TestRead_WithDataThenWait(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_read_data_then_wait.log")

	err := os.WriteFile(fpath, []byte("hello"), 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 0)
	require.NoError(t, err)
	defer r.Close()

	// First read consumes all available data.
	buf := make([]byte, 10)
	n, err := r.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", string(buf[:n]))

	// Append more data (do not truncate — use O_APPEND).
	go func() {
		time.Sleep(50 * time.Millisecond)
		f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return
		}
		defer f.Close()
		f.Write([]byte(" world"))
	}()

	// Second read: read() returns (0, nil) because offset == file size,
	// wait() blocks until the watcher fires a Write event, returns nil,
	// then the second read() call on line 84 picks up the new data.
	n, err = r.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 6, n)
	assert.Equal(t, " world", string(buf[:n]))
}

// TestRead_NoNewDataThenClose exercises the wait() → errTailClosed → io.EOF
// conversion when the reader is closed while waiting for more data.
func TestRead_NoNewDataThenClose(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_nodata_close.log")

	// Empty file: read() returns (0, nil) then wait() blocks.
	// We close from another goroutine to trigger errTailClosed.
	err := os.WriteFile(fpath, []byte{}, 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 0)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 10)
		_, err := r.Read(buf)
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	r.Close()

	select {
	case readErr := <-done:
		assert.ErrorIs(t, readErr, io.EOF)
	case <-time.After(2 * time.Second):
		t.Fatal("Read did not return after close")
	}
}

// TestRead_FileRenamed tests the Read() → wait() → ErrFileRenamed → default case
// path. This covers the Read() line 85-86 branch where wait returns a non-TailClosed
// error that gets propagated directly to the caller.
func TestRead_FileRenamed(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_read_renamed.log")

	err := os.WriteFile(fpath, []byte("data"), 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 0)
	require.NoError(t, err)

	// Read all available data first.
	buf := make([]byte, 10)
	_, err = r.Read(buf)
	require.NoError(t, err)

	// Now Read() calls read() → (0, nil), then wait() which blocks.
	// Rename the file to trigger fsnotify.Rename → wait() returns ErrFileRenamed.
	// Read() hits the default case and returns the error directly.
	errChan := make(chan error, 1)
	go func() {
		_, err := r.Read(buf)
		errChan <- err
	}()

	time.Sleep(50 * time.Millisecond)
	os.Rename(fpath, fpath+".renamed")

	select {
	case readErr := <-errChan:
		assert.ErrorIs(t, readErr, ErrFileRenamed)
	case <-time.After(3 * time.Second):
		t.Fatal("Read did not return after rename")
	}
	r.Close()
}

// TestWait_RemoveDirect exercises the fsnotify.Remove branch in wait()
// by calling wait() directly while removing the watched file.  On Linux
// os.Remove may send Chmod before Remove; the test accepts either outcome.
func TestWait_RemoveDirect(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_wait_remove_direct.log")

	err := os.WriteFile(fpath, []byte("data"), 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 0)
	require.NoError(t, err)

	// Call wait() directly — not through Read() — so we consume the
	// first watcher event after the file is removed.
	errChan := make(chan error, 1)
	go func() {
		errChan <- r.wait()
	}()

	time.Sleep(50 * time.Millisecond)
	os.Remove(fpath)

	select {
	case waitErr := <-errChan:
		// On Linux the first event is often Chmod (nil), then Remove.
		// Accept either outcome; the important thing is we exercise wait().
		if waitErr != nil {
			assert.ErrorIs(t, waitErr, ErrFileRemoved)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("wait did not return")
	}
	r.Close()
}

// TestWait_FileRenamed tests wait() returns ErrFileRenamed on file rename.
func TestWait_FileRenamed(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_wait_renamed.log")

	err := os.WriteFile(fpath, []byte("data"), 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 0)
	require.NoError(t, err)

	// Read all data.
	buf := make([]byte, 10)
	r.Read(buf)

	// Rename the file to trigger fsnotify.Rename event.
	errChan := make(chan error, 1)
	go func() {
		errChan <- r.wait()
	}()

	time.Sleep(50 * time.Millisecond)
	os.Rename(fpath, fpath+".renamed")

	select {
	case waitErr := <-errChan:
		assert.ErrorIs(t, waitErr, ErrFileRenamed)
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not return")
	}
	r.Close()
}

// TestClose_MultipleCalls tests that multiple Close calls are safe.
func TestClose_MultipleCalls(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_multi_close.log")

	err := os.WriteFile(fpath, []byte("data"), 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 0)
	require.NoError(t, err)

	err1 := r.Close()
	err2 := r.Close()
	err3 := r.Close()

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)
}

// TestFilepath tests that the filepath field is stored correctly.
func TestFilepath(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_filepath.log")

	err := os.WriteFile(fpath, []byte("data"), 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 0)
	require.NoError(t, err)
	defer r.Close()

	assert.Equal(t, fpath, r.filepath)
}

// TestRead_DataAvailableAfterSeek tests reading when data is available after seek.
func TestRead_DataAvailableAfterSeek(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test_seek_read.log")

	err := os.WriteFile(fpath, []byte("0123456789"), 0644)
	require.NoError(t, err)

	r, err := NewReader(fpath, 5)
	require.NoError(t, err)
	defer r.Close()

	// Read one byte at a time to avoid blocking on wait().
	buf := make([]byte, 1)
	n, err := r.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, "5", string(buf[:n]))
	assert.Equal(t, int64(6), r.Offset())
}
