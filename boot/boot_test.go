package boot

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/donkeywon/golib/plugin"
	"github.com/stretchr/testify/require"
)

const testDaemonType DaemonType = "testdaemon"

// testDaemonCfg 值类型,与真实 daemon 的 NewCfg() Cfg 模式一致
type testDaemonCfg struct {
	Field string `yaml:"field" json:"field"`
	Num   int    `yaml:"num" json:"num"`
}

type testDaemon struct {
	cfg testDaemonCfg
}

// SetCfg 与 httpd.SetCfg 相同模式:断言值类型
func (d *testDaemon) SetCfg(cfg any) {
	d.cfg = cfg.(testDaemonCfg)
}

func (d *testDaemon) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func regTestDaemon(t *testing.T) {
	t.Helper()
	Reg(testDaemonType, func() Daemon { return &testDaemon{} }, func() any { return testDaemonCfg{} })
	t.Cleanup(func() {
		_daemonTypes = slices.DeleteFunc(_daemonTypes, func(t DaemonType) bool { return t == testDaemonType })
	})
}

func writeTestCfg(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfg.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestBuildCfgMap_NoCfgCreator(t *testing.T) {
	typ := DaemonType("nocfgdaemon")
	Reg(typ, func() Daemon { return &testDaemon{} }, nil)
	t.Cleanup(func() {
		_daemonTypes = slices.DeleteFunc(_daemonTypes, func(t DaemonType) bool { return t == typ })
	})

	opts := createOptions()
	require.NotPanics(t, func() {
		buildCfgMap(&opts)
	}, "未注册 cfg creator 的 daemon 不应 panic(CreateCfg 返回 nil)")
}

// 验证 1:buildCfgMap 对值类型 cfg creator,cfgMap 里存的是 *testDaemonCfg 还是 *any
func TestBuildCfgMap_ValueCfg(t *testing.T) {
	regTestDaemon(t)

	opts := createOptions()
	cfgMap, _ := buildCfgMap(&opts)

	cfg := cfgMap[string(testDaemonType)]
	require.NotNil(t, cfg)
	require.IsType(t, &testDaemonCfg{}, cfg, "值类型 cfg 应以 *testDaemonCfg 存入 cfgMap,实际是 %T", cfg)
}

// 验证 2:loadCfgFromFile 从 yaml 填充后,cfgMap 里的值是否保留 testDaemonCfg 类型和内容
func TestLoadCfgFromFile_ValueCfg(t *testing.T) {
	regTestDaemon(t)

	cfgPath := writeTestCfg(t, string(testDaemonType)+":\n  field: hello\n  num: 42\n")
	opts := createOptions()
	opts.CfgPath = cfgPath
	cfgMap, _ := buildCfgMap(&opts)

	require.NoError(t, loadCfgFromFile(&opts, cfgMap))

	cfg := cfgMap[string(testDaemonType)]
	cc, ok := cfg.(*testDaemonCfg)
	require.Truef(t, ok, "加载后 cfg 应为 *testDaemonCfg,实际是 %T", cfg)
	require.Equal(t, "hello", cc.Field)
	require.Equal(t, 42, cc.Num)
}

// 验证 3:完整链路(有配置文件)——模拟 createDaemons 的 cfg 传递,SetCfg 不应 panic
func TestCreateDaemon_WithCfgFile(t *testing.T) {
	regTestDaemon(t)

	cfgPath := writeTestCfg(t, string(testDaemonType)+":\n  field: hello\n  num: 42\n")
	opts := createOptions()
	opts.CfgPath = cfgPath
	cfgMap, _ := buildCfgMap(&opts)
	require.NoError(t, loadCfgFromFile(&opts, cfgMap))

	require.NotPanics(t, func() {
		cfg := cfgMap[string(testDaemonType)]
		_ = plugin.CreateWithCfg[Daemon](testDaemonType, reflect.ValueOf(cfg).Elem().Interface())
	}, "有配置文件时创建 daemon 不应 panic")
}

// 验证 4:完整链路(无配置文件)——对照组
func TestCreateDaemon_WithoutCfgFile(t *testing.T) {
	regTestDaemon(t)

	opts := createOptions()
	cfgMap, _ := buildCfgMap(&opts)

	require.NotPanics(t, func() {
		cfg := cfgMap[string(testDaemonType)]
		_ = plugin.CreateWithCfg[Daemon](testDaemonType, reflect.ValueOf(cfg).Elem().Interface())
	}, "无配置文件时创建 daemon 不应 panic")
}
