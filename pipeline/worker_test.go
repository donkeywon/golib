package pipeline

import (
	"testing"

	"github.com/donkeywon/golib/oss"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

func TestCopy(t *testing.T) {
	f := NewFileReader()
	f.SetCfg(&FileCfg{
		Path: "/tmp/test.file",
	})
	tests.Init(f)

	o := NewOSSWriter()
	o.SetCfg(&OSSCfg{
		Cfg: &oss.Cfg{
			URL:     "",
			Ak:      "",
			Sk:      "",
			Timeout: 10,
			Region:  "",
		},
	})
	o.WithOptions(EnableBuf(8 * 1024 * 1024))
	tests.Init(o)

	compress := NewCompressWriter()
	compress.SetCfg(&CompressCfg{
		Type:        CompressTypeZstd,
		Level:       CompressLevelFast,
		Concurrency: 2,
	})
	tests.Init(compress)

	c := NewCopy()
	c.ReadFrom(f)
	c.WriteTo(o)
	tests.Init(c)
	require.NoError(t, runner.Init(c))

	runner.Start(c)
	if c.Err() != nil {
		c.Error("copy failed", c.Err())
	}
}
