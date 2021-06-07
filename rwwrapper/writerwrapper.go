package rwwrapper

import (
	"errors"
	"io"
	"sync/atomic"
)

type flusher interface {
	Flush() error
}

type WriterWrapper struct {
	w  io.WriteCloser
	nw uint64
}

func NewWriterWrapper(w io.WriteCloser) *WriterWrapper {
	return &WriterWrapper{
		w: w,
	}
}

func (i *WriterWrapper) Write(p []byte) (int, error) {
	nw, err := i.w.Write(p)
	atomic.AddUint64(&i.nw, uint64(nw))
	return nw, err
}

func (i *WriterWrapper) Flush() error {
	if f, ok := i.w.(flusher); ok {
		return f.Flush()
	}
	return nil
}

func (i *WriterWrapper) Close() error {
	return i.w.Close()
}

func (i *WriterWrapper) Nwrite() uint64 {
	return atomic.LoadUint64(&i.nw)
}

// 比如调用gzip.Writer.Close并不会调用gzip.Writer.w的Close方法，这里手动包一层writer
// 例如使用gzip写到file，下面几个writer的关系是
//
//	WriterWrapperr {
//	        w: gzip.Writer {
//	                w: *WriterWrapper {
//	                        w: *os.File
//	                }
//	        }
//	        ww: *WriterWrapper {
//	                w: *os.File
//	        }
//	}
type WriterWrapperr struct {
	w  io.WriteCloser
	ww *WriterWrapper
	nw uint64
}

func NewWriterWrapperr() *WriterWrapperr {
	return &WriterWrapperr{}
}

func (w *WriterWrapperr) Write(p []byte) (n int, err error) {
	n, err = w.w.Write(p)
	atomic.AddUint64(&w.nw, uint64(n))
	return
}

func (w *WriterWrapperr) Flush() error {
	return w.ww.Flush()
}

func (w *WriterWrapperr) Close() error {
	if w.w == nil {
		return w.ww.Close()
	}
	return errors.Join(w.w.Close(), w.ww.Close())
}

func (w *WriterWrapperr) SetW(wc io.WriteCloser) *WriterWrapperr {
	w.w = wc
	return w
}

func (w *WriterWrapperr) SetWW(ww *WriterWrapper) *WriterWrapperr {
	w.ww = ww
	return w
}

func (w *WriterWrapperr) Set(wc io.WriteCloser, ww *WriterWrapper) *WriterWrapperr {
	w.SetW(wc)
	w.SetWW(ww)
	return w
}

func (w *WriterWrapperr) Nwrite() uint64 {
	if w == nil || w.ww == nil {
		return 0
	}
	return w.ww.Nwrite()
}
