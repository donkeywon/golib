package tests

import (
	"context"

	"github.com/donkeywon/golib/log"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/reflects"
)

var (
	l           = log.Default()
	debugRunner = &struct {
		runner.Runner
	}{
		Runner: runner.Create("test"),
	}

	dl            = log.Default()
	defaultRunner = &struct {
		runner.Runner
	}{
		Runner: runner.Create("test"),
	}
)

func init() {
	l.SetLogLevel("debug")

	debugRunner.SetCtx(context.Background())
	reflects.SetFirstMatchedField(debugRunner.Runner, l)

	defaultRunner.SetCtx(context.Background())
	reflects.SetFirstMatchedField(defaultRunner.Runner, dl)
}

// DebugInit for test case only, with debug log level.
func DebugInit(to runner.Runner) {
	to.WithLoggerFrom(debugRunner)
	to.SetCtx(debugRunner.Ctx())
}

// Init for test case only, with info log level.
func Init(to runner.Runner) {
	to.WithLoggerFrom(defaultRunner)
	to.SetCtx(defaultRunner.Ctx())
}
