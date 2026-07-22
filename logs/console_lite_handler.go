package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/petermattis/goid"
)

var bufferPool = sync.Pool{
	New: func() any { return make([]byte, 0, 1024) },
}

type ConsoleLiteHandler struct {
	mu                sync.Mutex
	w                 io.Writer
	opts              slog.HandlerOptions
	preformattedAttrs []byte
	groups            []string
}

func NewConsoleLiteHandler(w io.Writer, opts *slog.HandlerOptions) *ConsoleLiteHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &ConsoleLiteHandler{w: w, opts: *opts}
}

func (h *ConsoleLiteHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *ConsoleLiteHandler) Handle(ctx context.Context, r slog.Record) error {
	buf := bufferPool.Get().([]byte)[:0]
	defer func() {
		const maxKeep = 16 << 10
		if cap(buf) <= maxKeep {
			bufferPool.Put(buf[:0])
		}
	}()

	buf = append(buf, levelChar(r.Level))
	buf = r.Time.AppendFormat(buf, "0102 15:04:05.000000")
	buf = append(buf, '\t')
	buf = append(buf, r.Message...)
	buf = append(buf, '\t', '{')

	first := true
	if src := formatSourceLite(r); src != "" {
		buf, first = writeJSON(buf, first, "source", slog.StringValue(src))
	}

	if len(h.preformattedAttrs) > 0 {
		if !first {
			buf = append(buf, ',', ' ')
		}
		buf = append(buf, h.preformattedAttrs...)
		first = false
	}

	if r.NumAttrs() > 0 {
		pos := len(buf)
		anyAppended := false
		r.Attrs(func(a slog.Attr) bool {
			b, f, ok := appendAttr(buf, first, a, h.opts.ReplaceAttr, h.groups)
			buf, first = b, f
			if ok {
				anyAppended = true
			}
			return true
		})
		if !anyAppended {
			buf = buf[:pos]
		}
	}

	buf, _ = writeJSON(buf, first, "goid", slog.Int64Value(goid.Get()))
	buf = append(buf, '}', '\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf)
	return err
}

func (h *ConsoleLiteHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	h2 := h.clone()
	buf := make([]byte, 0, 256)
	first := true
	if len(h2.preformattedAttrs) > 0 {
		first = false
	}
	for _, a := range attrs {
		v := a.Value.Resolve()
		if h2.opts.ReplaceAttr != nil && v.Kind() != slog.KindGroup {
			a = h2.opts.ReplaceAttr(h.groups, a)
			v = a.Value.Resolve()
		}
		if a.Equal(slog.Attr{}) {
			continue
		}
		buf, first = writeJSON(buf, first, a.Key, v)
	}
	h2.preformattedAttrs = append(h2.preformattedAttrs, buf...)
	return h2
}

func (h *ConsoleLiteHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := h.clone()
	h2.groups = append(h2.groups, name)
	return h2
}

func (h *ConsoleLiteHandler) clone() *ConsoleLiteHandler {
	groups := make([]string, len(h.groups))
	copy(groups, h.groups)
	pfa := make([]byte, len(h.preformattedAttrs))
	copy(pfa, h.preformattedAttrs)
	return &ConsoleLiteHandler{
		w:                 h.w,
		opts:              h.opts,
		preformattedAttrs: pfa,
		groups:            groups,
	}
}

func formatSourceLite(r slog.Record) string {
	src := r.Source()
	if src == nil || src.File == "" {
		return ""
	}
	file := src.File
	slashes := 0
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' {
			slashes++
			if slashes == 2 {
				file = file[i+1:]
				break
			}
		}
	}
	if slashes == 1 {
		for i := len(file) - 1; i >= 0; i-- {
			if file[i] == '/' {
				file = file[i+1:]
				break
			}
		}
	}
	return file + ":" + strconv.Itoa(src.Line)
}

func writeJSON(buf []byte, first bool, key string, v slog.Value) ([]byte, bool) {
	if !first {
		buf = append(buf, ',', ' ')
	}
	buf = appendEscapedJSONString(buf, key)
	buf = append(buf, ':', ' ')
	return appendJSONValue(buf, v), false
}

func appendAttr(buf []byte, first bool, a slog.Attr, replaceAttr func([]string, slog.Attr) slog.Attr, groups []string) ([]byte, bool, bool) {
	v := a.Value.Resolve()
	if replaceAttr != nil && v.Kind() != slog.KindGroup {
		a = replaceAttr(groups, a)
		v = a.Value.Resolve()
	}
	if a.Equal(slog.Attr{}) {
		return buf, first, false
	}
	if v.Kind() == slog.KindGroup {
		attrs := v.Group()
		if len(attrs) == 0 {
			return buf, first, false
		}
		if a.Key != "" {
			if !first {
				buf = append(buf, ',', ' ')
			}
			buf = appendEscapedJSONString(buf, a.Key)
			buf = append(buf, ':', ' ', '{')
			innerFirst := true
			for _, ga := range attrs {
				buf, innerFirst = writeJSON(buf, innerFirst, ga.Key, ga.Value.Resolve())
			}
			return append(buf, '}'), false, true
		}
		for _, ga := range attrs {
			buf, first = writeJSON(buf, first, ga.Key, ga.Value.Resolve())
		}
		return buf, first, true
	}
	buf, first = writeJSON(buf, first, a.Key, v)
	return buf, first, true
}

