package boot

import (
	"context"
	"errors"
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

type SvcType string

const pluginLog SvcType = "log"

func init() {
	plugin.RegisterCfg(pluginLog, func() interface{} { return log.NewCfg() })
}

func Run(cfgPath string) {
	b := New(cfgPath)
	err := runner.Init(b)
	if err != nil {
		panic(errs.ErrToStackString(err))
	}
	runner.StartBG(b)
	<-b.Done()
	err = b.Error()
	if err != nil {
		b.Err("error occurred", err)
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

type Boot struct {
	runner.Runner
	cfgMap  map[SvcType]interface{}
	cancel  context.CancelFunc
	cfgPath string
}

func New(cfgPath string) *Boot {
	cfgMap := make(map[SvcType]interface{})
	for _, svcType := range _svcs {
		cfgMap[svcType] = plugin.CreateCfg(svcType)
	}
	cfgMap[pluginLog] = plugin.CreateCfg(pluginLog)

	ctx, cancel := context.WithCancel(context.Background())

	b := &Boot{
		Runner:  runner.NewBase("boot"),
		cfgPath: cfgPath,
		cfgMap:  cfgMap,
		cancel:  cancel,
	}

	b.SetCtx(ctx)
	return b
}

func (b *Boot) Init() error {
	err := b.loadCfg()
	if err != nil {
		return err
	}

	l, err := b.buildLogger()
	if err != nil {
		return err
	}
	b.WithLogger(l)

	for name, cfg := range b.cfgMap {
		b.Info("load config", "name", name, "cfg", cfg)
	}

	for _, svcType := range _svcs {
		svc, ok := plugin.CreateWithCfg(svcType, b.cfgMap[svcType]).(runner.Runner)
		if !ok {
			return errs.Errorf("svc is not a Runner: %+v", svcType)
		}
		b.AppendRunner(svc)
	}

	return b.Runner.Init()
}

func (b *Boot) Start() error {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, signals...)

	sig := <-signalCh
	b.Info("received signal, exit", "signal", sig.String())
	go runner.Stop(b)
	<-b.StopDone()
	return nil
}

func (b *Boot) Cancel() {
	b.cancel()
}

func (b *Boot) OnChildDone(child runner.Runner) error {
	b.Info("on svc done", "svc", child.Name())
	select {
	case <-b.Stopping():
		return nil
	default:
		if child.Error() != nil {
			runner.Stop(b)
		}
	}
	return nil
}

func (b *Boot) loadCfgFromEnv() error {
	for _, cfg := range b.cfgMap {
		err := env.Parse(cfg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Boot) loadCfgFromFile(path string) error {
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

func (b *Boot) loadCfg() error {
	return errors.Join(b.loadCfgFromFile(b.cfgPath), b.loadCfgFromEnv())
}

func (b *Boot) buildLogger() (*zap.Logger, error) {
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
