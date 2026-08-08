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
	"syscall"
	"time"

	"github.com/donkeywon/golib/buildinfo"
	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/paths"
	"github.com/donkeywon/golib/util/reflects"
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
	Run(context.Context) error
}

type initializer interface {
	Init(context.Context) error
}

type daemonInfo struct {
	d      Daemon
	ctx    context.Context
	cancel context.CancelCauseFunc
	done   chan struct{}
}

type signalError struct {
	signal os.Signal
}

func (s signalError) Error() string {
	return s.signal.String()
}

var (
	_daemonTypes       []DaemonType // dependencies in order
	_additionalCfgKeys []string
	_additionalCfgMap  = make(map[string]any)
	_daemonsMap        map[DaemonType]*daemonInfo

	errCanceled = errors.New("canceled")
)

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
	d, exists := _daemonsMap[typ]
	if !exists {
		panic(fmt.Errorf("daemon %s not exists, register first or get after created", typ))
	}
	dd, ok := d.d.(D)
	if !ok {
		panic(fmt.Errorf("daemon %s is not type of %s", typ, reflect.TypeFor[D]()))
	}
	return dd
}

func Boot(opts ...Option) {
	options, cfgMap := parseFlagsAndLoadCfg(opts...)
	l := buildLogger(&options)
	necessaryLog(&l, cfgMap)

	ctx, cancel := context.WithCancelCause(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-sigCh
		cancel(signalError{s})
	}()

	var err error
	_daemonsMap, err = createDaemons(ctx, cfgMap, &options, &l)
	if err != nil {
		if signaled, signal := isSignaled(ctx); signaled {
			l.Info().Str("signal", signal.String()).Err(err).Msg("signaled")
			os.Exit(0)
		}
		l.Error().Err(err).Msg("create daemons failed")
		os.Exit(1)
	}
	err = initDaemons(ctx, _daemonsMap, &l)
	if err != nil {
		if signaled, signal := isSignaled(ctx); signaled {
			l.Info().Str("signal", signal.String()).Err(err).Msg("signaled")
			os.Exit(0)
		}
		l.Error().Err(err).Msg("init daemons failed")
		os.Exit(1)
	}
	err = runDaemons(ctx, _daemonsMap, &l)
	if err != nil {
		l.Error().Err(err).Msg("run daemons failed")
		os.Exit(1)
	}
}

func isSignaled(ctx context.Context) (bool, os.Signal) {
	if serr, ok := errors.AsType[signalError](context.Cause(ctx)); ok {
		return true, serr.signal
	}
	return false, nil
}

func parseFlagsAndLoadCfg(opts ...Option) (options, map[string]any) {
	options := createOptions(opts...)

	if options.loggerCfgKey == "" {
		panic("empty logger cfg key")
	}
	if options.loggerCreator == nil {
		panic("nil logger creator")
	}

	cfgMap, cfgKeys := buildCfgMap(&options)
	flagParser, err := buildFlagParser(&options, cfgMap, cfgKeys)
	if err != nil {
		panic(errs.Wrap(err, "build flag parser failed"))
	}

	_, err = flagParser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			os.Exit(0)
		}

		// flag parser output content
		os.Exit(1)
	}

	if options.PrintVersion {
		fmt.Fprint(os.Stdout,
			"version:"+buildinfo.Version+"\n"+
				"build_time:"+buildinfo.BuildTime+"\n"+
				"commit_time:"+buildinfo.CommitTime.Local().Format(time.DateTime)+"\n"+
				"revision:"+buildinfo.Revision+"\n"+
				"go:"+runtime.Version()+"\n"+
				"arch:"+runtime.GOARCH+"\n")
		os.Exit(0)
	}

	err = loadCfgFromFile(&options, cfgMap)
	if err != nil {
		panic(errs.Wrap(err, "load cfg from file failed"))
	}

	err = loadCfgFromFlagsAndEnv(flagParser)
	if err != nil {
		panic(errs.Wrap(err, "load cfg from flags and env failed"))
	}

	err = validateCfg(cfgMap)
	if err != nil {
		panic(errs.Wrap(err, "validate cfg failed"))
	}

	return options, cfgMap
}

func buildLogger(options *options) zerolog.Logger {
	l, err := options.loggerCreator.Create()
	if err != nil {
		panic(errs.Wrap(err, "create logger failed"))
	}
	defaultContextLogger := l.With().Bool("logger_not_in_ctx", true).Logger()
	zerolog.DefaultContextLogger = &defaultContextLogger
	return l
}

func necessaryLog(l *zerolog.Logger, cfgMap map[string]any) {
	l.Info().Str("version", buildinfo.Version).Str("build_time", buildinfo.BuildTime).Str("revision", buildinfo.Revision).Time("commit_time", buildinfo.CommitTime).Msg("init")
	for name, cfg := range cfgMap {
		l.Debug().Str("name", name).Any("cfg", cfg).Msg("load config")
	}
}

