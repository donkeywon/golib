package pipeline

import (
	"fmt"
	"testing"
	"time"

	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/tests"
	"github.com/stretchr/testify/require"
)

func TestGroupByStarter(t *testing.T) {
	rws := []*RWCfg{}
	g := groupRWCfgByStarter(rws)
	require.Empty(t, g)

	rws = []*RWCfg{
		{Role: RWRoleReader},
	}
	g = groupRWCfgByStarter(rws)
	require.Len(t, g, 1)
	require.Len(t, g[0], 1)

	rws = []*RWCfg{
		{Role: RWRoleReader},
		{Role: RWRoleStarter},
		{Role: RWRoleWriter},
	}
	g = groupRWCfgByStarter(rws)
	require.Len(t, g, 1)
	require.Len(t, g[0], 3)

	rws = []*RWCfg{
		{Role: RWRoleReader},
		{Role: RWRoleReader},
	}
	g = groupRWCfgByStarter(rws)
	require.Len(t, g, 1)
	require.Len(t, g[0], 2)

	rws = []*RWCfg{
		{Role: RWRoleReader},
		{Role: RWRoleReader},
		{Role: RWRoleStarter},
		{Role: RWRoleStarter},
		{Role: RWRoleWriter},
		{Role: RWRoleWriter},
		{Role: RWRoleStarter},
	}
	g = groupRWCfgByStarter(rws)
	require.Len(t, g, 3)
	require.Len(t, g[0], 3)
	require.Len(t, g[1], 3)
	require.Len(t, g[2], 1)
}

func TestMultiGroupPipeline(t *testing.T) {
	cfg := NewCfg().
		Add(RWRoleReader, RWTypeFile, &FileRWCfg{Path: "/root/test.file"}, nil).
		Add(RWRoleStarter, RWTypeCopy, &CopyRWCfg{BufSize: 32 * 1024}, nil).
		Add(RWRoleWriter, RWTypeCompress, &CompressRWCfg{Type: CompressTypeZstd, Level: CompressLevelFast}, nil).
		Add(RWRoleStarter, RWTypeCmd, &cmd.Cfg{Command: []string{"bash", "-c", "cat > /root/test.file1"}}, nil)

	p := New()
	p.Cfg = cfg
	tests.DebugInit(p)
	err := runner.Init(p)
	require.NoError(t, err)

	p.rwGroups[0].LastWriter().HookWrite(func(n int, bs []byte, err error, cost int64, misc ...interface{}) error {
		time.Sleep(time.Second)
		return nil
	})

	go func() {
		time.Sleep(time.Second * 3)
		runner.Stop(p)
	}()

	runner.Start(p)
	require.NoError(t, p.Err())
}

func TestCompress(t *testing.T) {
	cfg := NewCfg().
		Add(RWRoleReader, RWTypeFile, &FileRWCfg{Path: "test.file.zst"}, nil).
		Add(RWRoleReader, RWTypeCompress, &CompressRWCfg{Type: CompressTypeZstd}, nil).
		Add(RWRoleStarter, RWTypeCopy, &CopyRWCfg{BufSize: 32 * 1024}, nil).
		Add(RWRoleWriter, RWTypeFile, &FileRWCfg{Path: "test.file"}, nil)

	p := New()
	p.Cfg = cfg
	tests.Init(p)
	err := runner.Init(p)
	require.NoError(t, err)

	runner.Start(p)
	require.NoError(t, p.Err())

	fmt.Println(p.RWGroups()[0].Readers()[1].Type())
}
