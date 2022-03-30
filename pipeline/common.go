package pipeline

import (
	"io"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

type closeFunc func() error

type Common interface {
	io.Closer
	runner.Runner
	plugin.Plugin[Type]
	optionApplier
}
