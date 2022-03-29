package v2

import (
	"testing"
	"time"

	"github.com/donkeywon/golib/oss"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

func TestPipelineWithCfg(t *testing.T) {
	c := NewCfg()
	c.
		Add(ReaderFile, &FileCfg{
			Path: "/tmp/test.file",
			Perm: 644,
		}, nil).
		Add(WorkerCopy, &CopyCfg{BufSize: 1024 * 1024}, nil).
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
	go func() {
		time.Sleep(time.Millisecond * 500)
		runner.Stop(ppl)
	}()
	require.NoError(t, runner.Init(ppl))
	runner.Start(ppl)
	require.NoError(t, ppl.Err())
}
