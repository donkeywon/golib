package plugin

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPlugin is a test plugin type.
type testPlugin struct {
	Name string
	Cfg  *TestCfg
	cfg2 TestCfg
}

type TestCfg struct {
	Value int
}

// testCfgSetterPlugin implements CfgSetter[TestCfg].
type testCfgSetterPlugin struct {
	Cfg TestCfg
}

func (p *testCfgSetterPlugin) SetCfg(cfg TestCfg) {
	p.Cfg = cfg
}

// testCfgSetterPtrPlugin implements CfgSetter[*TestCfg].
type testCfgSetterPtrPlugin struct {
	Cfg *TestCfg
}

func (p *testCfgSetterPtrPlugin) SetCfg(cfg *TestCfg) {
	p.Cfg = cfg
}

// testCfgSetterAnyPlugin implements CfgSetter[any].
type testCfgSetterAnyPlugin struct {
	Cfg TestCfg
}

func (p *testCfgSetterAnyPlugin) SetCfg(cfg any) {
	if c, ok := cfg.(TestCfg); ok {
		p.Cfg = c
	}
}

// testCfgSetterAnyPtrPlugin implements CfgSetter[any] (receives *C).
type testCfgSetterAnyPtrPlugin struct {
	Cfg *TestCfg
}

func (p *testCfgSetterAnyPtrPlugin) SetCfg(cfg any) {
	if c, ok := cfg.(*TestCfg); ok {
		p.Cfg = c
	}
}

func createTestPlugin() *testPlugin {
	return &testPlugin{Name: "test"}
}

func createTestCfg() *TestCfg {
	return &TestCfg{Value: 42}
}

func createTestCfgByValue() TestCfg {
	return TestCfg{Value: 42}
}

func createCfgSetterPlugin() *testCfgSetterPlugin {
	return &testCfgSetterPlugin{}
}

func createCfgSetterPtrPlugin() *testCfgSetterPtrPlugin {
	return &testCfgSetterPtrPlugin{}
}

func createCfgSetterAnyPlugin() *testCfgSetterAnyPlugin {
	return &testCfgSetterAnyPlugin{}
}

// TestReg tests the Reg function.
func TestReg(t *testing.T) {
	t.Run("valid plugin", func(t *testing.T) {
		typ := Type("test_valid")
		Reg[*testPlugin](typ, createTestPlugin, createTestCfg)

		_, exists := _pluginCreators[typ]
		assert.True(t, exists, "plugin creator should be registered")
	})

	t.Run("nil creator panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "nil plugin creator", func() {
			Reg[*testPlugin, *TestCfg](Type("nil_creator"), nil, createTestCfg)
		})
	})

	t.Run("nil type panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "nil plugin type", func() {
			Reg[*testPlugin, *TestCfg](nil, createTestPlugin, createTestCfg)
		})
	})

	t.Run("nil cfgCreator is allowed", func(t *testing.T) {
		typ := Type("test_nil_cfg")
		assert.NotPanics(t, func() {
			Reg[*testPlugin, *TestCfg](typ, createTestPlugin, nil)
		})

		_, exists := _pluginCreators[typ]
		assert.True(t, exists)
		_, exists = _pluginCfgCreators[typ]
		assert.False(t, exists, "cfg creator should not be registered when nil")
	})

	t.Run("duplicate reg replaces previous", func(t *testing.T) {
		typ := Type("test_dup")
		Reg[*testPlugin](typ, createTestPlugin, createTestCfg)

		// Second reg should not panic (allows replacing).
		assert.NotPanics(t, func() {
			Reg[*testPlugin](typ, createTestPlugin, createTestCfg)
		})
	})
}

// TestCreateWithCfg tests the CreateWithCfg function.
func TestCreateWithCfg(t *testing.T) {
	t.Run("valid plugin", func(t *testing.T) {
		typ := Type("test_create_with_cfg")
		Reg[*testPlugin](typ, createTestPlugin, createTestCfg)

		p := CreateWithCfg[*testPlugin](typ, &TestCfg{Value: 99})
		require.NotNil(t, p)
		assert.Equal(t, "test", p.Name)
		assert.Equal(t, 99, p.Cfg.Value)
	})

	t.Run("non-existent plugin panics", func(t *testing.T) {
		assert.Panics(t, func() {
			CreateWithCfg[*testPlugin](Type("does_not_exist"), &TestCfg{})
		})
	})

	t.Run("type mismatch panics", func(t *testing.T) {
		typ := Type("test_type_mismatch")
		Reg[*testPlugin](typ, createTestPlugin, createTestCfg)

		// Registering as *testPlugin but trying to get a different type.
		// This is tricky because we registered a function returning *testPlugin,
		// but type assertion to a different interface would fail.
		// Let's test with a real different type.
		assert.Panics(t, func() {
			CreateWithCfg[*testCfgSetterPlugin](typ, &TestCfg{})
		})
	})
}

