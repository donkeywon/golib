package pipeline

import (
	"testing"
	"time"

	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

func TestGroup(t *testing.T) {
	cfg := NewRWGroupCfg().SetStarter(RWTypeCopy, &CopyRWCfg{BufSize: 32 * 1024}, nil).
		// FromReader(RWTypeCompress, &CompressRWCfg{Type: CompressTypeZstd, Level: CompressLevelFast, Concurrency: 1}, nil).
		// FromReader(RWTypeFile, &FileRWCfg{Path: "/root/test.file.compress"}, nil).
		FromReader(RWTypeTail, &TailRWCfg{Path: "/root/test.file"}, nil).
		// ToWriter(RWTypeCompress, &CompressRWCfg{Type: CompressTypeZstd, Level: CompressLevelFast, Concurrency: 1}, nil).
		ToWriter(RWTypeFile, &FileRWCfg{Path: "/root/test.file"}, nil)

	g := NewRWGroup()
	g.RWGroupCfg = cfg
	tests.Init(g)

	err := runner.Init(g)
	require.NoError(t, err)

	go func() {
		time.Sleep(time.Second * 3)
		runner.Stop(g)
	}()

	runner.Start(g)
	require.NoError(t, g.Err())
}

func TestStore(t *testing.T) {
	cfg := NewRWGroupCfg().SetStarter(RWTypeStore, &StoreCfg{
		Type: StoreTypeSSH,
		Cfg: &SSHCfg{
			Addr: "127.0.0.1:22",
			User: "",
			Pwd:  "",
		},
		URL:     "/root/test.file1",
		Retry:   1,
		Timeout: 1,
	}, nil).
		FromReader(RWTypeTail, &TailRWCfg{Path: "/root/test.file"}, nil)
		// ToWriter(RWTypeFile, &FileRWCfg{Path: "/root/test.file"}, nil)

	g := NewRWGroup()
	g.RWGroupCfg = cfg
	tests.Init(g)

	require.NoError(t, runner.Init(g))

	go func() {
		time.Sleep(time.Second * 3)
		runner.Stop(g)
	}()

	runner.Start(g)
	require.NoError(t, g.Err())
}

func TestFtp(t *testing.T) {
	cfg := NewRWGroupCfg().SetStarter(RWTypeCopy, &CopyRWCfg{BufSize: 32 * 1024}, nil).
		FromReader(RWTypeFile, &FileRWCfg{Path: "/root/test.file"}, nil).
		ToWriter(RWTypeStore, &StoreCfg{
			Type: StoreTypeFtp,
			Cfg: &FtpCfg{
				Addr: "127.0.0.1:21",
				User: "",
				Pwd:  "",
			},
			URL:     "test.file1",
			Retry:   1,
			Timeout: 1,
		}, nil)

	g := NewRWGroup()
	g.RWGroupCfg = cfg
	tests.Init(g)

	require.NoError(t, runner.Init(g))

	runner.Start(g)
	require.NoError(t, g.Err())
}

func TestOSS(t *testing.T) {
	cfg := NewRWGroupCfg().SetStarter(RWTypeCopy, &CopyRWCfg{BufSize: 1024 * 1024}, nil).
		FromReader(RWTypeFile, &FileRWCfg{Path: "/root/test.file.zst"}, nil).
		ToWriter(RWTypeStore, &StoreCfg{
			Type: StoreTypeOss,
			Cfg: &OssCfg{
				Ak:     "",
				Sk:     "",
				Append: false,
			},
			URL:     "",
			Retry:   1,
			Timeout: 1,
		}, &RWCommonCfg{
			BufSize:          1024 * 1024,
			AsyncChanBufSize: 5,
			EnableAsync:      true,
		})

	g := NewRWGroup()
	g.RWGroupCfg = cfg
	tests.Init(g)

	require.NoError(t, runner.Init(g))

	runner.Start(g)
	require.NoError(t, g.Err())
}
