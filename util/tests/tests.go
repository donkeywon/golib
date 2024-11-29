package tests

import (
	"context"

	"github.com/donkeywon/golib/log"
	"github.com/donkeywon/golib/runner"
	"github.com/donkeywon/golib/util/reflects"
)

var (
	l           = log.Debug()
	debugRunner = &struct {
		runner.Runner
	}{
		Runner: runner.Create(""),
	}

	dl            = log.Default()
	defaultRunner = &struct {
		runner.Runner
	}{
		Runner: runner.Create(""),
	}
)

func init() {
	debugRunner.SetCtx(context.Background())
	reflects.SetFirst(debugRunner.Runner, l)

	defaultRunner.SetCtx(context.Background())
	reflects.SetFirst(defaultRunner.Runner, dl)
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
