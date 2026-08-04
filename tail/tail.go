package tail

import (
	"errors"
	"io"
	"os"
	"path/filepath"
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
	file          *os.File
	watcher       *fsnotify.Watcher
	closed        chan struct{}
	filepath      string
	offset        int64
	closeOnceFunc func() error
}

func NewReader(path string, offset int64) (*Reader, error) {
	var err error

	r := &Reader{
		filepath: path,
		offset:   offset,
		closed:   make(chan struct{}),
	}

	r.closeOnceFunc = sync.OnceValue(func() error {
		allErr := make([]error, 0, 2)
		close(r.closed)
		if r.file != nil {
			allErr = append(allErr, r.file.Close())
		}
		if r.watcher != nil {
			allErr = append(allErr, r.watcher.Close())
		}
		return errors.Join(allErr...)
	})

	r.file, err = os.Open(path)
	if err != nil {
		return nil, err
	}

	if r.offset > 0 {
		_, err = r.file.Seek(r.offset, io.SeekStart)
		if err != nil {
			return nil, r.closeWithErr(errs.Wrap(err, "file seek failed"))
		}
	}

	r.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, r.closeWithErr(errs.Wrap(err, "create notify watcher failed"))
	}
	err = r.watcher.Add(path)
	if err != nil {
		return nil, r.closeWithErr(errs.Wrapf(err, "watch failed: %s", path))
	}
	// Watch the parent directory as well: on some kernels deleting a file
	// that is still open only emits IN_ATTRIB on the file's own watch, so
	// remove/rename detection must come from directory events.
	err = r.watcher.Add(filepath.Dir(path))
	if err != nil {
		return nil, r.closeWithErr(errs.Wrapf(err, "watch dir failed: %s", filepath.Dir(path)))
	}

	return r, nil
}

func (r *Reader) Read(p []byte) (nr int, err error) {
	for {
		nr, err = r.read(p)
		if err != nil {
			if errors.Is(err, errTailClosed) {
				return 0, io.EOF
			}
			return
		}
		if nr > 0 {
			return nr, nil
		}

		err = r.wait()
		if err == nil {
			err = r.resetOffsetIfTruncated()
			if err != nil {
				return 0, err
			}
			continue
		}
		if errors.Is(err, errTailClosed) {
			return 0, io.EOF
		}
		return 0, err
	}
}

func (r *Reader) resetOffsetIfTruncated() error {
	fi, err := r.file.Stat()
	if err != nil {
		return err
	}
	if fi.Size() >= atomic.LoadInt64(&r.offset) {
		return nil
	}
	_, err = r.file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	atomic.StoreInt64(&r.offset, 0)
	return nil
}

func (r *Reader) read(p []byte) (int, error) {
	nr, err := r.file.Read(p)
	atomic.AddInt64(&r.offset, int64(nr))
	if err == nil || err == io.EOF {
		return nr, nil
	}
	if errors.Is(err, os.ErrClosed) {
		return 0, errTailClosed
	}

	return nr, err
}

func (r *Reader) Close() error {
	return r.closeOnceFunc()
}

func (r *Reader) closeWithErr(err error) error {
	return errors.Join(r.Close(), err)
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

func (r *Reader) wait() error {
	for {
		select {
		case <-r.closed:
			return errTailClosed
		case e, ok := <-r.watcher.Events:
			if !ok {
				return errTailClosed
			}
			// The directory watch reports events for every file in the
			// directory; only the watched file matters.
			if e.Name != r.filepath {
				continue
			}
			if e.Has(fsnotify.Remove) {
				return ErrFileRemoved
			}
			if e.Has(fsnotify.Rename) {
				return ErrFileRenamed
			}
			if e.Has(fsnotify.Write) {
				return nil
			}
		case err, ok := <-r.watcher.Errors:
			if !ok {
				return errTailClosed
			}
			return errs.Wrap(err, "watcher error occurred")
		}
	}
}
