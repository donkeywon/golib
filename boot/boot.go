package boot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"slices"
	"strings"

	"github.com/donkeywon/golib/buildinfo"
	"github.com/donkeywon/golib/consts"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/logs"
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

	errCanceled = errors.New("canceled")
)

func Boot(ctx context.Context, opt ...Option) {
	_b = create(opt...)
	err := runner.Init(ctx, _b)
	if err != nil {
		panic(errs.Wrap(err, "boot init failed"))
	}
	err = runner.Start(ctx, _b)
	if err != nil {
		panic(errs.Wrap(err, "error occurred"))
	}
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
		panic(fmt.Errorf("daemon %s is not type of %s", typ, reflect.TypeFor[D]()))
	}
	return dd
}

type options struct {
	CfgPath      string `env:"CFG_PATH" description:"config file path"   long:"config"  short:"c"`
	PrintVersion bool   `               description:"print version info" long:"version" short:"v"`

	loggerCfgKey   string
	loggerCreator  logs.Creator
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
	runner.Base
	*options

	cfgMap     map[string]any
	flagParser *flags.Parser

	daemonsMap map[DaemonType]Daemon
	errg       *errgroup.Group
	l          *slog.Logger
}

func create(opt ...Option) *booter {
	b := &booter{
		options:    createOptions(),
		daemonsMap: make(map[DaemonType]Daemon, len(_daemonTypes)),
	}

	for _, o := range opt {
		o(b)
	}

	return b
}

func (b *booter) Init(ctx context.Context) error {
	var err error

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

	b.l, err = b.options.loggerCreator.Create()
	if err != nil {
		return errs.Wrap(err, "create logger failed")
	}

	b.l.Info("init", "version", buildinfo.Version, "build_time", buildinfo.BuildTime, "revision", buildinfo.Revision, "commit_time", buildinfo.CommitTime)

	for name, cfg := range b.cfgMap {
		b.l.Debug("load config", "name", name, "cfg", cfg)
	}

	b.createDaemons()
	err = b.initDaemons(ctx)
	if err != nil {
		return errs.Wrap(err, "init daemons failed")
	}

	return nil
}

func (b *booter) Start(ctx context.Context) error {
	var cancel context.CancelCauseFunc
	ctx, cancel = context.WithCancelCause(ctx)
	defer cancel(errCanceled)

	ctx = logs.CtxWith(ctx, b.l)
	var errg *errgroup.Group
	errg, ctx = errgroup.WithContext(ctx)

	for _, daemonType := range _daemonTypes {
		daemon := b.daemonsMap[daemonType]
		errg.Go(func() error {
			e := runner.Start(ctx, daemon)
			if errors.Is(e, errCanceled) {
				return nil
			}
			if e != nil {
				b.l.Error("daemon failed", "err", e, "daemon", daemonType)
			} else {
				b.l.Error("daemon done, should not happen", "daemon", daemonType)
				e = errs.Errorf("daemon done, should not happen: %s", daemonType)
			}
			return e
		})
	}

	termSigCh := make(chan os.Signal, 1)
	signal.Notify(termSigCh, signals.TermSignals...)

	intSigCh := make(chan os.Signal, 1)
	signal.Notify(intSigCh, signals.IntSignals...)

	select {
	case sig := <-termSigCh:
		b.l.Info("received signal, exit", "signal", sig.String())
		go runner.Stop(ctx, b)
	case sig := <-intSigCh:
		b.l.Info("received signal, exit", "signal", sig.String())
		cancel(errCanceled)
	case <-b.Stopping():
		b.l.Info("exit due to stopping")
	}

	daemonErr := b.errg.Wait()
	if errors.Is(daemonErr, errCanceled) {
		b.l.Info("all daemon done")
		return nil
	}

	return daemonErr
}

func (b *booter) Stop(ctx context.Context) error {
	for i := len(_daemonTypes) - 1; i >= 0; i-- {
		runner.Stop(ctx, b.daemonsMap[_daemonTypes[i]])
	}
	return nil
}

func (b *booter) createDaemons() {
	for _, daemonType := range _daemonTypes {
		daemon := plugin.CreateWithCfg[Daemon](daemonType, b.cfgMap[string(daemonType)])
		b.daemonsMap[daemonType] = daemon

		onCreated := b.options.onCreated[daemonType]
		if onCreated != nil {
			onCreated()
		}
	}
}

func (b *booter) initDaemons(ctx context.Context) error {
	var err error
	for _, daemonType := range _daemonTypes {
		daemon := b.daemonsMap[daemonType]
		err = runner.Init(ctx, daemon)
		if err != nil {
			return errs.Wrapf(err, "init daemon failed: %s", daemonType)
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

func (b *booter) buildCfgMap() (map[string]any, []string) {
	cfgKeys := make([]string, 0, len(_daemonTypes)+len(_additionalCfgKeys)+1)
	cfgKeys = append(cfgKeys, b.options.loggerCfgKey)

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
	cfgMap[b.options.loggerCfgKey] = b.options.loggerCreator
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

		namespace := strings.ReplaceAll(name, ".", "_")
		g, err = parser.AddGroup(cases.Title(language.English).String(namespace)+" Options", "", cfgMap[name])
		if err != nil {
			return nil, errs.Wrapf(err, "add flags failed: %s", namespace)
		}
		g.Namespace = namespace
		if base.envPrefix != "" {
			g.EnvNamespace = strings.ToUpper(base.envPrefix + parser.EnvNamespaceDelimiter + namespace)
		} else {
			g.EnvNamespace = strings.ToUpper(namespace)
		}
	}

	return parser, nil
}