func createDaemons(ctx context.Context, cfgMap map[string]any, options *options, l *zerolog.Logger) (map[DaemonType]*daemonInfo, error) {
	daemonsMap := make(map[DaemonType]*daemonInfo, len(_daemonTypes))
	for _, daemonType := range _daemonTypes {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		cfg := cfgMap[string(daemonType)]
		d := plugin.CreateWithCfg[Daemon](daemonType, reflect.ValueOf(cfg).Elem().Interface())
		daemonsMap[daemonType] = &daemonInfo{
			d:    d,
			done: make(chan struct{}),
		}

		dctx := l.With().Str("daemon", string(daemonType)).Logger().WithContext(ctx)
		onCreated := options.onCreated[daemonType]
		if onCreated != nil {
			onCreated(dctx)
		}
	}
	return daemonsMap, nil
}

func initDaemons(ctx context.Context, daemonsMap map[DaemonType]*daemonInfo, l *zerolog.Logger) error {
	var err error
	for _, daemonType := range _daemonTypes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		dctx := l.With().Str("daemon", string(daemonType)).Logger().WithContext(ctx)
		di := daemonsMap[daemonType]
		if initer, ok := di.d.(initializer); ok {
			err = initer.Init(dctx)
			if err != nil {
				return errs.Wrapf(err, "init daemon failed: %s", daemonType)
			}
		}
	}
	return nil
}

func runDaemons(ctx context.Context, daemonsMap map[DaemonType]*daemonInfo, l *zerolog.Logger) error {
	errg, gctx := errgroup.WithContext(context.Background())
	for _, daemonType := range _daemonTypes {
		di := daemonsMap[daemonType]
		di.ctx, di.cancel = context.WithCancelCause(gctx)
		di.ctx = l.With().Str("daemon", string(daemonType)).Logger().WithContext(di.ctx)

		errg.Go(func() error {
			defer close(di.done)
			defer di.cancel(errCanceled)

			e := di.d.Run(di.ctx)
			if errors.Is(context.Cause(di.ctx), errCanceled) {
				l.Info().Str("daemon", string(daemonType)).Err(e).Msg("daemon canceled")
				return nil
			}
			if e != nil {
				return errs.Wrapf(e, "daemon failed: %s", daemonType)
			}

			l.Error().Str("daemon", string(daemonType)).Msg("daemon done, should not happen")
			return errs.Errorf("daemon done, should not happen: %s", daemonType)
		})
	}

	go func() {
		<-ctx.Done()
		stopDaemons()
	}()

	return errg.Wait()
}

func stopDaemons() {
	for _, daemonType := range slices.Backward(_daemonTypes) {
		di := _daemonsMap[daemonType]
		di.cancel(errCanceled)
		<-di.done
	}
}

func loadCfgFromFlagsAndEnv(flagParser *flags.Parser) error {
	_, err := flagParser.Parse()
	return err
}

func loadCfgFromFile(options *options, cfgMap map[string]any) error {
	cfgPath := options.CfgPath
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
	for name, cfg := range cfgMap {
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

func validateCfg(cfgMap map[string]any) error {
	for name, cfg := range cfgMap {
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

func buildCfgMap(options *options) (map[string]any, []string) {
	cfgKeys := make([]string, 0, len(_daemonTypes)+len(_additionalCfgKeys)+1)
	cfgKeys = append(cfgKeys, options.loggerCfgKey)

	cfgMap := make(map[string]any)
	for _, daemonType := range _daemonTypes {
		cfg := plugin.CreateCfg[any](daemonType)
		if !reflects.IsPointer(cfg) {
			pv := reflect.New(reflect.TypeOf(cfg))
			pv.Elem().Set(reflect.ValueOf(cfg))
			cfgMap[string(daemonType)] = pv.Interface()
		} else {
			cfgMap[string(daemonType)] = cfg
		}
		cfgKeys = append(cfgKeys, string(daemonType))
	}
	for name, cfg := range _additionalCfgMap {
		cfgMap[name] = cfg
		cfgKeys = append(cfgKeys, name)
	}
	cfgMap[options.loggerCfgKey] = options.loggerCreator
	return cfgMap, cfgKeys
}

func buildFlagParser(options *options, cfgMap map[string]any, cfgKeys []string) (*flags.Parser, error) {
	var err error
	parser := flags.NewParser(nil, flags.Default)
	parser.NamespaceDelimiter = "-"
	parser.EnvNamespaceDelimiter = "_"

	var g *flags.Group
	g, err = parser.AddGroup("Application Options", "", options)
	if err != nil {
		return nil, errs.Wrapf(err, "add base flags failed")
	}
	g.EnvNamespace = strings.ToUpper(options.envPrefix)

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
		if options.envPrefix != "" {
			g.EnvNamespace = strings.ToUpper(options.envPrefix + parser.EnvNamespaceDelimiter + namespace)
		} else {
			g.EnvNamespace = strings.ToUpper(namespace)
		}
	}

	return parser, nil
}
