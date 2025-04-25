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
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

type DaemonType string

type Daemon interface {
	runner.Runner
	plugin.Plugin
}

var (
	_daemons []DaemonType // dependencies in order
	_cfgMap  = make(map[string]any)
	_b       *Booter
)

func Boot(opt ...Option) {
	_b = New(opt...)
	err := runner.Init(_b)
	if err != nil {
		_b.Error("boot init failed", err)
		os.Exit(1)
	}
	err = v.Struct(_b)
	if err != nil {
		_b.Error("boot validate failed", err)
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

// SetLoggerLevel change log level dynamically after Boot.
func SetLoggerLevel(lvl string) {
	_b.SetLoggerLevel(lvl)
}

// RegDaemon register a Daemon creator and its config creator.
func RegDaemon(typ DaemonType, creator plugin.Creator[Daemon], cfgCreator plugin.CfgCreator[any]) {
	if slices.Contains(_daemons, typ) {
		panic("duplicate register daemon: " + typ)
	}
	plugin.RegWithCfg(typ, creator, cfgCreator)
	_daemons = append(_daemons, typ)
}

// RegCfg register additional config, cfg type must be pointer.
func RegCfg(name string, cfg any) {
	if _, exists := _cfgMap[name]; exists {
		panic("duplicate register cfg: " + name)
	}
	_cfgMap[name] = cfg
}

func UnRegDaemon(typ ...DaemonType) {
	_daemons = slices.DeleteFunc(_daemons, func(d DaemonType) bool { return slices.Contains(typ, d) })
}

type options struct {
	CfgPath        string `env:"CFG_PATH" flag-description:"config file path"        flag-long:"config"     flag-short:"c"`
	PrintVersion   bool   `flag-description:"print version info"                     flag-long:"version"    flag-short:"v"`
	EnvPrefix      string `flag-description:"define a prefix for each env field tag" flag-long:"env-prefix"`
	onConfigLoaded OnConfigLoadedFunc
}

type Booter struct {
	runner.Runner
	*options
	cfgMap     map[string]any
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
	reflects.SetFirstMatchedField(b.Runner, log.Default())

	b.cfgMap = buildCfgMap()
	b.cfgMap["log"] = b.logCfg
	b.flagParser, err = buildFlagParser(b.options, b.cfgMap)
	if err != nil {
		return errs.Wrap(err, "build flag parser failed")
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
		return errs.Wrap(err, "load cfg failed")
	}

	if b.options.onConfigLoaded != nil {
		b.options.onConfigLoaded(b.cfgMap)
	}

	err = b.validateCfg()
	if err != nil {
		return errs.Wrap(err, "validate cfg failed")
	}

	l, err := b.buildLogger()
	if err != nil {
		return errs.Wrap(err, "build logger failed")
	}
	ok := reflects.SetFirstMatchedField(b.Runner, l.WithLoggerName(b.Name()))
	if !ok {
		panic("boot set logger failed")
	}

	for name, cfg := range b.cfgMap {
		b.Debug("load config", "name", name, "cfg", cfg)
	}

	for _, daemonType := range _daemons {
		daemon := plugin.CreateWithCfg[DaemonType, Daemon](daemonType, b.cfgMap[string(daemonType)])
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
		return errs.Wrap(err, "parse env to boot options failed")
	}
	return nil
}

func (b *Booter) loadCfgFromFlag() error {
	_, err := b.flagParser.Parse()
	return err
}

func (b *Booter) loadCfgFromEnv() error {
	for path, cfg := range b.cfgMap {
		if !reflects.IsStructPointer(cfg) {
			continue
		}
		err := env.ParseWithOptions(cfg, env.Options{
			Prefix: b.options.EnvPrefix,
		})
		if err != nil {
			return errs.Wrapf(err, "parse env tocfg failed: %s", path)
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
		return errs.Errorf("cfg file not exists: %s", cfgPath)
	}

	f, err := os.ReadFile(cfgPath)
	if err != nil {
		return errs.Wrap(err, "read cfg file failed")
	}

	af, err := parser.ParseBytes(f, 0)
	if err != nil {
		return errs.Wrap(err, "parse cfg file failed")
	}

	var (
		node ast.Node
		yp   *yaml.Path
	)
	for name, cfg := range b.cfgMap {
		yp, err = yaml.PathString("$." + name)
		if err != nil {
			return errs.Wrapf(err, "invalid cfg name: %s", name)
		}
		node, err = yp.FilterFile(af)
		if errors.Is(err, yaml.ErrNotFoundNode) {
			continue
		}

		err = yaml.NodeToValue(node, cfg, yaml.CustomUnmarshaler(durationUnmarshaler))
		if err != nil {
			return errs.Wrapf(err, "unmarshal cfg fail: %s", name)
		}
	}

	return nil
}

func (b *Booter) loadCfg() error {
	return errors.Join(b.loadCfgFromFile(), b.loadCfgFromFlag(), b.loadCfgFromEnv())
}

func (b *Booter) validateCfg() error {
	for name, cfg := range b.cfgMap {
		if !reflects.IsStructPointer(cfg) {
			continue
		}
		err := v.Struct(cfg)
		if err != nil {
			return errs.Wrapf(err, "invalid cfg: %s", name)
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

func buildCfgMap() map[string]any {
	cfgMap := make(map[string]any)
	for _, daemonType := range _daemons {
		cfg := plugin.CreateCfg(daemonType)
		cfgMap[string(daemonType)] = cfg
	}
	for name, cfg := range _cfgMap {
		cfgMap[name] = cfg
	}
	return cfgMap
}

func buildFlagParser(options any, additional map[string]any) (*flags.Parser, error) {
	var err error
	parser := flags.NewParser(options, flags.Default, flags.FlagTagPrefix(consts.FlagTagPrefix))
	for name, cfg := range additional {
		if !reflects.IsStructPointer(cfg) {
			continue
		}
		_, err = parser.AddGroup(string(name)+" options", "", cfg)
		if err != nil {
			return nil, errs.Wrapf(err, "add flags fail: %s", name)
		}
	}
	return parser, nil
}
