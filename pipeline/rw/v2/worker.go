package v2

import (
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
	"io"
)

type WorkerType string

type Worker interface {
	runner.Runner
	plugin.Plugin

	WrapReader(io.Reader)
	WrapWriter(io.Writer)
}

type BaseWorker struct {
	runner.Runner
}
