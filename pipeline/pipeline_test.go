package pipeline

import (
	"testing"

	"github.com/donkeywon/golib/log"
	"github.com/donkeywon/golib/oss"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

type logWriter struct {
	log.Logger
}

func (l logWriter) Write(p []byte) (int, error) {
	l.Logger.Info("write", "len", len(p))
	return len(p), nil
}

func TestPipelineWithCfg(t *testing.T) {
	c := NewCfg()
	c.
		Add(ReaderFile, &FileCfg{
			Path: "/tmp/test.file",
			Perm: 644,
		}, nil).
		Add(WorkerCopy, &CopyCfg{}, nil).
		Add(WorkerCmd, &cmd.Cfg{Command: []string{"zstd"}}, nil).
		Add(WriterOSS, &OSSCfg{
			Cfg: &oss.Cfg{
				URL:     "",
				Ak:      "",
				Sk:      "",
				Timeout: 10,
				Region:  "",
			},
			Append: false,
		}, nil)

	ppl := New()
	ppl.SetCfg(c)
	tests.Init(ppl)

	ppl.Workers()[1].Writers()[0].(Writer).WithOptions(EnableBuf(8*1024*1024), MultiWrite(logWriter{ppl}))
	// go func() {
	// 	time.Sleep(time.Millisecond * 500)
	// 	runner.Stop(ppl)
	// }()
	require.NoError(t, runner.Init(ppl))
	runner.Start(ppl)
	if ppl.Err() != nil {
		ppl.Error("failed", ppl.Err())
	}
	require.NoError(t, ppl.Err())
}
