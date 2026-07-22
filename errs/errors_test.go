package errs

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func nestErr(i, max int) error {
	if i == max {
		return fmt.Errorf("err%d", i)
	}
	return Wrapf(nestErr(i+1, max), "err%d", i)
}

func errS() error {
	e1 := nestErr(0, 1)
	e2 := nestErr(0, 2)
	return Wrap(errors.Join(e1, e2), "errS")
}

func TestErr(t *testing.T) {
	err := errS()
	fmt.Printf("%+v", err)
}

func TestFormat(t *testing.T) {
	buf := getBuffer()
	defer buf.free()
	e := nestErr(0, 3)
	ErrToStack(e, buf, 0)
	t.Log(buf.String())
}

func TestWithStack_NilError(t *testing.T) {
	require.Nil(t, WithStack(nil))
}

func TestWrap_NilError(t *testing.T) {
	require.Nil(t, Wrap(nil, "msg"))
}

func TestWrapf_NilError(t *testing.T) {
	require.Nil(t, Wrapf(nil, "msg%d", 1))
}

func TestWithMessage_NilError(t *testing.T) {
	require.Nil(t, WithMessage(nil, "msg"))
}

func TestWithMessagef_NilError(t *testing.T) {
	require.Nil(t, WithMessagef(nil, "msg%d", 1))
}

func TestErrorf(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []any
		want   string
	}{
		{
			name:   "plain string",
			format: "test error",
			args:   nil,
			want:   "test error",
		},
		{
			name:   "formatted",
			format: "error code %d: %s",
			args:   []any{42, "not found"},
			want:   "error code 42: not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Errorf(tt.format, tt.args...)
			require.Equal(t, tt.want, err.Error())
		})
	}
}

func TestUnwrapChain(t *testing.T) {
	base := New("base error")
	wrapped := Wrap(base, "wrapped")
	msg := WithMessage(wrapped, "with message")

	// Unwrap through withMessage
	require.Equal(t, wrapped, errors.Unwrap(msg))
	// Unwrap through withStack (Wrap produces withMessage + withStack, so first Unwrap gives withMessage)
	unwrapped := errors.Unwrap(wrapped)
	require.NotNil(t, unwrapped)
	// Unwrap again to reach base
	unwrapped2 := errors.Unwrap(unwrapped)
	require.Equal(t, base, unwrapped2)
	// Check error messages
	require.Contains(t, msg.Error(), "with message")
	require.Contains(t, wrapped.Error(), "wrapped")
	require.Equal(t, "base error", base.Error())
}

func TestFundamental_FormatVerbs(t *testing.T) {
	f := New("test message").(*fundamental)

	tests := []struct {
		name string
		verb rune
		plus bool
		want string
	}{
		{
			name: "%s",
			verb: 's',
			want: "test message",
		},
		{
			name: "%q",
			verb: 'q',
			want: `"test message"`,
		},
		{
			name: "%v",
			verb: 'v',
			want: "test message",
		},
		{
			name: "%+v",
			verb: 'v',
			plus: true,
			want: "test message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fmt.Sprintf(formatVerb(tt.verb, tt.plus), f)
			if tt.plus {
				// %+v includes stack trace, just check it starts with the message
				require.True(t, len(result) > len("test message"))
			} else {
				require.Equal(t, tt.want, result)
			}
		})
	}
}

func TestWithStack_FormatVerbs(t *testing.T) {
	base := New("base error")
	ws := WithStack(base)

	tests := []struct {
		name     string
		format   string
		contains string
	}{
		{
			name:     "%s",
			format:   "%s",
			contains: "base error",
		},
		{
			name:     "%q",
			format:   "%q",
			contains: `"base error"`,
		},
		{
			name:     "%v",
			format:   "%v",
			contains: "base error",
		},
		{
			name:     "%+v",
			format:   "%+v",
			contains: "base error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fmt.Sprintf(tt.format, ws)
			require.Contains(t, result, tt.contains)
		})
	}
}

func TestWithMessage_Error(t *testing.T) {
	base := New("base error")
	wm := WithMessage(base, "wrapper msg")

	require.Equal(t, "wrapper msg: base error", wm.Error())
}

