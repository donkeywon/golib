package rwwrapper

import (
	"errors"
	"io"
)

type ReaderWrapper struct {
	r io.ReadCloser
}

func NewReaderWrapper(r io.ReadCloser) *ReaderWrapper {
	return &ReaderWrapper{
		r: r,
	}
}

func (irw *ReaderWrapper) Read(p []byte) (int, error) {
	return irw.r.Read(p)
}

func (irw *ReaderWrapper) Close() error {
	return irw.r.Close()
}

type ReaderWrapperr struct {
	r  io.Reader
	rr *ReaderWrapper
}

func NewReaderWrapperr() *ReaderWrapperr {
	return &ReaderWrapperr{}
}

func (rw *ReaderWrapperr) Read(p []byte) (int, error) {
	return rw.r.Read(p)
}

func (rw *ReaderWrapperr) Close() error {
	if rw.r == nil {
		return rw.rr.Close()
	}

	if rc, ok := rw.r.(io.ReadCloser); ok {
		return errors.Join(rc.Close(), rw.rr.Close())
	}
	return rw.rr.Close()
}

func (rw *ReaderWrapperr) SetR(r io.Reader) *ReaderWrapperr {
	rw.r = r
	return rw
}

func (rw *ReaderWrapperr) SetRR(rr *ReaderWrapper) *ReaderWrapperr {
	rw.rr = rr
	return rw
}

func (rw *ReaderWrapperr) Set(r io.Reader, rr *ReaderWrapper) *ReaderWrapperr {
	rw.SetR(r)
	rw.SetRR(rr)
	return rw
}
