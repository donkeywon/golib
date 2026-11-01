package pipeline

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/donkeywon/golib/util/cmds"

	"github.com/stretchr/testify/require"
)

func newReadyCmdWorker(t *testing.T, r io.Reader, w io.Writer, name string, args ...string) *cmdWorker {
	t.Helper()
	c := NewCmdWorker(name, args...).(*cmdWorker)
	if r != nil {
		c.ReadFromReader(r)
	}
	if w != nil {
		c.WriteToWriter(w)
	}
	require.NoError(t, c.Init(context.Background()))
	return c
}

func TestCmdWorkerSuccess(t *testing.T) {
	var out bytes.Buffer
	c := newReadyCmdWorker(t, nil, &out, "echo", "hello cmd")
	require.NoError(t, c.Run(context.Background()))
	require.Equal(t, "hello cmd\n", out.String())
}

func TestCmdWorkerStdin(t *testing.T) {
	var out bytes.Buffer
	c := newReadyCmdWorker(t, strings.NewReader("from stdin"), &out, "cat")
	require.NoError(t, c.Run(context.Background()))
	require.Equal(t, "from stdin", out.String())
}

func TestCmdWorkerFailure(t *testing.T) {
	c := newReadyCmdWorker(t, nil, &bytes.Buffer{}, "definitely-not-exist-xyz")
	err := c.Run(context.Background())
	require.ErrorContains(t, err, "exec cmd failed")
	require.ErrorContains(t, err, "executable file not found")
}

func TestCmdWorkerSignaled(t *testing.T) {
	c := newReadyCmdWorker(t, nil, &bytes.Buffer{}, "sh", "-c", "kill -9 $$")
	err := c.Run(context.Background())
	require.ErrorContains(t, err, "exec cmd failed")
	require.ErrorContains(t, err, "signal")
}

func TestCmdWorkerCanceled(t *testing.T) {
	// ctx 预先取消：cmd.Run 直接返回 context.Canceled（ProcessState 为 nil）
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := newReadyCmdWorker(t, nil, &bytes.Buffer{}, "echo", "never run")
	err := c.Run(ctx)
	require.ErrorContains(t, err, "exec cmd failed")
	require.ErrorIs(t, err, context.Canceled)
}

func TestCmdWorkerWithOptions(t *testing.T) {
	var out bytes.Buffer
	c := newReadyCmdWorker(t, nil, &out, "sh", "-c", "echo $PIPE_TEST_ENV")
	c.WithOptions(cmds.EnvKVs("PIPE_TEST_ENV", "env-ok"))
	require.NoError(t, c.Run(context.Background()))
	require.Equal(t, "env-ok\n", out.String())
}

func TestCmdWorkerDumpStdoutOverridden(t *testing.T) {
	// 已知缺陷（见报告 B1）：用户传入的 stdout 选项会被管道接线覆盖
	var dump bytes.Buffer
	var out bytes.Buffer
	c := newReadyCmdWorker(t, nil, &out, "echo", "hello")
	c.WithOptions(cmds.DumpStdout(&dump))
	require.NoError(t, c.Run(context.Background()))
	require.Equal(t, "hello\n", out.String())
	require.Empty(t, dump.String(), "当前行为：DumpStdout 被覆盖")
}

func TestCmdWorkerCloseErrors(t *testing.T) {
	// 关闭失败：既覆盖错误路径里的 Close(true) 错误分支，也覆盖 defer Close(false) 的错误分支
	// （注意这同时暴露了"错误路径双关闭"缺陷，见报告 B2）
	w := &closerWriter{err: errors.New("close boom")}
	c := newReadyCmdWorker(t, nil, w, "definitely-not-exist-xyz")
	err := c.Run(context.Background())
	require.ErrorContains(t, err, "exec cmd failed")
	require.ErrorContains(t, err, "close failed")
	require.ErrorContains(t, err, "close boom")
}

func TestCmdWorkerFailureWithoutStderr(t *testing.T) {
	// 失败但 stderr 为空：覆盖 "stderr 有内容才附加到错误" 的 false 分支
	c := newReadyCmdWorker(t, nil, &bytes.Buffer{}, "sh", "-c", "exit 7")
	err := c.Run(context.Background())
	require.ErrorContains(t, err, "exec cmd failed")
	require.NotContains(t, err.Error(), "stderr:")
}

