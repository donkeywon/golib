package errs

import (
	"fmt"
	"io"
	"strings"
	"unsafe"
)

var (
	_errsPrefix    = "multi error occurred:"
	_errsSeparator = "- "
	_errsIndent    = []byte("  ")
)

type wrappedErr interface {
	Unwrap() error
}

type wrappedErrs interface {
	Unwrap() []error
}

type anotherWrappedErrs interface {
	WrappedErrors() []error
}

type stackTracer interface {
	StackTrace() StackTrace
}

type errFmtState struct{ *buffer }

var _ fmt.State = errFmtState{}

func (errFmtState) Flag(c int) bool {
	switch c {
	case '+':
		return true
	default:
		return false
	}
}

func (errFmtState) Width() (wid int, ok bool)      { panic("should not be called") }
func (errFmtState) Precision() (prec int, ok bool) { panic("should not be called") }

var newLineBytes = []byte{'\n'}

func writeIndent(w io.Writer, indent []byte, indentCount int, skipFirst bool, s string) {
	first := true
	for len(s) > 0 {
		if first && skipFirst {
			first = false
		} else {
			for i := 0; i < indentCount; i++ {
				w.Write(indent)
			}
		}

		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			idx = len(s) - 1
		}

		io.WriteString(w, s[:idx+1])
		s = s[idx+1:]
	}
}

func ErrToStackString(err error) string {
	if err == nil {
		return ""
	}
	buf := getBuffer()
	defer buf.free()
	ErrToStack(err, buf, 0)
	return buf.String()
}

func ErrToStack(err error, w io.Writer, errsDepth int) {
	if err == nil {
		return
	}
	switch terr := err.(type) {
	case wrappedErrs, anotherWrappedErrs:
		var errs []error
		if tterr, ok := terr.(wrappedErrs); ok {
			errs = tterr.Unwrap()
		} else {
			errs = terr.(anotherWrappedErrs).WrappedErrors()
		}
		if len(errs) < 1 {
			return
		} else if len(errs) == 1 {
			ErrToStack(errs[0], w, errsDepth)
		} else {
			writeIndent(w, _errsIndent, errsDepth, false, _errsPrefix)
			for _, e := range errs {
				w.Write(newLineBytes)
				writeIndent(w, _errsIndent, errsDepth+1, false, _errsSeparator)

				_buf := getBuffer()
				ErrToStack(e, _buf, errsDepth+2)
				writeIndent(w, _errsIndent, 0, true, strings.TrimLeft(_buf.String(), " \n"))
				_buf.free()
			}
		}
	case wrappedErr:
		ErrToStack(terr.Unwrap(), w, errsDepth)

		if emsg, ok := err.(*withMessage); ok {
			w.Write(newLineBytes)
			writeIndent(w, _errsIndent, errsDepth, false, "cause: ")
			w.Write(string2Bytes(emsg.msg))
		} else if estack, ok := err.(*withStack); ok {
			sf := (*estack.stack)[:estack.foldAt]
			stackLen := len(*estack.stack)

			_buf := getBuffer()
			(&sf).Format(errFmtState{_buf}, 'v')
			writeIndent(w, _errsIndent, errsDepth, false, _buf.String())
			_buf.free()

			if estack.foldAt < stackLen {
				w.Write(newLineBytes)
				writeIndent(w, _errsIndent, errsDepth, false, fmt.Sprintf("\t... %d more", stackLen-estack.foldAt))
			}
		} else {
			w.Write(newLineBytes)
			writeIndent(w, _errsIndent, errsDepth, false, "cause: ")
			w.Write(string2Bytes(err.Error()))
		}
	case fmt.Formatter:
		_buf := getBuffer()
		terr.Format(errFmtState{_buf}, 'v')
		writeIndent(w, _errsIndent, errsDepth, false, strings.TrimLeft(bytes2String(_buf.Bytes()), " \n"))
		_buf.free()
	default:
		w.Write(string2Bytes(terr.Error()))
	}
}

func PanicToErr(p interface{}) error {
	return PanicToErrWithMsg(p, "panic")
}

func PanicToErrWithMsg(p interface{}, msg string) error {
	var err error
	switch pt := p.(type) {
	case error:
		err = Wrap(pt, msg)
	default:
		if msg == "" {
			err = Errorf("%+v", p)
		} else {
			err = Errorf("%s: %+v", msg, p)
		}
	}
	return err
}

// for zero dep.
func bytes2String(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}

func string2Bytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}
