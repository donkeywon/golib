package boot

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"time"

	"github.com/donkeywon/golib/common"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/log"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util"

	"github.com/caarlos0/env/v6"
	"github.com/goccy/go-yaml"
	"go.uber.org/zap"
)

var (
	flagPrintVersion bool
	flagCfgPath      string
)

type SvcType string

type Svc interface {
	runner.Runner
	plugin.Plugin
}

const pluginLog SvcType = "log"

func init() {
	plugin.RegisterCfg(pluginLog, func() interface{} { return log.NewCfg() })

	parseFlag()
	if flagPrintVersion {
		fmt.Printf("version:%s\ngithash:%s\nbuildstamp:%s\n", common.Version, common.GitHash, common.BuildStamp)
		os.Exit(0)
	}
}

func Boot() {
	b := New(flagCfgPath)
	err := runner.Init(b)
	if err != nil {
		b.Error("boot init fail", err)
		os.Exit(1)
	}
	err = util.V.Struct(b)
	if err != nil {
		b.Error("boot validate fail", err)
		os.Exit(1)
	}
	runner.StartBG(b)
	<-b.Done()
	err = b.Err()
	if err != nil {
		b.Error("error occurred", err)
		os.Exit(1)
	}
}

var (
	_svcs []SvcType // dependencies in order
)

func RegisterSvc(typ SvcType, creator plugin.Creator, cfgCreator plugin.CfgCreator) {
	if slices.Contains(_svcs, typ) {
		return
	}
	plugin.Register(typ, creator)
	plugin.RegisterCfg(typ, cfgCreator)
	_svcs = append(_svcs, typ)
}

type Booter struct {
	runner.Runner
	cfgMap  map[SvcType]interface{}
	cancel  context.CancelFunc
	cfgPath string
}

func New(cfgPath string) *Booter {
	cfgMap := make(map[SvcType]interface{})
	for _, svcType := range _svcs {
		cfgMap[svcType] = plugin.CreateCfg(svcType)
	}
	cfgMap[pluginLog] = plugin.CreateCfg(pluginLog)

	ctx, cancel := context.WithCancel(context.Background())

	b := &Booter{
		Runner:  runner.NewBase("boot"),
		cfgPath: cfgPath,
		cfgMap:  cfgMap,
		cancel:  cancel,
	}

	// use default logger as temp logger
	util.ReflectSet(b.Runner, log.Default())

	b.SetCtx(ctx)
	return b
}

func (b *Booter) Init() error {
	err := b.loadCfg()
	if err != nil {
		return err
	}

	err = b.validateCfg()
	if err != nil {
		return err
	}

	l, err := b.buildLogger()
	if err != nil {
		return err
	}
	ok := util.ReflectSet(b.Runner, l.Named(b.Name()))
	if !ok {
		return errs.Errorf("boot set logger fail")
	}

	for name, cfg := range b.cfgMap {
		b.Info("load config", "name", name, "cfg", cfg)
	}

	for _, svcType := range _svcs {
		svc, isSvc := plugin.CreateWithCfg(svcType, b.cfgMap[svcType]).(Svc)
		if !isSvc {
			return errs.Errorf("svc %+v is not a Svc", svcType)
		}
		b.AppendRunner(svc)
	}

	return b.Runner.Init()
}

func (b *Booter) Start() error {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, signals...)

	select {
	case sig := <-signalCh:
		b.Info("received signal, exit", "signal", sig.String())
		go runner.Stop(b)
		<-b.StopDone()
	case <-b.Stopping():
		b.Info("exit due to stopping")
	}

	return nil
}

func (b *Booter) Cancel() {
	b.cancel()
}

func (b *Booter) OnChildDone(child runner.Runner) error {
	b.Info("on svc done", "svc", child.Name())
	select {
	case <-b.Stopping():
		return nil
	default:
		if child.Err() != nil {
			runner.Stop(b)
		}
	}
	return nil
}

func (b *Booter) loadCfgFromEnv() error {
	for _, cfg := range b.cfgMap {
		err := env.Parse(cfg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Booter) loadCfgFromFile() error {
	path := b.cfgPath
	if path == "" {
		path = common.CfgPath
		if !util.FileExist(path) {
			return nil
		}
	} else if !util.FileExist(path) {
		return errs.Errorf("config file not exists: %s", path)
	}

	f, err := os.ReadFile(path)
	if err != nil {
		return errs.Wrap(err, "read config file fail")
	}

	fileCfgMap := make(map[SvcType]interface{})
	err = yaml.UnmarshalWithOptions(f, &fileCfgMap, yaml.CustomUnmarshaler(durationUnmarshaler))
	if err != nil {
		return errs.Wrap(err, "unmarshal config file fail")
	}

	for svcType, cfg := range b.cfgMap {
		v, exists := fileCfgMap[svcType]
		if !exists {
			continue
		}

		// v is map[string]interface{}
		bs, _ := yaml.Marshal(v)
		err = yaml.Unmarshal(bs, cfg)
		if err != nil {
			return errs.Wrapf(err, "unmarshal svc %s config fail", svcType)
		}
	}

	return nil
}

func (b *Booter) loadCfg() error {
	return errors.Join(b.loadCfgFromFile(), b.loadCfgFromEnv())
}

func (b *Booter) validateCfg() error {
	for s, cfg := range b.cfgMap {
		err := util.V.Struct(cfg)
		if err != nil {
			return errs.Wrapf(err, "invalid svc(%s) cfg", s)
		}
	}
	return nil
}

func (b *Booter) buildLogger() (*zap.Logger, error) {
	lc, ok := b.cfgMap[pluginLog].(*log.Cfg)
	if !ok {
		return nil, errs.Errorf("unexpected log Cfg type")
	}
	return lc.Build()
}

func durationUnmarshaler(d *time.Duration, b []byte) error {
	tmp, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}

	*d = tmp
	return nil
}

func parseFlag() {
	flag.BoolVar(&flagPrintVersion, "v", false, "print version info")
	flag.StringVar(&flagCfgPath, "c", "", "specify config file path")
	flag.Parse()
}