func TestWithMessage_Unwrap(t *testing.T) {
	base := New("base error")
	wm := WithMessage(base, "wrapper msg")

	require.Equal(t, base, errors.Unwrap(wm))
}

func TestWithMessage_LogValue(t *testing.T) {
	base := New("base error")
	wm := WithMessage(base, "wrapper msg")

	v := wm.(*withMessage).LogValue()
	require.Equal(t, slog.KindString, v.Kind())
	require.NotEmpty(t, v.String())
}

func TestWithMessage_FormatVerbs(t *testing.T) {
	base := New("base error")
	wm := WithMessage(base, "wrapper msg")

	tests := []struct {
		name     string
		format   string
		contains string
	}{
		{
			name:     "%s",
			format:   "%s",
			contains: "wrapper msg: base error",
		},
		{
			name:     "%q",
			format:   "%q",
			contains: "wrapper msg: base error",
		},
		{
			name:     "%v",
			format:   "%v",
			contains: "wrapper msg: base error",
		},
		{
			name:     "%+v",
			format:   "%+v",
			contains: "wrapper msg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fmt.Sprintf(tt.format, wm)
			require.Contains(t, result, tt.contains)
		})
	}
}

func TestWriteIndent(t *testing.T) {
	tests := []struct {
		name        string
		indent      []byte
		indentCount int
		skipFirst   bool
		input       string
		want        string
	}{
		{
			name:        "no newlines",
			indent:      []byte("  "),
			indentCount: 1,
			skipFirst:   false,
			input:       "hello",
			want:        "  hello",
		},
		{
			name:        "with newlines",
			indent:      []byte(">>"),
			indentCount: 2,
			skipFirst:   false,
			input:       "line1\nline2\n",
			want:        ">>>>line1\n>>>>line2\n",
		},
		{
			name:        "skip first",
			indent:      []byte("  "),
			indentCount: 1,
			skipFirst:   true,
			input:       "line1\nline2\n",
			want:        "line1\n  line2\n",
		},
		{
			name:        "empty string",
			indent:      []byte("  "),
			indentCount: 1,
			skipFirst:   false,
			input:       "",
			want:        "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeIndent(&buf, tt.indent, tt.indentCount, tt.skipFirst, tt.input)
			require.Equal(t, tt.want, buf.String())
		})
	}
}

func TestErrToStack_NilError(t *testing.T) {
	buf := getBuffer()
	defer buf.free()
	ErrToStack(nil, buf, 0)
	require.Empty(t, buf.String())
}

func TestErrToStack_SingleError(t *testing.T) {
	err := New("single error")
	result := ErrToStackString(err)
	require.Contains(t, result, "single error")
}

func TestErrToStack_WrappedError(t *testing.T) {
	base := New("base error")
	wrapped := Wrap(base, "wrapped error")
	result := ErrToStackString(wrapped)
	require.Contains(t, result, "wrapped error")
	require.Contains(t, result, "base error")
}

func TestErrToStack_JoinedErrors(t *testing.T) {
	e1 := New("error one")
	e2 := New("error two")
	joined := errors.Join(e1, e2)
	result := ErrToStackString(joined)
	require.Contains(t, result, "error one")
	require.Contains(t, result, "error two")
}

func TestErrToStackString(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil error",
			err:  nil,
			want: "",
		},
		{
			name: "new error",
			err:  New("test error"),
			want: "test error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrToStackString(tt.err)
			if tt.err == nil {
				require.Equal(t, tt.want, result)
			} else {
				require.Contains(t, result, tt.want)
			}
		})
	}
}

func TestPanicToErr_StringPanic(t *testing.T) {
	err := PanicToErr("string panic value")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "string panic value")
}

