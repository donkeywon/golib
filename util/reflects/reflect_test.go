package reflects

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testStruct struct {
	A string
	B int
}

type testStructWithInterface struct {
	A any
	B int
}

func TestSetFirstMatchedField(t *testing.T) {
	t.Run("matching struct field by kind", func(t *testing.T) {
		s := &testStruct{A: "old", B: 0}
		ok := SetFirstMatchedField(s, "new")
		assert.True(t, ok)
		assert.Equal(t, "new", s.A)
		assert.Equal(t, 0, s.B)
	})

	t.Run("matching struct field int", func(t *testing.T) {
		s := &testStruct{A: "", B: 0}
		ok := SetFirstMatchedField(s, 42)
		assert.True(t, ok)
		assert.Equal(t, 42, s.B)
	})

	t.Run("struct with interface field", func(t *testing.T) {
		s := &testStructWithInterface{A: nil, B: 0}
		ok := SetFirstMatchedField(s, "hello")
		assert.True(t, ok)
		assert.Equal(t, "hello", s.A)
	})

	t.Run("non-struct input", func(t *testing.T) {
		var x int = 5
		ok := SetFirstMatchedField(&x, "value")
		assert.False(t, ok)
	})

	t.Run("non-pointer struct panics on Elem", func(t *testing.T) {
		s := testStruct{A: "old", B: 0}
		assert.Panics(t, func() {
			SetFirstMatchedField(s, "new")
		})
	})

	t.Run("no matching field kind", func(t *testing.T) {
		s := &testStruct{A: "old", B: 0}
		ok := SetFirstMatchedField(s, 3.14) // float64 doesn't match string or int
		assert.False(t, ok)
	})

	t.Run("nil pointer to struct returns false", func(t *testing.T) {
		var s *testStruct
		// nil pointer: Indirect returns zero Value (Kind=Invalid), so Kind!=Struct -> false
		ok := SetFirstMatchedField(s, "value")
		assert.False(t, ok)
	})
}

func TestGetFuncName(t *testing.T) {
	t.Run("named function", func(t *testing.T) {
		name := GetFuncName(TestGetFuncName)
		assert.Contains(t, name, "TestGetFuncName")
	})

	t.Run("anonymous function", func(t *testing.T) {
		f := func() {}
		name := GetFuncName(f)
		assert.NotEmpty(t, name)
		assert.Contains(t, name, "TestGetFuncName")
	})

	t.Run("nil value", func(t *testing.T) {
		name := GetFuncName(nil)
		assert.Equal(t, "", name)
	})

	t.Run("non-func panics", func(t *testing.T) {
		assert.Panics(t, func() {
			GetFuncName(42)
		})
	})

	t.Run("non-func string panics", func(t *testing.T) {
		assert.Panics(t, func() {
			GetFuncName("not a func")
		})
	})
}

func TestIsStructPointer(t *testing.T) {
	t.Run("pointer to struct", func(t *testing.T) {
		assert.True(t, IsStructPointer(&testStruct{}))
	})

	t.Run("non-pointer struct", func(t *testing.T) {
		assert.False(t, IsStructPointer(testStruct{}))
	})

	t.Run("pointer to int", func(t *testing.T) {
		v := 42
		assert.False(t, IsStructPointer(&v))
	})

	t.Run("nil", func(t *testing.T) {
		assert.False(t, IsStructPointer(nil))
	})

	t.Run("string", func(t *testing.T) {
		assert.False(t, IsStructPointer("hello"))
	})
}

func TestIsPointer(t *testing.T) {
	t.Run("pointer to struct", func(t *testing.T) {
		assert.True(t, IsPointer(&testStruct{}))
	})

	t.Run("pointer to int", func(t *testing.T) {
		v := 42
		assert.True(t, IsPointer(&v))
	})

	t.Run("non-pointer struct", func(t *testing.T) {
		assert.False(t, IsPointer(testStruct{}))
	})

	t.Run("nil", func(t *testing.T) {
		assert.False(t, IsPointer(nil))
	})
}

func TestIsStruct(t *testing.T) {
	t.Run("struct value", func(t *testing.T) {
		assert.True(t, IsStruct(testStruct{}))
	})

	t.Run("pointer to struct", func(t *testing.T) {
		assert.False(t, IsStruct(&testStruct{}))
	})

	t.Run("int", func(t *testing.T) {
		assert.False(t, IsStruct(42))
	})

	t.Run("nil", func(t *testing.T) {
		assert.False(t, IsStruct(nil))
	})
}

func TestIsFunc(t *testing.T) {
	t.Run("function", func(t *testing.T) {
		assert.True(t, IsFunc(func() {}))
	})

	t.Run("non-function int", func(t *testing.T) {
		assert.False(t, IsFunc(42))
	})

	t.Run("nil", func(t *testing.T) {
		assert.False(t, IsFunc(nil))
	})

	t.Run("string", func(t *testing.T) {
		assert.False(t, IsFunc("hello"))
	})
}
