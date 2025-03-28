package v2

import (
	"io"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/runner"
)

type Common interface {
	io.Closer
	runner.Runner
	plugin.Plugin[Type]
	optionApplier
}

type CommonReader interface {
	Common
	Reader
}

type CommonWriter interface {
	Common
	Writer
}

type CommonReadWriter interface {
	Common
	Reader
	Writer
}

type ReadWriter interface {
	Reader
	Writer
}
