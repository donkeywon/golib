package boot

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
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
	"github.com/donkeywon/golib/util/paths"
	"github.com/donkeywon/golib/util/reflects"
	"github.com/donkeywon/golib/util/signals"
	"github.com/donkeywon/golib/util/v"
	"github.com/goccy/go-yaml"
)

type DaemonType string

type Daemon interface {
	runner.Runner
	plugin.Plugin
}

var (
	_daemons []DaemonType // dependencies in order
	_cfgMap  = make(map[string]interface{})
	_b       *Booter
)

func Boot(opt ...Option) {
	_b = New(opt...)
	err := runner.Init(_b)
	if err != nil {
		_b.Error("boot init fail", err)
		os.Exit(1)
	}
	err = v.Struct(_b)
	if err != nil {
		_b.Error("boot validate fail", err)
		os.Exit(1)
	}
	runner.StartBG(_b)
	<-_b.Done()
	err = _b.Err()
	if err != nil {
		_b.Error("error occurred", err)
		os.Exit(1)
	}
}

func SetLoggerLevel(lvl string) {
	_b.SetLoggerLevel(lvl)
}

// RegDaemon register a Daemon creator and its config creator.
func RegDaemon(typ DaemonType, creator plugin.Creator, cfgCreator plugin.CfgCreator) {
	if slices.Contains(_daemons, typ) {
		return
	}
	plugin.RegisterWithCfg(typ, creator, cfgCreator)
	_daemons = append(_daemons, typ)
}

// RegCfg register additional config, cfg type must be pointer.
func RegCfg(name string, cfg interface{}) {
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
	cfgMap     map[DaemonType]interface{}
	logCfg     *log.Cfg
	flagParser *flags.Parser
}

func New(opt ...Option) *Booter {
	b := &Booter{
		Runner:  runner.Create("boot"),
		logCfg:  log.NewCfg(),
		options: &options{},
	}

	for _, o := range opt {
		o(b)
	}

	return b
}

func (b *Booter) Init() error {
	var err error

	// use default logger as temp logger
	reflects.SetFirst(b.Runner, log.Default())

	b.cfgMap = buildCfgMap()
	b.cfgMap["log"] = b.logCfg
	b.flagParser, err = buildFlagParser(b.options, b.cfgMap)
	if err != nil {
		return errs.Wrap(err, "build flag parser fail")
	}

	err = b.loadOptions()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			os.Exit(0)
		}

		os.Exit(1)
	}
	if b.options.PrintVersion {
		fmt.Fprint(os.Stdout,
			"Version:"+buildinfo.Version+"\n"+
				"BuildTime:"+buildinfo.BuildTime+"\n"+
				"CommitTime:"+buildinfo.CommitTime+"\n"+
				"Revision:"+buildinfo.Revision+"\n"+
				"GoVersion:"+runtime.Version()+"\n"+
				"Arch:"+runtime.GOARCH+"\n")
		os.Exit(0)
	}

	err = b.loadCfg()
	if err != nil {
		return errs.Wrap(err, "load cfg fail")
	}

	err = b.validateCfg()
	if err != nil {
		return errs.Wrap(err, "validate cfg fail")
	}

	l, err := b.buildLogger()
	if err != nil {
		return errs.Wrap(err, "build logger fail")
	}
	ok := reflects.SetFirst(b.Runner, l.WithLoggerName(b.Name()))
	if !ok {
		panic("boot set logger fail")
	}

	for name, cfg := range b.cfgMap {
		b.Debug("load config", "name", name, "cfg", cfg)
	}

	for _, daemonType := range _daemons {
		daemon, isDaemon := plugin.CreateWithCfg(daemonType, b.cfgMap[daemonType]).(Daemon)
		if !isDaemon {
			return errs.Errorf("plugin %+v is not a Daemon", daemonType)
		}
		b.AppendRunner(daemon)
	}

	return b.Runner.Init()
}

