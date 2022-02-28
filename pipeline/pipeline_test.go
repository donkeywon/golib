package pipeline

import (
	"fmt"
	"testing"
	"time"

	"github.com/donkeywon/golib/pipeline/rw"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

func TestGroupByStarter(t *testing.T) {
	rws := []*rw.Cfg{}
	g := groupRWCfgByStarter(rws)
	require.Empty(t, g)

	rws = []*rw.Cfg{
		{Role: rw.RoleReader},
	}
	g = groupRWCfgByStarter(rws)
	require.Len(t, g, 1)
	require.Len(t, g[0], 1)

	rws = []*rw.Cfg{
		{Role: rw.RoleReader},
		{Role: rw.RoleStarter},
		{Role: rw.RoleWriter},
	}
	g = groupRWCfgByStarter(rws)
	require.Len(t, g, 1)
	require.Len(t, g[0], 3)

	rws = []*rw.Cfg{
		{Role: rw.RoleReader},
		{Role: rw.RoleReader},
	}
	g = groupRWCfgByStarter(rws)
	require.Len(t, g, 1)
	require.Len(t, g[0], 2)

	rws = []*rw.Cfg{
		{Role: rw.RoleReader},
		{Role: rw.RoleReader},
		{Role: rw.RoleStarter},
		{Role: rw.RoleStarter},
		{Role: rw.RoleWriter},
		{Role: rw.RoleWriter},
		{Role: rw.RoleStarter},
	}
	g = groupRWCfgByStarter(rws)
	require.Len(t, g, 3)
	require.Len(t, g[0], 3)
	require.Len(t, g[1], 3)
	require.Len(t, g[2], 1)
}

func TestMultiGroupPipeline(t *testing.T) {
	cfg := NewCfg().
		//Add(RoleReader, TypeFile, &FileCfg{Path: "/root/test.file"}, nil).
		//Add(RoleWriter, TypeCompress, &CompressCfg{Type: CompressTypeZstd, Level: CompressLevelBetter, Concurrency: 1}, nil).
		//Add(rw.RoleStarter, rw.TypeCmd, &cmd.Cfg{Command: []string{"zstd"}}, nil).
		Add(rw.RoleStarter, rw.TypeCopy, &rw.CopyCfg{}, nil).
		Add(rw.RoleWriter, rw.TypeFile, &rw.FileCfg{Path: "/dev/null"}, nil)

	p := New()
	p.Cfg = cfg
	tests.Init(p)
	err := runner.Init(p)
	require.NoError(t, err)

	//p.rwGroups[0].LastWriter().HookWrite(func(n int, bs []byte, err error, cost int64, misc ...any) error {
	//	time.Sleep(time.Second)
	//	return nil
	//})
	//
	go func() {
		time.Sleep(time.Second * 1)
		runner.Stop(p)
	}()

	runner.Start(p)
	require.NoError(t, p.Err())
}

func TestCompress(t *testing.T) {
	cfg := NewCfg().
		Add(rw.RoleReader, rw.TypeFile, &rw.FileCfg{Path: "test.file.zst"}, nil).
		Add(rw.RoleReader, rw.TypeCompress, &rw.CompressCfg{Type: rw.CompressTypeZstd}, nil).
		Add(rw.RoleStarter, rw.TypeCopy, &rw.CopyCfg{BufSize: 32 * 1024}, nil).
		Add(rw.RoleWriter, rw.TypeFile, &rw.FileCfg{Path: "test.file"}, nil)

	p := New()
	p.Cfg = cfg
	tests.Init(p)
	err := runner.Init(p)
	require.NoError(t, err)

	runner.Start(p)
	require.NoError(t, p.Err())

	fmt.Println(p.RWGroups()[0].Readers()[1].Type())
}