func TestCmdWorkerFailureWithStderr(t *testing.T) {
	// 失败且 stderr 有内容：覆盖把 stderr 附加到错误的 true 分支
	c := newReadyCmdWorker(t, nil, &bytes.Buffer{}, "sh", "-c", "echo boom-err 1>&2; exit 7")
	err := c.Run(context.Background())
	require.ErrorContains(t, err, "exec cmd failed")
	require.ErrorContains(t, err, "stderr: boom-err")
}

func TestCmdWorkerSuccessWithStderr(t *testing.T) {
	// 成功但有 stderr：覆盖成功路径上的 stderr 日志字段
	c := newReadyCmdWorker(t, nil, &bytes.Buffer{}, "sh", "-c", "echo warn 1>&2")
	require.NoError(t, c.Run(context.Background()))
}

func TestCmdWorkerSuccessButCloseError(t *testing.T) {
	// 命令成功，但优雅关闭失败 -> defer 分支覆盖
	w := &closerWriter{err: errors.New("close boom")}
	c := newReadyCmdWorker(t, nil, w, "echo", "ok")
	err := c.Run(context.Background())
	require.ErrorContains(t, err, "close failed")
}

func TestCmdWorkerSupportZeroCopy(t *testing.T) {
	require.True(t, NewCmdWorker("echo").(*cmdWorker).SupportZeroCopy())
}

// ---- keepLastWriter（stderr 有界尾部缓冲）----

func TestKeepLastWriterPartial(t *testing.T) {
	w := newKeepLastWriter(8)
	n, err := w.Write([]byte("abc"))
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Equal(t, "abc", string(w.Bytes()))
	require.False(t, w.Truncated())
}

func TestKeepLastWriterExactFill(t *testing.T) {
	// 恰好填满：overwrite && offset==0 -> 返回整块
	w := newKeepLastWriter(4)
	n, err := w.Write([]byte("abcd"))
	require.NoError(t, err)
	require.Equal(t, 4, n)
	require.Equal(t, "abcd", string(w.Bytes()))
	require.False(t, w.Truncated(), "恰好填满不算截断")
}

func TestKeepLastWriterWrapOrder(t *testing.T) {
	// 超过容量：必须按时间顺序保留尾部
	w := newKeepLastWriter(8)
	n, err := w.Write([]byte("0123456789"))
	require.NoError(t, err)
	require.Equal(t, 10, n)
	require.Equal(t, "23456789", string(w.Bytes()))
	require.True(t, w.Truncated())

	// 继续写，仍应保持顺序
	_, _ = w.Write([]byte("XY"))
	require.Equal(t, "456789XY", string(w.Bytes()))
}

func TestKeepLastWriterZeroNPanics(t *testing.T) {
	// 契约：上限必须 > 0，否则 Write 会死循环（见 cmd.go 注释）
	require.PanicsWithValue(t, "keepLastWriter requires at least one byte", func() { newKeepLastWriter(0) })
}

func TestKeepLastWriterEmptyWrite(t *testing.T) {
	w := newKeepLastWriter(4)
	n, err := w.Write(nil)
	require.NoError(t, err)
	require.Equal(t, 0, n)
	require.Empty(t, w.Bytes())
	require.False(t, w.Truncated())
}

func TestCmdWorkerStderrTruncated(t *testing.T) {
	// 端到端：>8KB stderr -> 有界 + 标出截断 + 保留尾部顺序 + 用户 sink 仍完整
	var dump bytes.Buffer
	c := newReadyCmdWorker(t, nil, &bytes.Buffer{}, "sh", "-c", "head -c 20000 /dev/zero | tr '\\0' 'x' 1>&2; printf TAIL-END 1>&2; exit 3")
	c.WithOptions(cmds.DumpStderr(&dump))
	err := c.Run(context.Background())
	require.Error(t, err)
	require.Less(t, len(err.Error()), 12*1024, "错误消息必须有界")
	require.Contains(t, err.Error(), "stderr(truncated)")
	require.Contains(t, err.Error(), "TAIL-END", "尾部必须保留且顺序正确")
	require.Equal(t, 20008, dump.Len(), "用户 sink 仍应收到完整 stderr")
}