func (b *Booter) Start() error {
	b.Info("starting", "version", buildinfo.Version, "build_time", buildinfo.BuildTime, "revision", buildinfo.Revision)

	termSigCh := make(chan os.Signal, 1)
	signal.Notify(termSigCh, signals.TermSignals...)

	intSigCh := make(chan os.Signal, 1)
	signal.Notify(intSigCh, signals.IntSignals...)

	select {
	case sig := <-termSigCh:
		b.Info("received signal, exit", "signal", sig.String())
		go runner.StopAndWait(b)
		<-b.StopDone()
	case sig := <-intSigCh:
		b.Info("received signal, exit", "signal", sig.String())
		b.Cancel()
		<-b.StopDone()
	case <-b.Stopping():
		b.Info("exit due to stopping")
	}

	return nil
}

func (b *Booter) OnChildDone(child runner.Runner) error {
	select {
	case <-b.Stopping():
		return nil
	case <-b.Ctx().Done():
		return nil
	default:
		err := child.Err()
		if err != nil {
			b.Error("daemon exit abnormally", err, "daemon", child.Name())
			runner.Stop(b)
		} else {
			b.Error("daemon exit, should not happen", nil, "daemon", child.Name())
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
		return errs.Wrap(err, "parse env to boot options fail")
	}
	return nil
}

func (b *Booter) loadCfgFromFlag() error {
	_, err := b.flagParser.Parse()
	return err
}

func (b *Booter) loadCfgFromEnv() error {
	for daemonType, cfg := range b.cfgMap {
		if !reflects.IsStructPointer(cfg) {
			continue
		}
		err := env.ParseWithOptions(cfg, env.Options{
			Prefix: b.options.EnvPrefix,
		})
		if err != nil {
			return errs.Wrapf(err, "parse env to daemon(%s) cfg fail", daemonType)
		}
	}
	return nil
}

func (b *Booter) loadCfgFromFile() error {
	cfgPath := b.options.CfgPath
	if cfgPath == "" {
		cfgPath = consts.CfgPath
		if !paths.FileExist(cfgPath) {
			return nil
		}
	} else if !paths.FileExist(cfgPath) {
		return errs.Errorf("config file not exists: %s", cfgPath)
	}

	f, err := os.ReadFile(cfgPath)
	if err != nil {
		return errs.Wrap(err, "read config file fail")
	}

	fileCfgMap := make(map[DaemonType]interface{})
	err = yaml.UnmarshalWithOptions(f, &fileCfgMap, yaml.CustomUnmarshaler(durationUnmarshaler))
	if err != nil {
		return errs.Wrap(err, "unmarshal config file fail")
	}

	for daemonType, cfg := range b.cfgMap {
		v, exists := fileCfgMap[daemonType]
		if !exists {
			continue
		}

		// v is map[string]interface{}
		bs, _ := yaml.Marshal(v)
		err = yaml.Unmarshal(bs, cfg)
		if err != nil {
			return errs.Wrapf(err, "unmarshal daemon %s config fail", daemonType)
		}
	}

	return nil
}

func (b *Booter) loadCfg() error {
	return errors.Join(b.loadCfgFromFile(), b.loadCfgFromFlag(), b.loadCfgFromEnv())
}

func (b *Booter) validateCfg() error {
	for s, cfg := range b.cfgMap {
		if !reflects.IsStructPointer(cfg) {
			continue
		}
		err := v.Struct(cfg)
		if err != nil {
			return errs.Wrapf(err, "invalid daemon(%s) cfg", s)
		}
	}
	return nil
}

func (b *Booter) buildLogger() (log.Logger, error) {
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

func buildCfgMap() map[DaemonType]interface{} {
	cfgMap := make(map[DaemonType]interface{})
	for _, daemonType := range _daemons {
		cfg := plugin.CreateCfg(daemonType)
		cfgMap[daemonType] = cfg
	}
	for k, cfg := range _cfgMap {
		cfgMap[DaemonType(k)] = cfg
	}
	return cfgMap
}

func buildFlagParser(data interface{}, cfgMap map[DaemonType]interface{}) (*flags.Parser, error) {
	var err error
	parser := flags.NewParser(data, flags.Default, flags.FlagTagPrefix(consts.FlagTagPrefix))
	for daemonType, cfg := range cfgMap {
		if !reflects.IsStructPointer(cfg) {
			continue
		}
		_, err = parser.AddGroup(string(daemonType)+" options", "", cfg)
		if err != nil {
			return nil, errs.Wrapf(err, "add %s flags fail", daemonType)
		}
	}
	return parser, nil
}