func TestPanicToErrWithMsg_StringPanic(t *testing.T) {
	tests := []struct {
		name   string
		panicV any
		msg    string
		want   string
	}{
		{
			name:   "with message",
			panicV: "boom",
			msg:    "something failed",
			want:   "something failed: boom",
		},
		{
			name:   "empty message",
			panicV: "boom",
			msg:    "",
			want:   "boom",
		},
		{
			name:   "int panic",
			panicV: 42,
			msg:    "numeric panic",
			want:   "numeric panic: 42",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PanicToErrWithMsg(tt.panicV, tt.msg)
			require.NotNil(t, err)
			require.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestPanicToErrWithMsg_ErrorPanic(t *testing.T) {
	original := New("original error")
	err := PanicToErrWithMsg(original, "panic handler")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "panic handler")
	require.Contains(t, err.Error(), "original error")
}

func TestErrFmtState_Flag(t *testing.T) {
	s := errFmtState{}

	tests := []struct {
		name string
		c    int
		want bool
	}{
		{
			name: "plus flag",
			c:    '+',
			want: true,
		},
		{
			name: "hash flag",
			c:    '#',
			want: false,
		},
		{
			name: "minus flag",
			c:    '-',
			want: false,
		},
		{
			name: "zero flag",
			c:    '0',
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, s.Flag(tt.c))
		})
	}
}

func TestErrFmtState_Width(t *testing.T) {
	s := errFmtState{}
	require.Panics(t, func() {
		s.Width()
	})
}

func TestErrFmtState_Precision(t *testing.T) {
	s := errFmtState{}
	require.Panics(t, func() {
		s.Precision()
	})
}

func TestBufferPool_GetBufferAndFree(t *testing.T) {
	buf := getBuffer()
	require.NotNil(t, buf)
	require.NotNil(t, buf.Buffer)

	// Write something and verify Reset works
	buf.WriteString("test")
	require.Equal(t, "test", buf.String())

	buf.free()

	// Get another buffer and verify it's been Reset
	buf2 := getBuffer()
	require.Empty(t, buf2.String())
	buf2.free()
}

func TestFrame_FormatVerbs(t *testing.T) {
	st := callers()
	require.NotEmpty(t, *st)
	f := Frame((*st)[0])

	tests := []struct {
		name     string
		format   string
		notEmpty bool
	}{
		{
			name:     "%s",
			format:   "%s",
			notEmpty: true,
		},
		{
			name:     "%+s",
			format:   "%+s",
			notEmpty: true,
		},
		{
			name:     "%d",
			format:   "%d",
			notEmpty: true,
		},
		{
			name:     "%n",
			format:   "%n",
			notEmpty: true,
		},
		{
			name:     "%v",
			format:   "%v",
			notEmpty: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fmt.Sprintf(tt.format, f)
			if tt.notEmpty {
				require.NotEmpty(t, result)
			}
			t.Logf("%s: %s", tt.format, result)
		})
	}
}

func TestFrame_MarshalText_KnownFrame(t *testing.T) {
	st := callers()
	require.NotEmpty(t, *st)
	f := Frame((*st)[0])

	text, err := f.MarshalText()
	require.NoError(t, err)
	require.NotEmpty(t, text)
	require.NotEqual(t, []byte("unknown"), text)
}

func TestFrame_MarshalText_UnknownFrame(t *testing.T) {
	f := Frame(0)
	text, err := f.MarshalText()
	require.NoError(t, err)
	require.Equal(t, []byte("unknown"), text)
}

func TestFrame_FileLineName(t *testing.T) {
	st := callers()
	require.NotEmpty(t, *st)
	f := Frame((*st)[0])

	// These should not panic and return reasonable values
	file := f.file()
	line := f.line()
	name := f.name()

	require.NotEmpty(t, file)
	require.NotEqual(t, "unknown", file)
	require.Greater(t, line, 0)
	require.NotEmpty(t, name)
	require.NotEqual(t, "unknown", name)
}

func TestFrame_FileLineName_Unknown(t *testing.T) {
	f := Frame(0)

	require.Equal(t, "unknown", f.file())
	require.Equal(t, 0, f.line())
	require.Equal(t, "unknown", f.name())
}

func TestStack_Format(t *testing.T) {
	st := callers()

	var buf bytes.Buffer
	st.Format(errFmtState{getBuffer()}, 'v')

	// Format with %+v
	result := fmt.Sprintf("%+v", st)
	require.NotEmpty(t, result)
	require.Contains(t, result, "\n")

	_ = buf
}

