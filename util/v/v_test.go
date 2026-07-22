package v_test

import (
	"context"
	"testing"

	v "github.com/donkeywon/golib/util/v"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type simpleStruct struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
}

func TestStruct(t *testing.T) {
	t.Run("valid struct", func(t *testing.T) {
		s := simpleStruct{Name: "John", Email: "john@example.com"}
		err := v.Struct(s)
		assert.NoError(t, err)
	})

	t.Run("invalid struct missing required field", func(t *testing.T) {
		s := simpleStruct{Name: "", Email: "john@example.com"}
		err := v.Struct(s)
		assert.Error(t, err)
	})

	t.Run("invalid struct bad email", func(t *testing.T) {
		s := simpleStruct{Name: "John", Email: "not-an-email"}
		err := v.Struct(s)
		assert.Error(t, err)
	})
}

func TestGlobalVars(t *testing.T) {
	// Verify global func vars are callable (non-nil)
	require.NotNil(t, v.Struct)
	require.NotNil(t, v.StructCtx)
	require.NotNil(t, v.StructFiltered)
	require.NotNil(t, v.StructFilteredCtx)
	require.NotNil(t, v.StructPartial)
	require.NotNil(t, v.StructPartialCtx)
	require.NotNil(t, v.StructExcept)
	require.NotNil(t, v.StructExceptCtx)
	require.NotNil(t, v.Var)
	require.NotNil(t, v.VarCtx)
	require.NotNil(t, v.VarWithValue)
	require.NotNil(t, v.VarWithValueCtx)

	// Simple call to StructCtx
	s := simpleStruct{Name: "Test", Email: "test@example.com"}
	err := v.StructCtx(context.Background(), s)
	assert.NoError(t, err)
}
