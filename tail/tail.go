package tail

import (
	"errors"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/donkeywon/golib/errs"
	"github.com/fsnotify/fsnotify"
)

var (
	errTailClosed = errors.New("tail closed")

	ErrFileRemoved = errors.New("file removed")
	ErrFileRenamed = errors.New("file renamed")
)

type Reader struct {
	fi       os.FileInfo
	file     *os.File
	watcher  *fsnotify.Watcher
	closed   chan struct{}
	filepath string
	offset   int64
	once     sync.Once
}

func NewReader(filepath string, offset int64) (*Reader, error) {
	var err error

	r := &Reader{
		filepath: filepath,
		offset:   offset,
		closed:   make(chan struct{}),
	}

	r.file, err = os.Open(filepath)
	if err != nil {
		return nil, err
	}

	if r.offset > 0 {
		_, err = r.file.Seek(r.offset, io.SeekStart)
		if err != nil {
			return nil, r.close(errs.Wrap(err, "file seek failed"))
		}
	}

	r.fi, err = r.file.Stat()
	if err != nil {
		return nil, r.close(errs.Wrap(err, "get file stat failed"))
	}

	r.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, r.close(errs.Wrap(err, "create notify watcher failed"))
	}
	_ = r.watcher.Add(filepath)

	return r, nil
}

func (r *Reader) Read(p []byte) (int, error) {
	nr, err := r.read(p)
	if err != nil {
		return nr, err
	}

	if nr > 0 {
		return nr, nil
	}

	err = r.wait()
	switch {
	case errors.Is(err, errTailClosed):
		return 0, io.EOF
	case err == nil:
		return r.read(p)
	default:
		return 0, err
	}
}

func (r *Reader) read(p []byte) (int, error) {
	nr, err := r.file.Read(p)
	atomic.AddInt64(&r.offset, int64(nr))
	if err == nil || errors.Is(err, io.EOF) {
		return nr, nil
	}

	return nr, err
}

func (r *Reader) Close() error {
	return r.close(nil)
}

func (r *Reader) Offset() int64 {
	return atomic.LoadInt64(&r.offset)
}

func (r *Reader) Len() int64 {
	// file size is growing
	return -1
}

func (r *Reader) File() *os.File {
	return r.file
}

func (r *Reader) FileInfo() os.FileInfo {
	return r.fi
}

func (r *Reader) close(err error) error {
	r.once.Do(func() {
		close(r.closed)
		if r.file != nil {
			err = errors.Join(err, r.file.Close())
		}
		if r.watcher != nil {
			err = errors.Join(err, r.watcher.Close())
		}
	})
	return err
}

func (r *Reader) wait() error {
	select {
	case <-r.closed:
		return errTailClosed
	case e := <-r.watcher.Events:
		switch e.Op {
		case fsnotify.Remove:
			return ErrFileRemoved
		case fsnotify.Rename:
			return ErrFileRenamed
		default:
			return nil
		}
	case err := <-r.watcher.Errors:
		return errs.Wrap(err, "watcher error occurred")
	}
}