// TestCreateCfg tests the CreateCfg function.
func TestCreateCfg(t *testing.T) {
	t.Run("existent cfg", func(t *testing.T) {
		typ := Type("test_create_cfg")
		Reg[*testPlugin](typ, createTestPlugin, createTestCfg)

		cfg := CreateCfg[*TestCfg](typ)
		assert.NotNil(t, cfg)
		assert.Equal(t, 42, cfg.Value)
	})

	t.Run("non-existent cfg returns zero", func(t *testing.T) {
		cfg := CreateCfg[*TestCfg](Type("does_not_exist"))
		assert.Nil(t, cfg)
	})

	t.Run("cfg created by value (not pointer)", func(t *testing.T) {
		typ := Type("test_cfg_by_value")
		Reg[*testPlugin](typ, createTestPlugin, createTestCfgByValue)

		cfg := CreateCfg[TestCfg](typ)
		assert.Equal(t, 42, cfg.Value)
	})

	t.Run("cfg with *C return type", func(t *testing.T) {
		typ := Type("test_cfg_ptr_return")
		// Register cfgCreator that returns *TestCfg.
		Reg[*testPlugin](typ, createTestPlugin, createTestCfg)

		// CreateCfg[*TestCfg] should match the *C case.
		cfg := CreateCfg[*TestCfg](typ)
		assert.NotNil(t, cfg)
		assert.Equal(t, 42, cfg.Value)
	})

	t.Run("ptr creator with value request matches *C case", func(t *testing.T) {
		typ := Type("test_cfg_ptr_to_value")
		// Register cfgCreator that returns *TestCfg.
		Reg[*testPlugin](typ, createTestPlugin, createTestCfg)

		// CreateCfg[TestCfg] (value type) but registered creator returns *TestCfg.
		// This hits case *C: branch (line 88).
		cfg := CreateCfg[TestCfg](typ)
		assert.Equal(t, 42, cfg.Value)
	})

	t.Run("type mismatch panics", func(t *testing.T) {
		typ := Type("test_cfg_mismatch_panic")
		// Register cfgCreator that returns *TestCfg.
		Reg[*testPlugin](typ, createTestPlugin, createTestCfg)

		// CreateCfg[int] but creator returns *TestCfg → type mismatch.
		assert.Panics(t, func() {
			CreateCfg[int](typ)
		})
	})
}

// TestCreate tests the Create function.
func TestCreate(t *testing.T) {
	t.Run("valid plugin with cfg", func(t *testing.T) {
		typ := Type("test_create")
		Reg[*testPlugin](typ, createTestPlugin, createTestCfg)

		p := Create[*testPlugin, *TestCfg](typ)
		require.NotNil(t, p)
		assert.Equal(t, "test", p.Name)
		assert.Equal(t, 42, p.Cfg.Value)
	})

	t.Run("plugin without cfg creator", func(t *testing.T) {
		typ := Type("test_create_no_cfg")
		Reg[*testPlugin, *TestCfg](typ, createTestPlugin, nil)

		// This should work; there's no cfg registered.
		p := Create[*testPlugin, *TestCfg](typ)
		require.NotNil(t, p)
	})
}

// TestValidate tests the validate function.
func TestValidate(t *testing.T) {
	t.Run("nil creator panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "nil plugin creator", func() {
			validate[*testPlugin, *TestCfg](Type("test"), nil, createTestCfg)
		})
	})

	t.Run("nil type panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "nil plugin type", func() {
			validate[*testPlugin, *TestCfg](nil, createTestPlugin, createTestCfg)
		})
	})

	t.Run("valid", func(t *testing.T) {
		assert.NotPanics(t, func() {
			validate[*testPlugin, *TestCfg](Type("test"), createTestPlugin, createTestCfg)
		})
	})
}

