package pipeline

import (
	"testing"
	"time"

	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/cmd"
	"github.com/donkeywon/golib/util/test"
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
		To(RWRoleReader, RWTypeFile, &FileRWCfg{Path: "/root/test.file"}, nil).
		To(RWRoleStarter, RWTypeCopy, &CopyRWCfg{BufSize: 32 * 1024}, nil).
		To(RWRoleWriter, RWTypeCompress, &CompressRWCfg{Type: CompressTypeZstd, Level: CompressLevelFast}, nil).
		To(RWRoleStarter, RWTypeCmd, &cmd.Cfg{Command: []string{"bash", "-c", "cat > /root/test.file1"}}, nil)

	p := New()
	p.Cfg = cfg
	test.DebugInherit(p)
	err := runner.Init(p)
	require.NoError(t, err)

	p.rwGroups[0].LastWriter().RegisterWriteHook(func(n int, bs []byte, err error, cost int64, misc ...interface{}) error {
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
