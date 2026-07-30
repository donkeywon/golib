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
	"time"

	"github.com/donkeywon/golib/buildinfo"
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
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type DaemonType string

type Daemon interface {
	runner.Runner
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

func Stop(ctx context.Context) error {
	return runner.Stop(ctx, _b)
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
	if !reflects.IsPointer(cfg) {
		panic(fmt.Sprintf("cfg type must be pointer: %s/%T", name, cfg))
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

	loggerCfgKey  string
	loggerCreator logs.Creator
	envPrefix     string
	onCreated     map[DaemonType]OnCreatedFunc
}

func createOptions() *options {
	return &options{
		onCreated:     make(map[DaemonType]OnCreatedFunc),
		loggerCfgKey:  "log",
		loggerCreator: &logs.RotateLoggerCreator{Filepath: "stderr"},
	}
}

type booter struct {
	runner.Base
	*options

	cfgMap     map[string]any
	flagParser *flags.Parser

	daemonsMap map[DaemonType]Daemon
	l          zerolog.Logger
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

	if b.options.loggerCfgKey == "" {
		return errs.New("empty logger cfg key")
	}
	if b.options.loggerCreator == nil {
		return errs.New("nil logger creator")
	}

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
			"version:"+buildinfo.Version+"\n"+
				"build_time:"+buildinfo.BuildTime+"\n"+
				"commit_time:"+buildinfo.CommitTime.Local().Format(time.DateTime)+"\n"+
				"revision:"+buildinfo.Revision+"\n"+
				"go:"+runtime.Version()+"\n"+
				"arch:"+runtime.GOARCH+"\n")
		os.Exit(0)
	}

	err = b.loadCfg()
	if err != nil {
		return errs.Wrap(err, "load cfg failed")
	}

	err = b.validateCfg()
	if err != nil {
		return errs.Wrap(err, "validate cfg failed")
	}

	b.l, err = b.options.loggerCreator.Create()
	if err != nil {
		return errs.Wrap(err, "create logger failed")
	}

	b.l.Info().Str("version", buildinfo.Version).Str("build_time", buildinfo.BuildTime).Str("revision", buildinfo.Revision).Time("commit_time", buildinfo.CommitTime).Msg("init")

	for name, cfg := range b.cfgMap {
		b.l.Debug().Str("name", name).Any("cfg", cfg).Msg("load config")
	}

	b.createDaemons(ctx)
	err = b.initDaemons(b.l.WithContext(ctx))
	if err != nil {
		return errs.Wrap(err, "init daemons failed")
	}

	return nil
}

func (b *booter) Start(ctx context.Context) error {
	var cancel context.CancelCauseFunc
	ctx, cancel = context.WithCancelCause(ctx)
	defer cancel(errCanceled)

	var errg *errgroup.Group
	errg, ctx = errgroup.WithContext(ctx)

	for _, daemonType := range _daemonTypes {
		daemon := b.daemonsMap[daemonType]
		errg.Go(func() error {
			ctx = b.l.With().Str("daemon", string(daemonType)).Logger().WithContext(ctx)
			e := runner.Start(ctx, daemon)
			if errors.Is(e, context.Canceled) && errors.Is(context.Cause(ctx), errCanceled) {
				return nil
			}
			if e != nil {
				return errs.Wrapf(e, "daemon failed: %s", daemonType)
			}

			select {
			case <-b.Stopping():
				return nil
			default:
			}
			b.l.Error().Str("daemon", string(daemonType)).Msg("daemon done, should not happen")
			e = errs.Errorf("daemon done, should not happen: %s", daemonType)
			return e
		})
	}

	termSigCh := make(chan os.Signal, 1)
	signal.Notify(termSigCh, signals.TermSignals...)
	defer signal.Stop(termSigCh)

	intSigCh := make(chan os.Signal, 1)
	signal.Notify(intSigCh, signals.IntSignals...)
	defer signal.Stop(intSigCh)

	select {
	case sig := <-termSigCh:
		b.l.Info().Str("signal", sig.String()).Msg("received signal")
		go func() {
			e := runner.Stop(ctx, b)
			if e != nil {
				b.l.Error().Err(e).Msg("stop booter failed")
			}
		}()
	case sig := <-intSigCh:
		b.l.Info().Str("signal", sig.String()).Msg("received signal")
		cancel(errCanceled)
	case <-b.Stopping():
		b.l.Info().Msg("stopping")
	}

	return errg.Wait()
}

func (b *booter) Stop(ctx context.Context) error {
	allErr := make([]error, 0, len(_daemonTypes))
	for _, typ := range slices.Backward(_daemonTypes) {
		select {
		case <-ctx.Done():
			return errors.Join(append(allErr, ctx.Err())...)
		default:
			err := runner.StopAndWait(ctx, b.daemonsMap[typ])
			if err != nil {
				allErr = append(allErr, errs.Wrapf(err, "stop daemon failed: %s", typ))
			}
		}
	}
	return errors.Join(allErr...)
}

func (b *booter) createDaemons(ctx context.Context) {
	for _, daemonType := range _daemonTypes {
		cfg := b.cfgMap[string(daemonType)]
		daemon := plugin.CreateWithCfg[Daemon](daemonType, reflect.ValueOf(cfg).Elem().Interface())
		b.daemonsMap[daemonType] = daemon

		onCreated := b.options.onCreated[daemonType]
		if onCreated != nil {
			onCreated(ctx)
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
		return nil
	}

	if !paths.FileExist(cfgPath) {
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
		if !reflects.IsPointer(cfg) {
			cfgMap[string(daemonType)] = &cfg
		} else {
			cfgMap[string(daemonType)] = cfg
		}
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
