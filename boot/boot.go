package boot

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/donkeywon/go-flags"
	"github.com/donkeywon/golib/buildinfo"
	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/log"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util"
	"github.com/goccy/go-yaml"
	"go.uber.org/zap"
)

const FlagTagPrefix = "flag-"

type SvcType string

type Svc interface {
	runner.Runner
	plugin.Plugin
}

func Boot(opt ...Option) {
	b := New(opt...)
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
	_svcs   []SvcType // dependencies in order
	_cfgMap = make(map[string]interface{})
)

// RegisterSvc register a Svc creator and its config creator.
func RegisterSvc(typ SvcType, creator plugin.Creator, cfgCreator plugin.CfgCreator) {
	if slices.Contains(_svcs, typ) {
		return
	}
	plugin.Register(typ, creator)
	plugin.RegisterCfg(typ, cfgCreator)
	_svcs = append(_svcs, typ)
}

// RegisterCfg register additional config, cfg type must be pointer.
func RegisterCfg(name string, cfg interface{}) {
	if _, exists := _cfgMap[name]; exists {
		return
	}
	_cfgMap[name] = cfg
}

type options struct {
	CfgPath      string `env:"CFG_PATH" flag-description:"config file path"        flag-long:"config"     flag-short:"c"`
	PrintVersion bool   `flag-description:"print version info"                     flag-long:"version"    flag-short:"v"`
	EnvPrefix    string `flag-description:"define a prefix for each env field tag" flag-long:"env-prefix"`
}

type Booter struct {
	runner.Runner
	*options
	extraOpts  []Option
	cfgMap     map[SvcType]interface{}
	logCfg     *log.Cfg
	flagParser *flags.Parser
}

func New(opt ...Option) *Booter {
	b := &Booter{
		Runner:    runner.Create("boot"),
		logCfg:    log.NewCfg(),
		extraOpts: opt,
		options:   &options{},
	}

	for _, o := range opt {
		o.apply(b)
	}

	return b
}

func (b *Booter) Init() error {
	var err error

	// use default logger as temp logger
	util.ReflectSet(b.Runner, log.Default())

	b.cfgMap = buildCfgMap()
	b.flagParser, err = buildFlagParser(b.options, b.cfgMap)
	if err != nil {
		return errs.Wrap(err, "build flag parser fail")
	}

	b.cfgMap["log"] = b.logCfg
	_, err = b.flagParser.AddGroup("log options", "", b.logCfg)
	if err != nil {
		return err
	}

	err = b.loadOptions()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			os.Exit(0)
		}
		return err
	}
	if b.options.PrintVersion {
		fmt.Println("version:" + buildinfo.Version)
		fmt.Println("githash:" + buildinfo.GitHash)
		fmt.Println("buildstamp:" + buildinfo.BuildStamp)
		os.Exit(0)
	}

	err = b.loadCfg()
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

func (b *Booter) loadOptions() error {
	_, err := b.flagParser.Parse()
	if err != nil {
		return err
	}

	err = env.ParseWithOptions(b.options, env.Options{
		Prefix: b.options.EnvPrefix,
	})
	if err != nil {
		return err
	}
	return nil
}

func (b *Booter) loadCfgFromFlag() error {
	_, err := b.flagParser.Parse()
	return err
}

func (b *Booter) loadCfgFromEnv() error {
	for _, cfg := range b.cfgMap {
		if !util.IsStructPointer(cfg) {
			continue
		}
		err := env.ParseWithOptions(cfg, env.Options{
			Prefix: b.options.EnvPrefix,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Booter) loadCfgFromFile() error {
	cfgPath := b.options.CfgPath
	if cfgPath == "" {
		cfgPath = consts.CfgPath
		if !util.FileExist(cfgPath) {
			return nil
		}
	} else if !util.FileExist(cfgPath) {
		return errs.Errorf("config file not exists: %s", cfgPath)
	}

	f, err := os.ReadFile(cfgPath)
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
	return errors.Join(b.loadCfgFromFile(), b.loadCfgFromFlag(), b.loadCfgFromEnv())
}

func (b *Booter) validateCfg() error {
	for s, cfg := range b.cfgMap {
		if !util.IsStructPointer(cfg) {
			continue
		}
		err := util.V.Struct(cfg)
		if err != nil {
			return errs.Wrapf(err, "invalid svc(%s) cfg", s)
		}
	}
	return nil
}

func (b *Booter) buildLogger() (*zap.Logger, error) {
	return b.logCfg.Build()
}

func durationUnmarshaler(d *time.Duration, b []byte) error {
	tmp, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}

	*d = tmp
	return nil
}

func buildCfgMap() map[SvcType]interface{} {
	cfgMap := make(map[SvcType]interface{})
	for _, svcType := range _svcs {
		cfg := plugin.CreateCfg(svcType)
		cfgMap[svcType] = cfg
	}
	for k, cfg := range _cfgMap {
		cfgMap[SvcType(k)] = cfg
	}
	return cfgMap
}

func buildFlagParser(data interface{}, cfgMap map[SvcType]interface{}) (*flags.Parser, error) {
	var err error
	parser := flags.NewParser(data, flags.Default, flags.FlagTagPrefix(FlagTagPrefix))
	for svcType, cfg := range cfgMap {
		if !util.IsStructPointer(cfg) {
			continue
		}
		_, err = parser.AddGroup(string(svcType)+" service options", "", cfg)
		if err != nil {
			return nil, err
		}
	}
	return parser, nil
}
