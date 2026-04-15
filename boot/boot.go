package boot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"slices"
	"strings"

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
	"github.com/jessevdk/go-flags"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type DaemonType string

type Daemon interface {
	runner.Runner
	plugin.Plugin
}

var (
	_daemonTypes       []DaemonType // dependencies in order
	_additionalCfgKeys []string
	_additionalCfgMap  = make(map[string]any)
	_b                 *booter
)

func Boot(opt ...Option) {
	_b = create(opt...)
	_b.SetCtx(context.Background())
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
	err = runner.Run(_b)
	if err != nil {
		_b.Error("error occurred", err)
		os.Exit(1)
	}
}

// SetLogLevel change log level dynamically after Boot.
func SetLogLevel(lvl string) {
	if _b == nil {
		panic("SetLogLevel must called after Boot")
	}
	_b.SetLogLevel(lvl)
}

// Reg register a Daemon creator and its config creator.
func Reg(typ DaemonType, creator plugin.Creator[Daemon], cfgCreator plugin.CfgCreator[any]) {
	if !slices.Contains(_daemonTypes, typ) {
		_daemonTypes = append(_daemonTypes, typ)
	}
	plugin.Reg(typ, creator, cfgCreator)
}

// RegCfg register additional config, cfg type must be pointer.
func RegCfg(name string, cfg any) {
	if _, exists := _additionalCfgMap[name]; exists {
		panic("duplicate register cfg: " + name)
	}
	if slices.Contains(_daemonTypes, DaemonType(name)) {
		panic("duplicate register cfg: " + name)
	}
	_additionalCfgKeys = append(_additionalCfgKeys, name)
	_additionalCfgMap[name] = cfg
}

func Get[D Daemon](typ DaemonType) D {
	d, exists := _b.daemonsMap[typ]
	if !exists {
		panic(fmt.Errorf("daemon %s not exists, register first or get after created", typ))
	}
	dd, ok := d.(D)
	if !ok {
		panic(fmt.Errorf("daemon %s is not type of %s", typ, reflect.TypeOf((*D)(nil)).Elem()))
	}
	return dd
}

type options struct {
	CfgPath        string `env:"CFG_PATH" description:"config file path"   long:"config"  short:"c"`
	PrintVersion   bool   `               description:"print version info" long:"version" short:"v"`
	envPrefix      string
	onConfigLoaded map[DaemonType]OnConfigLoadedFunc
	onCreated      map[DaemonType]OnCreatedFunc
	onInitialized  map[DaemonType]OnInitializedFunc
}

func createOptions() *options {
	return &options{
		onConfigLoaded: make(map[DaemonType]OnConfigLoadedFunc),
		onCreated:      make(map[DaemonType]OnCreatedFunc),
		onInitialized:  make(map[DaemonType]OnInitializedFunc),
	}
}

type booter struct {
	runner.Runner
	*options

	cfgMap     map[string]any
	logCfg     *log.Cfg
	flagParser *flags.Parser

	daemonsMap map[DaemonType]Daemon
	errg       *errgroup.Group
}

func create(opt ...Option) *booter {
	b := &booter{
		Runner:     runner.Create("boot"),
		logCfg:     log.NewCfg(),
		options:    createOptions(),
		daemonsMap: make(map[DaemonType]Daemon, len(_daemonTypes)),
	}

	for _, o := range opt {
		o(b)
	}

	return b
}

