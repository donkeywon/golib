package common

import (
	"errors"
)

var (
	ErrReadFromClosedReader = errors.New("read from closed reader")
	ErrWriteToClosedWriter  = errors.New("write to closed writer")
)
