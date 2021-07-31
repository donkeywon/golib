package errs

import (
	"fmt"
	"io"
	"strings"

	"github.com/donkeywon/golib/util/bufferpool"
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

type errFmtState struct{ *bufferpool.Buffer }

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
	buf := bufferpool.GetBuffer()
	defer buf.Free()
	ErrToStack(err, buf, 0)
	return buf.String()
}

func ErrToStack(err error, buf *bufferpool.Buffer, errsDepth int) {
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
			ErrToStack(errs[0], buf, errsDepth)
		} else {
			writeIndent(buf, _errsIndent, errsDepth, false, _errsPrefix)
			for _, e := range errs {
				buf.WriteByte('\n')
				writeIndent(buf, _errsIndent, errsDepth+1, false, _errsSeparator)

				_buf := bufferpool.GetBuffer()
				ErrToStack(e, _buf, errsDepth+2)
				writeIndent(buf, _errsIndent, 0, true, strings.TrimLeft(_buf.String(), " \n"))
				_buf.Free()
			}
		}
	case wrappedErr:
		ErrToStack(terr.Unwrap(), buf, errsDepth)

		if emsg, ok := err.(*withMessage); ok {
			buf.WriteByte('\n')
			writeIndent(buf, _errsIndent, errsDepth, false, "cause: ")
			buf.WriteString(emsg.msg)
		} else if estack, ok := err.(*withStack); ok {
			sf := (*estack.stack)[:estack.foldAt]
			stackLen := len(*estack.stack)

			_buf := bufferpool.GetBuffer()
			(&sf).Format(errFmtState{_buf}, 'v')
			writeIndent(buf, _errsIndent, errsDepth, false, _buf.String())
			_buf.Free()

			if estack.foldAt < stackLen {
				buf.WriteByte('\n')
				writeIndent(buf, _errsIndent, errsDepth, false, fmt.Sprintf("\t... %d more", stackLen-estack.foldAt))
			}
		} else {
			buf.WriteByte('\n')
			writeIndent(buf, _errsIndent, errsDepth, false, "cause: ")
			buf.WriteString(err.Error())
		}
	case fmt.Formatter:
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		_buf := bufferpool.GetBuffer()
		terr.Format(errFmtState{_buf}, 'v')
		writeIndent(buf, _errsIndent, errsDepth, false, strings.TrimLeft(_buf.String(), " \n"))
		_buf.Free()
	default:
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(terr.Error())
	}
}

func PanicToErr(p interface{}) error {
	return PanicToErrWithMsg(p, "panic")
}

func PanicToErrWithMsg(p interface{}, msg string) error {
	var err error
	switch pt := p.(type) {
	case error:
		err = pt
	default:
		if msg == "" {
			err = Errorf("%+v", p)
		} else {
			err = Errorf("%s: %+v", msg, p)
		}
	}
	return err
}
