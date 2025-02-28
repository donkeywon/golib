package pipeline

import (
	"testing"
	"time"

	"github.com/donkeywon/golib/pipeline/rw"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

func TestGroup(t *testing.T) {
	cfg := NewRWGroupCfg().
		//SetStarter(TypeCopy, &CopyCfg{BufSize: 32 * 1024}, nil).
		// FromReader(TypeCompress, &CompressCfg{Type: CompressTypeZstd, Level: CompressLevelFast, Concurrency: 1}, nil).
		// FromReader(TypeFile, &FileCfg{Path: "/root/test.file.compress"}, nil).
		//FromReader(TypeTail, &TailCfg{Path: "/root/test.file"}, nil).
		//ToWriter(TypeCompress, &CompressCfg{Type: CompressTypeZstd, Level: CompressLevelBetter, Concurrency: 1}, nil).
		ToWriter(rw.TypeFile, &rw.FileCfg{Path: "/dev/null"}, nil)

	g := NewRWGroup()
	g.RWGroupCfg = cfg
	tests.Init(g)

	err := runner.Init(g)
	require.NoError(t, err)

	//go func() {
	//	time.Sleep(time.Second * 1)
	//	runner.Stop(g)
	//}()

	runner.Start(g)
	require.NoError(t, g.Err())
}

func TestStore(t *testing.T) {
	cfg := NewRWGroupCfg().SetStarter(rw.TypeSSH, &rw.SSHCfg{
		Addr:    "127.0.0.1:22",
		User:    "",
		Pwd:     "",
		Path:    "/root/test.file1",
		Timeout: 1,
	}, nil).
		FromReader(rw.TypeTail, &rw.TailCfg{Path: "/root/test.file"}, nil)
	// ToWriter(TypeFile, &FileCfg{Path: "/root/test.file"}, nil)

	g := NewRWGroup()
	g.RWGroupCfg = cfg
	tests.DebugInit(g)

	require.NoError(t, runner.Init(g))

	go func() {
		time.Sleep(time.Second * 3)
		runner.Stop(g)
	}()

	runner.Start(g)
	require.NoError(t, g.Err())
}

func TestFtp(t *testing.T) {
	cfg := NewRWGroupCfg().SetStarter(rw.TypeCopy, &rw.CopyCfg{BufSize: 32 * 1024}, nil).
		FromReader(rw.TypeFile, &rw.FileCfg{Path: "/root/test.file"}, nil).
		ToWriter(rw.TypeFtp, &rw.FtpCfg{
			Addr:    "127.0.0.1:21",
			User:    "",
			Pwd:     "",
			Path:    "test.file1",
			Retry:   1,
			Timeout: 1,
		}, nil)

	g := NewRWGroup()
	g.RWGroupCfg = cfg
	tests.DebugInit(g)

	require.NoError(t, runner.Init(g))

	runner.Start(g)
	require.NoError(t, g.Err())
}

func TestOSS(t *testing.T) {
	cfg := NewRWGroupCfg().SetStarter(rw.TypeCopy, &rw.CopyCfg{BufSize: 1024 * 1024}, nil).
		FromReader(rw.TypeFile, &rw.FileCfg{Path: "/root/test.file.zst"}, nil).
		ToWriter(rw.TypeOss, &rw.OSSCfg{
			Ak:      "",
			Sk:      "",
			Append:  false,
			URL:     "",
			Retry:   1,
			Timeout: 10,
		}, &rw.ExtraCfg{
			BufSize:          1024 * 1024,
			AsyncChanBufSize: 5,
			EnableAsync:      true,
		})

	g := NewRWGroup()
	g.RWGroupCfg = cfg
	tests.DebugInit(g)

	require.NoError(t, runner.Init(g))

	runner.Start(g)
	require.NoError(t, g.Err())
}
