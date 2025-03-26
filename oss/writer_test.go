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
	w := NewMultiPartWriter(context.TODO(), c)
	testWriter(t, w, c)
}

func TestAppendWriter(t *testing.T) {
	c := testCfg()
	w := NewAppendWriter(context.TODO(), c)
	testWriter(t, w, c)
}

func TestMultiPartWriterWithoutReadFrom(t *testing.T) {
	c := testCfg()
	w := NewMultiPartWriter(context.TODO(), c)
	testWriter(t, bufio.NewWriterSize(w, 1024*1024), c)
}

func TestAppendWriterWithoutReadFrom(t *testing.T) {
	c := testCfg()
	w := NewAppendWriter(context.TODO(), c)
	testWriter(t, &apwWithoutReadFrom{AppendWriter: w}, c)
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

func testWriter(t *testing.T, w io.Writer, c *Cfg) {
	err := oss.Delete(context.TODO(), time.Minute, c.URL, c.Ak, c.Sk, c.Region)
	require.NoError(t, err)

	f, _ := os.OpenFile("/tmp/test.file", os.O_CREATE|os.O_RDWR, 0644)
	defer f.Close()

	_, err = io.Copy(w, f)
	require.NoError(t, err)
	if wf, ok := w.(flusher); ok {
		require.NoError(t, wf.Flush())
	}
	if wc, ok := w.(io.Closer); ok {
		require.NoError(t, wc.Close())
	}
}
