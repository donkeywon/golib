package oss

import (
	"bufio"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/donkeywon/golib/util/oss"
	"github.com/stretchr/testify/require"
)

func TestMultiPartWriter(t *testing.T) {
	c := testCfg()
	c.Parallel = 4
	w := NewMultiPartWriter(context.TODO(), c)
	testWriter(t, w, c, 32*1024)
}

func TestAppendWriter(t *testing.T) {
	c := testCfg()
	w := NewAppendWriter(context.TODO(), c)
	testWriter(t, w, c, 32*1024)
}

func TestMultiPartWriterWithoutReadFrom(t *testing.T) {
	c := testCfg()
	c.Parallel = 4
	w := NewMultiPartWriter(context.TODO(), c)
	testWriter(t, bufio.NewWriterSize(&mpwWithoutReadFrom{MultiPartWriter: w}, 5*1024*1024), c, 32*1024)
	require.NoError(t, w.Close())
}

func TestAppendWriterWithoutReadFrom(t *testing.T) {
	c := testCfg()
	w := NewAppendWriter(context.TODO(), c)
	testWriter(t, &apwWithoutReadFrom{AppendWriter: w}, c, 5*1024*1024)
}

func testCfg() *Cfg {
	return &Cfg{
		URL:     "",
		Retry:   3,
		Timeout: 10,
		Ak:      "",
		Sk:      "",
		Region:  "",
	}
}

type flusher interface {
	Flush() error
}

type noWriteTo struct{}

func (noWriteTo) WriteTo(io.Writer) (int64, error) {
	panic("can't happen")
}

type fileWithoutWriteTo struct {
	noWriteTo
	*os.File
}

func testWriter(t *testing.T, w io.Writer, c *Cfg, bufSize int) {
	err := oss.Delete(context.TODO(), time.Minute, c.URL, c.Ak, c.Sk, c.Region)
	require.NoError(t, err)

	f, _ := os.OpenFile("/tmp/test.file.zst", os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()

	buf := make([]byte, bufSize)
	_, err = io.CopyBuffer(w, fileWithoutWriteTo{File: f}, buf)
	require.NoError(t, err)
	if wf, ok := w.(flusher); ok {
		require.NoError(t, wf.Flush())
	}
	if wc, ok := w.(io.Closer); ok {
		require.NoError(t, wc.Close())
	}
}
