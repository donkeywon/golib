package v2

import "errors"

var (
	ErrWrapTwice  = errors.New("wrap twice")
	ErrWrapNil    = errors.New("wrap nil")
	ErrCannotWrap = errors.New("can not wrap")
)
