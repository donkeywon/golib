package v2

import "errors"

var (
	ErrWrapTwice   = errors.New("wrap twice")
	ErrWrapNil     = errors.New("wrap nil")
	ErrNotWrapper  = errors.New("not wrapper")
	ErrInvalidWrap = errors.New("invalid wrap")
)
