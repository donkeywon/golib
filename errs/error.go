package errs

type Code int

type Error interface {
	error
	Code() Code
	With(kvs ...any)
}
