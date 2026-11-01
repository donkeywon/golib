package pipeline

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/donkeywon/golib/aio"

	"github.com/stretchr/testify/require"
)

func TestBufRead(t *testing.T) {
	r := BufRead(4)(strings.NewReader("abcdef"))
	br, ok := r.(*bufio.Reader)
	require.True(t, ok)
	require.GreaterOrEqual(t, br.Size(), 4, "bufio 有最小缓冲(16)")

	b, err := io.ReadAll(br)
	require.NoError(t, err)
	require.Equal(t, "abcdef", string(b))
}

func TestBufWrite(t *testing.T) {
	var out bytes.Buffer
	w := BufWrite(4)(&out)
	bw, ok := w.(*bufio.Writer)
	require.True(t, ok)
	require.GreaterOrEqual(t, bw.Size(), 4, "bufio 有最小缓冲(16)")

	_, err := bw.WriteString("abcdef")
	require.NoError(t, err)
	require.NoError(t, bw.Flush())
	require.Equal(t, "abcdef", out.String())
}

func TestAsyncWrite(t *testing.T) {
	var out bytes.Buffer
	w := AsyncWrite()(&out)
	aw, ok := w.(*aio.AsyncWriter)
	require.True(t, ok)

	_, err := aw.Write([]byte("async-data"))
	require.NoError(t, err)
	require.NoError(t, aw.Close())
	require.Equal(t, "async-data", out.String())
}

func TestTee(t *testing.T) {
	var side bytes.Buffer
	r := Tee(&side)(strings.NewReader("tee-data"))

	b, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "tee-data", string(b))
	require.Equal(t, "tee-data", side.String())
}

func TestTeeMultiWriters(t *testing.T) {
	var a, b bytes.Buffer
	r := Tee(&a, &b)(strings.NewReader("multi-tee"))

	_, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "multi-tee", a.String())
	require.Equal(t, "multi-tee", b.String())
}

func TestTeeEmptyPanics(t *testing.T) {
	require.PanicsWithValue(t, "empty tee", func() { Tee() })
}

func TestMultiWrite(t *testing.T) {
	var main, side bytes.Buffer
	w := MultiWrite(&side)(&main)

	_, err := w.Write([]byte("multi-write"))
	require.NoError(t, err)
	require.Equal(t, "multi-write", main.String())
	require.Equal(t, "multi-write", side.String())
}

func TestMultiWriteMultiSides(t *testing.T) {
	var main, a, b bytes.Buffer
	w := MultiWrite(&a, &b)(&main)

	_, err := w.Write([]byte("x"))
	require.NoError(t, err)
	require.Equal(t, "x", main.String())
	require.Equal(t, "x", a.String())
	require.Equal(t, "x", b.String())
}

func TestMultiWriteEmptyPanics(t *testing.T) {
	require.PanicsWithValue(t, "empty multi write", func() { MultiWrite() })
}