func TestStackTrace_Format(t *testing.T) {
	st := callers()
	trace := st.StackTrace()
	require.NotEmpty(t, trace)

	tests := []struct {
		name   string
		format string
	}{
		{
			name:   "%s",
			format: "%s",
		},
		{
			name:   "%v",
			format: "%v",
		},
		{
			name:   "%+v",
			format: "%+v",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fmt.Sprintf(tt.format, trace)
			require.NotEmpty(t, result)
			t.Logf("%s: %s", tt.format, result)
		})
	}
}

func TestCallers_NonEmpty(t *testing.T) {
	st := callers()
	require.NotNil(t, st)
	require.NotEmpty(t, *st)
}

func TestFuncname(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "full path",
			input: "github.com/donkeywon/golib/errs.TestFuncname",
			want:  "TestFuncname",
		},
		{
			name:  "simple name",
			input: "main.main",
			want:  "main",
		},
		{
			name:  "receiver method",
			input: "github.com/donkeywon/golib/errs.(*withStack).Format",
			want:  "(*withStack).Format",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, funcname(tt.input))
		})
	}
}

func TestDeduplicateWithStackErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "fundamental error",
			err:  New("test error"),
		},
		{
			name: "wrapped error",
			err:  Wrap(New("base"), "wrapped"),
		},
		{
			name: "std errors join",
			err:  errors.Join(New("e1"), New("e2")),
		},
		{
			name: "plain error",
			err:  errors.New("plain error"),
		},
		{
			name: "nil error",
			err:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				// Nil not directly tested - deduplicateWithStackErr is called
				// from WithStack which handles nil.
				return
			}
			// Wrap the error to exercise deduplicateWithStackErr via WithStack
			result := WithStack(tt.err)
			require.NotNil(t, result)
			_ = ErrToStackString(result)
		})
	}
}

func TestDeduplicateWithStackTracer(t *testing.T) {
	st := callers()

	// Test with a stackTracer (fundamental has stack)
	fund := New("test error")
	tracer, ok := fund.(*fundamental)
	require.True(t, ok)

	// Should return a valid index (0 to len(*st))
	idx := deduplicateWithStackTracer(tracer.stack, st)
	require.GreaterOrEqual(t, idx, 0)
	require.LessOrEqual(t, idx, len(*st))
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{
			name: "a less than b",
			a:    1,
			b:    5,
			want: 1,
		},
		{
			name: "b less than a",
			a:    5,
			b:    1,
			want: 1,
		},
		{
			name: "equal",
			a:    3,
			b:    3,
			want: 3,
		},
		{
			name: "negative",
			a:    -5,
			b:    2,
			want: -5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, minInt(tt.a, tt.b))
		})
	}
}

func TestWithStack_LogValue(t *testing.T) {
	base := New("base error")
	ws := WithStack(base)

	v := ws.(*withStack).LogValue()
	require.Equal(t, slog.KindString, v.Kind())
	require.NotEmpty(t, v.String())
}

func TestFundamental_LogValue(t *testing.T) {
	f := New("test message")

	v := f.(*fundamental).LogValue()
	require.Equal(t, slog.KindString, v.Kind())
	require.NotEmpty(t, v.String())
}

func TestErrToStack_FmtFormatter(t *testing.T) {
	// Test the fmt.Formatter branch in ErrToStack
	// A fundamental satisfies fmt.Formatter
	f := New("test")
	result := ErrToStackString(f)
	require.Contains(t, result, "test")
}

func TestErrToStack_DefaultBranch(t *testing.T) {
	// Test the default branch with a plain error (not fmt.Formatter, not wrapped, etc.)
	plainErr := &plainErrorNotFormatter{msg: "plain error"}
	result := ErrToStackString(plainErr)
	require.Contains(t, result, "plain error")
}

// plainErrorNotFormatter is an error that does NOT implement fmt.Formatter.
type plainErrorNotFormatter struct {
	msg string
}

func (e *plainErrorNotFormatter) Error() string { return e.msg }