func appendJSONMarshal(buf []byte, v any) ([]byte, error) {
	enc := jsonEncoderPool.Get().(*jsonEncoder)
	defer func() {
		const maxBufferSize = 16 << 10
		if enc.buf.Cap() > maxBufferSize {
			return
		}
		enc.buf.Reset()
		jsonEncoderPool.Put(enc)
	}()
	if err := enc.json.Encode(v); err != nil {
		return buf, err
	}
	b := bytes.TrimRight(enc.buf.Bytes(), "\n")
	return append(buf, b...), nil
}

func appendJSONValue(buf []byte, v slog.Value) []byte {
	switch v.Kind() {
	case slog.KindString:
		return appendEscapedJSONString(buf, v.String())
	case slog.KindInt64:
		return strconv.AppendInt(buf, v.Int64(), 10)
	case slog.KindUint64:
		return strconv.AppendUint(buf, v.Uint64(), 10)
	case slog.KindFloat64:
		b, err := appendJSONMarshal(buf, v.Float64())
		if err != nil {
			return appendEscapedJSONString(buf, fmt.Sprintf("!ERROR:%v", err))
		}
		return b
	case slog.KindBool:
		return strconv.AppendBool(buf, v.Bool())
	case slog.KindDuration:
		return strconv.AppendInt(buf, int64(v.Duration()), 10)
	case slog.KindTime:
		buf = append(buf, '"')
		buf = v.Time().AppendFormat(buf, time.RFC3339Nano)
		return append(buf, '"')
	case slog.KindGroup:
		buf = append(buf, '{')
		first := true
		for _, a := range v.Group() {
			buf, first = writeJSON(buf, first, a.Key, a.Value.Resolve())
		}
		return append(buf, '}')
	case slog.KindAny:
		a := v.Any()
		if err, ok := a.(error); ok {
			return appendEscapedJSONString(buf, err.Error())
		}
		b, err := appendJSONMarshal(buf, a)
		if err != nil {
			return appendEscapedJSONString(buf, fmt.Sprintf("!ERROR:%v", err))
		}
		return b
	default:
		panic("bad kind: " + v.Kind().String())
	}
}

type jsonEncoder struct {
	buf  bytes.Buffer
	json *json.Encoder
}

var jsonEncoderPool = &sync.Pool{
	New: func() any {
		enc := &jsonEncoder{}
		enc.json = json.NewEncoder(&enc.buf)
		enc.json.SetEscapeHTML(false)
		return enc
	},
}

func levelChar(l slog.Level) byte {
	switch l {
	case slog.LevelDebug:
		return 'D'
	case slog.LevelInfo:
		return 'I'
	case slog.LevelWarn:
		return 'W'
	case slog.LevelError:
		return 'E'
	default:
		return '?'
	}
}

func appendEscapedJSONString(buf []byte, s string) []byte {
	buf = append(buf, '"')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if safeSet[b] {
				i++
				continue
			}
			if start < i {
				buf = append(buf, s[start:i]...)
			}
			buf = append(buf, '\\')
			switch b {
			case '\\', '"':
				buf = append(buf, b)
			case '\n':
				buf = append(buf, 'n')
			case '\r':
				buf = append(buf, 'r')
			case '\t':
				buf = append(buf, 't')
			default:
				buf = append(buf, "u00"...)
				buf = append(buf, hex[b>>4], hex[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				buf = append(buf, s[start:i]...)
			}
			buf = append(buf, `\ufffd`...)
			i += size
			start = i
			continue
		}
		if c == '\u2028' || c == '\u2029' {
			if start < i {
				buf = append(buf, s[start:i]...)
			}
			buf = append(buf, `\u202`...)
			buf = append(buf, hex[c&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		buf = append(buf, s[start:]...)
	}
	return append(buf, '"')
}

const hex = "0123456789abcdef"

var safeSet = [utf8.RuneSelf]bool{
	' ': true, '!': true, '"': false, '#': true, '$': true, '%': true, '&': true,
	'\'': true, '(': true, ')': true, '*': true, '+': true, ',': true, '-': true,
	'.': true, '/': true, '0': true, '1': true, '2': true, '3': true, '4': true,
	'5': true, '6': true, '7': true, '8': true, '9': true, ':': true, ';': true,
	'<': true, '=': true, '>': true, '?': true, '@': true, 'A': true, 'B': true,
	'C': true, 'D': true, 'E': true, 'F': true, 'G': true, 'H': true, 'I': true,
	'J': true, 'K': true, 'L': true, 'M': true, 'N': true, 'O': true, 'P': true,
	'Q': true, 'R': true, 'S': true, 'T': true, 'U': true, 'V': true, 'W': true,
	'X': true, 'Y': true, 'Z': true, '[': true, '\\': false, ']': true, '^': true,
	'_': true, '`': true, 'a': true, 'b': true, 'c': true, 'd': true, 'e': true,
	'f': true, 'g': true, 'h': true, 'i': true, 'j': true, 'k': true, 'l': true,
	'm': true, 'n': true, 'o': true, 'p': true, 'q': true, 'r': true, 's': true,
	't': true, 'u': true, 'v': true, 'w': true, 'x': true, 'y': true, 'z': true,
	'{': true, '|': true, '}': true, '~': true, '\u007f': true,
}
