package pipeline

import (
	"testing"

	"github.com/donkeywon/golib/oss"
	"github.com/donkeywon/golib/ratelimit"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

type logWriter struct {
	Common
}

func (l *logWriter) Write(p []byte) (int, error) {
	l.Info("write", "len", len(p))
	return len(p), nil
}

func (l *logWriter) Set(c Common) {
	l.Common = c
}

func TestPipelineWithCfg(t *testing.T) {
	c := NewCfg().
		Add(&WorkerCfg{
			Type: WorkerCopy,
			Readers: []*ReaderCfg{
				{
					CommonCfg: CommonCfg{
						Type: ReaderFile,
						Cfg: &FileCfg{
							Path: "/tmp/test.file",
							Perm: 644,
						},
					},
					CommonOption: CommonOption{
						ProgressLogInterval: 1,
					},
				},
			},
		}).
		Add(&WorkerCfg{
			Type: WorkerCmd,
			Cfg:  &cmd.Cfg{Command: []string{"zstd"}},
			Writers: []*WriterCfg{
				{
					CommonCfg: CommonCfg{
						Type: WriterCompress,
						Cfg: &CompressCfg{
							Type:        CompressTypeZstd,
							Level:       CompressLevelFast,
							Concurrency: 4,
						},
					},
				},
				{
					CommonCfg: CommonCfg{
						Type: WriterOSS,
						Cfg: &OSSCfg{
							Cfg: &oss.Cfg{
								URL:     "",
								Ak:      "",
								Sk:      "",
								Timeout: 10,
								Region:  "",
							},
							Append: false,
						},
					},
					CommonOption: CommonOption{
						BufSize: 5 * 1024 * 1024,
						RateLimitCfg: &ratelimit.Cfg{
							Type: ratelimit.TypeSleep,
							Cfg:  &ratelimit.SleepRateLimiterCfg{Millisecond: 100},
						},
					},
				},
			},
		})

	ppl := New()
	ppl.SetCfg(c)
	tests.Init(ppl)

	// ppl.Workers()[1].Writers()[0].(Writer).WithOptions(MultiWrite(&logWriter{}))
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
