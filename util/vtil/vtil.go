package vtil

import "github.com/go-playground/validator/v10"

var (
	V                 = validator.New(validator.WithRequiredStructEnabled())
	Struct            = V.Struct
	StructCtx         = V.StructCtx
	StructFiltered    = V.StructFiltered
	StructFilteredCtx = V.StructFilteredCtx
	StructPartial     = V.StructPartial
	StructPartialCtx  = V.StructPartialCtx
	StructExcept      = V.StructExcept
	StructExceptCtx   = V.StructExceptCtx
	Var               = V.Var
	VarCtx            = V.VarCtx
	VarWithValue      = V.VarWithValue
	VarWithValueCtx   = V.VarWithValueCtx
)