func (b *booter) Init() error {
	var err error

	// use default logger as temp logger
	reflects.SetFirstMatchedField(b.Runner, log.Default())

	var cfgKeys []string
	b.cfgMap, cfgKeys = b.buildCfgMap()
	b.flagParser, err = buildFlagParser(b.options, b.cfgMap, cfgKeys)
	if err != nil {
		return errs.Wrap(err, "build flag parser failed")
	}

	err = b.loadCfgFromFlags()
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

	for t, f := range b.options.onConfigLoaded {
		f(b.cfgMap[string(t)])
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

	b.Info("init", "version", buildinfo.Version, "build_time", buildinfo.BuildTime, "revision", buildinfo.Revision)

	for name, cfg := range b.cfgMap {
		b.Debug("load config", "name", name, "cfg", cfg)
	}

	var ctx context.Context
	b.errg, ctx = errgroup.WithContext(b.Ctx())
	b.createDaemons(ctx)

	err = b.initDaemons()
	if err != nil {
		return errs.Wrap(err, "init daemons failed")
	}

	err = b.Runner.Init()
	if err != nil {
		return errs.Wrap(err, "init booter failed")
	}

	return nil
}

func (b *booter) Start() error {
	for _, daemonType := range _daemonTypes {
		daemon := b.daemonsMap[daemonType]
		b.errg.Go(func() error {
			e := runner.Run(daemon)
			select {
			case <-b.Ctx().Done():
				return nil
			case <-b.Stopping():
				return nil
			default:
			}

			if e != nil {
				b.Error("daemon failed", e, "daemon", daemon.Name())
			} else {
				b.Error("daemon done, should not happen", nil, "daemon", daemon.Name())
				e = errs.Errorf("daemon %s done, should not happen", daemon.Name())
			}
			runner.Stop(b)
			b.AppendError(e)
			return e
		})
	}

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

	b.errg.Wait()
	b.Info("all daemon done")
	return nil
}

func (b *booter) Stop() error {
	select {
	case <-b.Ctx().Done():
		// Booter cancelled, all daemons are stopping now
		return nil
	default:
	}
	for i := len(_daemonTypes) - 1; i >= 0; i-- {
		runner.StopAndWait(b.daemonsMap[_daemonTypes[i]])
	}
	return nil
}

func (b *booter) createDaemons(ctx context.Context) {
	for _, daemonType := range _daemonTypes {
		daemon := plugin.CreateWithCfg[Daemon](daemonType, b.cfgMap[string(daemonType)])
		daemon.SetCtx(ctx)
		daemon.Inherit(b)
		b.daemonsMap[daemonType] = daemon

		onCreated := b.options.onCreated[daemonType]
		if onCreated != nil {
			onCreated()
		}
	}
}

func (b *booter) initDaemons() error {
	var err error
	for _, daemonType := range _daemonTypes {
		daemon := b.daemonsMap[daemonType]
		err = runner.Init(daemon)
		if err != nil {
			return errs.Wrapf(err, "init daemon %s failed", daemonType)
		}

		onInitialized := b.options.onInitialized[daemonType]
		if onInitialized != nil {
			onInitialized()
		}
	}
	return nil
}

func (b *booter) loadCfgFromFlags() error {
	_, err := b.flagParser.Parse()
	return err
}

func (b *booter) loadCfgFromFile() error {
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

		err = yaml.NodeToValue(node, cfg)
		if err != nil {
			return errs.Wrapf(err, "unmarshal cfg fail: %s", name)
		}
	}

	return nil
}

func (b *booter) loadCfg() error {
	return errors.Join(b.loadCfgFromFile(), b.loadCfgFromFlags())
}

func (b *booter) validateCfg() error {
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

func (b *booter) buildLogger() (log.Logger, error) {
	return b.logCfg.Build()
}

func (b *booter) buildCfgMap() (map[string]any, []string) {
	cfgKeys := make([]string, 0, len(_daemonTypes)+len(_additionalCfgKeys)+1)
	cfgKeys = append(cfgKeys, "log")

	cfgMap := make(map[string]any)
	for _, daemonType := range _daemonTypes {
		cfg := plugin.CreateCfg[any](daemonType)
		cfgMap[string(daemonType)] = cfg
		cfgKeys = append(cfgKeys, string(daemonType))
	}
	for name, cfg := range _additionalCfgMap {
		cfgMap[name] = cfg
		cfgKeys = append(cfgKeys, name)
	}
	cfgMap["log"] = b.logCfg
	return cfgMap, cfgKeys
}

func buildFlagParser(base *options, cfgMap map[string]any, cfgKeys []string) (*flags.Parser, error) {
	var err error
	parser := flags.NewParser(nil, flags.Default)
	parser.NamespaceDelimiter = "-"
	parser.EnvNamespaceDelimiter = "_"

	var g *flags.Group
	g, err = parser.AddGroup("Application Options", "", base)
	if err != nil {
		return nil, errs.Wrapf(err, "add base flags failed")
	}
	g.EnvNamespace = strings.ToUpper(base.envPrefix)

	for _, name := range cfgKeys {
		if !reflects.IsStructPointer(cfgMap[name]) {
			continue
		}

		g, err = parser.AddGroup(cases.Title(language.English).String(name)+" Options", "", cfgMap[name])
		if err != nil {
			return nil, errs.Wrapf(err, "add flags failed: %s", name)
		}
		g.Namespace = name
		if base.envPrefix != "" {
			g.EnvNamespace = strings.ToUpper(base.envPrefix + parser.EnvNamespaceDelimiter + name)
		} else {
			g.EnvNamespace = strings.ToUpper(name)
		}
	}

	return parser, nil
}