// TestSetCfg tests the SetCfg function.
func TestSetCfg(t *testing.T) {
	t.Run("nil plugin panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "nil plugin", func() {
			SetCfg[*TestCfg](nil, &TestCfg{})
		})
	})

	t.Run("via CfgSetter interface", func(t *testing.T) {
		p := createCfgSetterPlugin()
		cfg := TestCfg{Value: 100}
		SetCfg(p, cfg)
		assert.Equal(t, 100, p.Cfg.Value)
	})

	t.Run("via *CfgSetter interface", func(t *testing.T) {
		p := createCfgSetterPtrPlugin()
		cfg := TestCfg{Value: 200}
		SetCfg(p, cfg)
		assert.NotNil(t, p.Cfg)
		assert.Equal(t, 200, p.Cfg.Value)
	})

	t.Run("via CfgSetter[any]", func(t *testing.T) {
		p := createCfgSetterAnyPlugin()
		cfg := TestCfg{Value: 300}
		SetCfg(p, cfg)
		assert.Equal(t, 300, p.Cfg.Value)
	})

	t.Run("via reflection pointer with matching field", func(t *testing.T) {
		typ := Type("test_setcfg_reflect")
		Reg[*testPlugin](typ, createTestPlugin, createTestCfg)

		p := CreateWithCfg[*testPlugin](typ, &TestCfg{Value: 555})
		assert.Equal(t, 555, p.Cfg.Value)
	})

	t.Run("via reflection with non-pointer panics", func(t *testing.T) {
		p := testPlugin{}
		assert.Panics(t, func() {
			SetCfg(p, &TestCfg{Value: 10})
		})
	})

	t.Run("via reflection no matching field panics", func(t *testing.T) {
		// testPlugin has *TestCfg field, not TestCfg value type.
		// So passing TestCfg (not *TestCfg) won't match any field.
		p := &testPlugin{}
		// Wait - the testPlugin has Cfg *TestCfg which is *TestCfg.
		// If I pass TestCfg (by value), there's no field with type TestCfg.
		// It should panic.
		assert.Panics(t, func() {
			SetCfg(p, TestCfg{Value: 10})
		})
	})

	t.Run("nil cfg value is skipped", func(t *testing.T) {
		p := &testPlugin{}
		assert.NotPanics(t, func() {
			SetCfg[*TestCfg](p, nil)
		})
		assert.Nil(t, p.Cfg)
	})
}

// TestIsNil tests the isNil helper.
func TestIsNil(t *testing.T) {
	t.Run("nil value", func(t *testing.T) {
		assert.True(t, isNil(nil, reflectValueOf(nil)))
	})

	t.Run("nil pointer", func(t *testing.T) {
		var p *TestCfg
		assert.True(t, isNil(p, reflectValueOf(p)))
	})

	t.Run("non-nil pointer", func(t *testing.T) {
		p := &TestCfg{Value: 10}
		assert.False(t, isNil(p, reflectValueOf(p)))
	})

	t.Run("nil map", func(t *testing.T) {
		var m map[string]int
		assert.True(t, isNil(m, reflectValueOf(m)))
	})

	t.Run("non-nil map", func(t *testing.T) {
		m := map[string]int{"a": 1}
		assert.False(t, isNil(m, reflectValueOf(m)))
	})

	t.Run("nil slice", func(t *testing.T) {
		var s []int
		assert.True(t, isNil(s, reflectValueOf(s)))
	})

	t.Run("non-nil slice", func(t *testing.T) {
		s := []int{1, 2}
		assert.False(t, isNil(s, reflectValueOf(s)))
	})

	t.Run("nil interface", func(t *testing.T) {
		var iface any
		assert.True(t, isNil(iface, reflectValueOf(iface)))
	})

	t.Run("nil function", func(t *testing.T) {
		var fn func()
		assert.True(t, isNil(fn, reflectValueOf(fn)))
	})

	t.Run("nil channel", func(t *testing.T) {
		var ch chan int
		assert.True(t, isNil(ch, reflectValueOf(ch)))
	})

	t.Run("int value", func(t *testing.T) {
		assert.False(t, isNil(42, reflectValueOf(42)))
	})

	t.Run("string value", func(t *testing.T) {
		assert.False(t, isNil("hello", reflectValueOf("hello")))
	})

	t.Run("struct value", func(t *testing.T) {
		assert.False(t, isNil(TestCfg{}, reflectValueOf(TestCfg{})))
	})
}

// reflectValueOf is a helper to get reflect.Value.
func reflectValueOf(v any) reflect.Value {
	if v == nil {
		return reflect.Value{}
	}
	return reflect.ValueOf(v)
}

// TestSetCfg_Reflection tests SetCfg via reflection with various scenarios.
func TestSetCfg_Reflection(t *testing.T) {
	t.Run("matching exported field", func(t *testing.T) {
		p := &testPlugin{}
		SetCfg(p, &TestCfg{Value: 777})
		assert.Equal(t, 777, p.Cfg.Value)
	})

	t.Run("field match by unexported struct field", func(t *testing.T) {
		// testPlugin has cfg2 TestCfg (unexported). CanSet() returns false.
		// So it shouldn't be matched.
		p := &testPlugin{}
		// This should match Cfg *TestCfg, not cfg2.
		SetCfg(p, &TestCfg{Value: 888})
		assert.Equal(t, 888, p.Cfg.Value)
	})
}

// TestErrVariables tests the error variables.
func TestErrVariables(t *testing.T) {
	assert.NotNil(t, ErrInvalidPluginCreator)
	assert.NotNil(t, ErrInvalidPluginCfgCreator)
	assert.Equal(t, "invalid plugin creator", ErrInvalidPluginCreator.Error())
	assert.Equal(t, "invalid plugin cfg creator", ErrInvalidPluginCfgCreator.Error())
}