func TestErrToStack_SingleWrappedError(t *testing.T) {
	// Test the branch where wrappedErrs has a single error
	e := &singleWrappedErr{err: New("single")}
	result := ErrToStackString(e)
	require.Contains(t, result, "single")
}

type singleWrappedErr struct {
	err error
}

func (e *singleWrappedErr) Error() string { return e.err.Error() }

func (e *singleWrappedErr) Unwrap() []error { return []error{e.err} }

func TestErrToStack_EmptyWrappedErrors(t *testing.T) {
	e := &emptyWrappedErr{}
	result := ErrToStackString(e)
	require.Empty(t, result)
}

type emptyWrappedErr struct{}

func (e *emptyWrappedErr) Error() string { return "empty" }

func (e *emptyWrappedErr) Unwrap() []error { return nil }

func TestErrToStack_AnotherWrappedErrs(t *testing.T) {
	e := &anotherWrappedErrsImpl{errs: []error{New("error A"), New("error B")}}
	result := ErrToStackString(e)
	require.Contains(t, result, "error A")
	require.Contains(t, result, "error B")
}

type anotherWrappedErrsImpl struct {
	errs []error
}

func (e *anotherWrappedErrsImpl) Error() string { return "multi" }

func (e *anotherWrappedErrsImpl) WrappedErrors() []error { return e.errs }

func TestNew(t *testing.T) {
	err := New("new error")
	require.Equal(t, "new error", err.Error())
	require.NotEmpty(t, ErrToStackString(err))
}

func TestWithStack(t *testing.T) {
	base := New("base")
	ws := WithStack(base)
	require.NotNil(t, ws)
	require.Contains(t, ws.Error(), "base")
}

func TestWrap(t *testing.T) {
	base := New("base")
	wrapped := Wrap(base, "wrapped")
	require.NotNil(t, wrapped)
	require.Contains(t, wrapped.Error(), "wrapped")
}

func TestWrapf(t *testing.T) {
	base := New("base")
	wrapped := Wrapf(base, "wrapped %d", 42)
	require.NotNil(t, wrapped)
	require.Contains(t, wrapped.Error(), "wrapped 42")
}

func TestWithMessage(t *testing.T) {
	base := New("base")
	wm := WithMessage(base, "msg")
	require.NotNil(t, wm)
	require.Contains(t, wm.Error(), "msg")
}

func TestWithMessagef(t *testing.T) {
	base := New("base")
	wm := WithMessagef(base, "msg %d", 42)
	require.NotNil(t, wm)
	require.Contains(t, wm.Error(), "msg 42")
}

func TestPanicToErr_Nil(t *testing.T) {
	defer func() {
		r := recover()
		require.Nil(t, r)
	}()
	err := PanicToErr(nil)
	// nil panic value should still produce an error
	require.NotNil(t, err)
}

func TestStack_Format_NonPlusV(t *testing.T) {
	st := callers()
	buf := getBuffer()
	defer buf.free()
	// Non-'+' flag 'v' should be a no-op
	st.Format(errFmtState{buf}, 's')
	require.Empty(t, buf.String())
}

func TestFrame_pc(t *testing.T) {
	st := callers()
	f := Frame((*st)[0])
	pc := f.pc()
	require.Greater(t, pc, uintptr(0))
	// pc should be uintptr(f) - 1
	require.Equal(t, uintptr(f)-1, pc)
}

func TestStackTrace_Format_hashFlag(t *testing.T) {
	st := callers()
	trace := st.StackTrace()
	result := fmt.Sprintf("%#v", trace)
	require.NotEmpty(t, result)
	require.Contains(t, result, "[")
	require.Contains(t, result, "]")
}

func TestDeduplicateWithStackErr_NilUnwrap(t *testing.T) {
	// Test branch where Unwrap returns nil
	nilUnwrap := &nilUnwrapError{}
	ws := WithStack(nilUnwrap)
	require.NotNil(t, ws)
}

type nilUnwrapError struct{}

func (e *nilUnwrapError) Error() string { return "nil unwrap" }

func (e *nilUnwrapError) Unwrap() error { return nil }

