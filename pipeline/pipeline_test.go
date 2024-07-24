package pipeline

import (
	"testing"

	"github.com/donkeywon/golib/oss"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

type logWriter struct {
	Common
	total int64
}

func (l *logWriter) Write(p []byte) (int, error) {
	n := len(p)
	l.total += int64(n)
	l.Info("write", "len", n, "total", l.total)
	return n, nil
}

func (l *logWriter) Set(c Common) {
	l.Common = c
}

func TestPipelineWithCfg(t *testing.T) {
	c := NewCfg()

	c.Add(WorkerCopy, NewCopyCfg(), &CommonOption{}).
		ReadFrom(ReaderFile, &FileCfg{
			Path: "/tmp/test.file",
			Perm: 644,
		}, &CommonOption{
			Count: true,
			Hash:  "xxh3",
		}).
		WriteTo(WriterCompress, &CompressCfg{
			Type:        CompressTypeZstd,
			Level:       CompressLevelFast,
			Concurrency: 4,
		}, &CommonOption{}).
		WriteTo(WriterOSS,
			&OSSCfg{
				Cfg: &oss.Cfg{
					URL:     "",
					Ak:      "",
					Sk:      "",
					Timeout: 10,
					Region:  "",
				},
				Append: false,
			},
			&CommonOption{
				ProgressLogInterval: 1,
				BufSize:             5 * 1024 * 1024,
			})

	//c.Add(WorkerCmd, &cmd.Cfg{Command: []string{"cat"}}, CommonOption{}).
	//	WriteTo(WriterCompress, &CompressCfg{
	//		Type:        CompressTypeZstd,
	//		Level:       CompressLevelFast,
	//		Concurrency: 4,
	//	}, CommonOption{}).
	//	WriteTo(WriterOSS,
	//		&OSSCfg{
	//			Cfg: &oss.Cfg{
	//				URL:     "",
	//				Ak:      "",
	//				Sk:      "",
	//				Timeout: 10,
	//				Region:  "",
	//			},
	//			Append: false,
	//		},
	//		CommonOption{
	//			BufSize: 5 * 1024 * 1024,
	//		})

	ppl := New()
	ppl.SetCfg(c)
	//ppl.Workers()[1].Writers()[0].(Writer).WrapWriter(io.Discard)

	tests.DebugInit(ppl)

	//go func() {
	//	time.Sleep(time.Millisecond * 500)
	//	runner.Stop(ppl)
	//}()
	require.NoError(t, runner.Init(ppl))
	err := runner.Run(ppl)
	if err != nil {
		ppl.Error("failed", err)
	}
	require.NoError(t, err)
}

func TestPipelineCmd(t *testing.T) {
	c := NewCfg()
	c.Add(WorkerCmd, &cmd.Cfg{Command: []string{"bash", "-c", "cat /tmp/test.file"}}, &CommonOption{})
	c.Add(WorkerCmd, &cmd.Cfg{Command: []string{"bash", "-c", "cat > /tmp/test.file.1"}}, &CommonOption{})
	ppl := New()
	ppl.SetCfg(c)
	tests.DebugInit(ppl)
	require.NoError(t, runner.Init(ppl))
	err := runner.Run(ppl)
	if err != nil {
		ppl.Error("failed", err)
	}
	require.NoError(t, err)
}
