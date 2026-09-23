package pipeline

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type plainErrReader struct{ err error }

func (r *plainErrReader) Read([]byte) (int, error) { return 0, r.err }

func newReadyCopyWorker(t *testing.T, r io.Reader, w io.Writer, bufSize int) *copyWorker {
	t.Helper()
	c := NewCopyWorker(bufSize).(*copyWorker)
	c.ReadFromReader(r)
	c.WriteToWriter(w)
	require.NoError(t, c.Init(context.Background()))
	return c
}

func TestCopyWorkerDefaultBufSize(t *testing.T) {
	var out bytes.Buffer
	c := newReadyCopyWorker(t, strings.NewReader("abc"), &out, 0) // <=0 -> 32KB
	require.NoError(t, c.Run(context.Background()))
	require.Equal(t, "abc", out.String())
	require.Equal(t, 32*1024, c.bufSize)
}

func TestCopyWorkerCopyError(t *testing.T) {
	c := newReadyCopyWorker(t, &plainErrReader{err: errors.New("read boom")}, &bytes.Buffer{}, 8)
	err := c.Run(context.Background())
	require.ErrorContains(t, err, "copy failed")
	require.ErrorContains(t, err, "read boom")
}

func TestCopyWorkerCopyCanceled(t *testing.T) {
	// reader 直接返回 context.Canceled -> 走 "copy canceled" 分支
	c := newReadyCopyWorker(t, &plainErrReader{err: context.Canceled}, &bytes.Buffer{}, 8)
	err := c.Run(context.Background())
	require.ErrorContains(t, err, "copy canceled")
}

func TestCopyWorkerGracefulCloseError(t *testing.T) {
	// 复制成功，但优雅关闭失败
	w := &closerWriter{err: errors.New("close boom")}
	c := newReadyCopyWorker(t, strings.NewReader("abc"), w, 8)
	err := c.Run(context.Background())
	require.ErrorContains(t, err, "close failed")
	require.ErrorContains(t, err, "close boom")
}

func TestCopyWorkerCancelPathCloseOK(t *testing.T) {
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()

	var out bytes.Buffer
	c := newReadyCopyWorker(t, pr, &out, 8)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	time.Sleep(50 * time.Millisecond) // 让 copy 阻塞在 read 上
	cancel()

	select {
	case err := <-done:
		// 看门狗强制关闭；关流成功 -> 不追加 close 错误
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Run 未在取消后返回")
	}
}

func TestCopyWorkerCancelPathCloseError(t *testing.T) {
	pr, _ := io.Pipe()
	w := &closerWriter{err: errors.New("force close boom")}
	c := newReadyCopyWorker(t, pr, w, 8)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.ErrorContains(t, err, "close failed")
		require.ErrorContains(t, err, "force close boom")
	case <-time.After(3 * time.Second):
		t.Fatal("Run 未在取消后返回")
	}
}

func TestCopyWorkerNilReaderWriter(t *testing.T) {
	// 缺陷（见报告）：未接线就 Run 会对 nil 接口调用 Read -> panic
	c := NewCopyWorker(8).(*copyWorker)
	require.NoError(t, c.Init(context.Background()))
	require.Panics(t, func() { _ = c.Run(context.Background()) })
}