func TestErrToStack_WithStack_FoldAt(t *testing.T) {
	// Test the withStack branch in ErrToStack, ensuring foldAt logic is covered
	base := New("base")
	ws := WithStack(base)
	result := ErrToStackString(ws)
	require.Contains(t, result, "base")
	// foldAt is typically less than stack length for deduplicated frames
}

func TestErrToStack_WrappedErrWithNilUnwrap(t *testing.T) {
	// Test the wrappedErr branch where Unwrap returns nil
	nilUnwrap := &nilUnwrapError{}
	result := ErrToStackString(nilUnwrap)
	require.Contains(t, result, "nil unwrap")
}

func TestDeduplicateWithStackErr_MixedTypes(t *testing.T) {
	// Test with an error chain that has both wrappedErr and stackTracer
	base := New("base")
	// WithStack creates a withStack that is both wrappedErr and stackTracer
	ws := WithStack(base)
	// Wrapping it again exercises the combined path
	ws2 := Wrap(ws, "outer")
	require.NotNil(t, ws2)
	result := ErrToStackString(ws2)
	require.Contains(t, result, "outer")
	require.Contains(t, result, "base")
}

func TestDeduplicateWithStackErr_WithMessageChain(t *testing.T) {
	// Test the branch where a wrappedErr is not a stackTracer
	base := New("base")
	wm := WithMessage(base, "msg layer")
	ws := WithStack(wm)
	require.NotNil(t, ws)
	result := ErrToStackString(ws)
	require.Contains(t, result, "msg layer")
	require.Contains(t, result, "base")
}

func TestErrToStack_Nil(t *testing.T) {
	ErrToStack(nil, nil, 0)
	// Should not panic
}

func TestDeduplicateWithStackErr_AnotherWrappedErrs(t *testing.T) {
	// Test the anotherWrappedErrs branch (line 219-220 in stack.go)
	e := &anotherWrappedErrsWithItems{
		errs: []error{New("err A"), New("err B")},
	}
	ws := WithStack(e)
	require.NotNil(t, ws)
	result := ErrToStackString(ws)
	require.Contains(t, result, "err A")
	require.Contains(t, result, "err B")
}

type anotherWrappedErrsWithItems struct {
	errs []error
}

func (e *anotherWrappedErrsWithItems) Error() string          { return "multi err" }
func (e *anotherWrappedErrsWithItems) WrappedErrors() []error { return e.errs }

func TestDeduplicateWithStackErr_SingleAnotherWrappedErr(t *testing.T) {
	// Test the anotherWrappedErrs branch with a single error (line 219-220)
	e := &anotherWrappedErrsSingle{
		err: New("single err"),
	}
	ws := WithStack(e)
	require.NotNil(t, ws)
}

type anotherWrappedErrsSingle struct {
	err error
}

func (e *anotherWrappedErrsSingle) Error() string          { return e.err.Error() }
func (e *anotherWrappedErrsSingle) WrappedErrors() []error { return []error{e.err} }

func TestDeduplicateWithStackErr_NonMatchingTypes(t *testing.T) {
	// Test the fallthrough case where err is not wrappedErr/wrappedErrs/stackTracer
	// This covers line 237 in stack.go: return len(*s)
	plain := &plainErrorNotFormatter{msg: "plain"}
	ws := WithStack(plain)
	require.NotNil(t, ws)
	result := ErrToStackString(ws)
	require.Contains(t, result, "plain")
}

func TestDeduplicateWithStackTracer_NoMatch(t *testing.T) {
	// Test deduplicateWithStackTracer when there's no overlap between stacks.
	// Create a mock stackTracer with a PC that won't match any real stack.
	st := callers()
	mockTracer := &mockStackTracer{stack: []uintptr{0xDEADBEEF, 0xCAFEBABE}}
	idx := deduplicateWithStackTracer(mockTracer, st)
	// No match means return len(*s)
	require.Equal(t, len(*st), idx)
}

type mockStackTracer struct {
	stack []uintptr
}

func (m *mockStackTracer) Stack() []uintptr { return m.stack }

// formatVerb builds a format string with optional '+' flag.
func formatVerb(verb rune, plus bool) string {
	if plus {
		return "%+" + string(verb)
	}
	return "%" + string(verb)
}
