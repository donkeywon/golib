package test

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
)

func init() {
	debugRunner.SetCtx(context.Background())
	reflects.Set(debugRunner.Runner, l)
}

// DebugInherit for test only.
func DebugInherit(to runner.Runner) {
	to.WithLoggerFrom(debugRunner)
	to.SetCtx(debugRunner.Ctx())
}
