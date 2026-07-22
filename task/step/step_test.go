package step

import (
	"testing"

	"github.com/donkeywon/golib/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeConst(t *testing.T) {
	assert.Equal(t, Type("cmd"), TypeCmd)
	assert.NotEmpty(t, string(TypeCmd))
}

func TestCfg_UnmarshalJSON(t *testing.T) {
	t.Run("valid with registered type", func(t *testing.T) {
		testType := Type("test_step_type")
		plugin.Reg(testType, func() Step { return &Base{} }, func() any { return &struct{ Name string }{} })

		data := []byte(`{"type":"test_step_type","cfg":{"Name":"hello"}}`)
		c := &Cfg{}
		err := c.UnmarshalJSON(data)
		require.NoError(t, err)
		assert.Equal(t, testType, c.Type)
		assert.NotNil(t, c.Cfg)
	})

	t.Run("valid with unregistered type", func(t *testing.T) {
		data := []byte(`{"type":"unknown_step","cfg":{}}`)
		c := &Cfg{}
		err := c.UnmarshalJSON(data)
		require.NoError(t, err)
		assert.Equal(t, Type("unknown_step"), c.Type)
		assert.Nil(t, c.Cfg)
	})

	t.Run("empty type", func(t *testing.T) {
		data := []byte(`{}`)
		c := &Cfg{}
		err := c.UnmarshalJSON(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty step type")
	})

	t.Run("non-string type", func(t *testing.T) {
		data := []byte(`{"type":123}`)
		c := &Cfg{}
		err := c.UnmarshalJSON(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid step type")
	})
}

func TestCfg_UnmarshalYAML(t *testing.T) {
	t.Run("valid with registered type", func(t *testing.T) {
		testType := Type("test_step_yaml_type")
		plugin.Reg(testType, func() Step { return &Base{} }, func() any { return &struct{ Name string }{} })

		data := []byte("type: test_step_yaml_type\ncfg:\n  Name: hello\n")
		c := &Cfg{}
		err := c.UnmarshalYAML(data)
		require.NoError(t, err)
		assert.Equal(t, testType, c.Type)
		assert.NotNil(t, c.Cfg)
	})

	t.Run("valid with unregistered type", func(t *testing.T) {
		data := []byte("type: unknown_step_yaml\ncfg:\n  Name: x\n")
		c := &Cfg{}
		err := c.UnmarshalYAML(data)
		require.NoError(t, err)
		assert.Equal(t, Type("unknown_step_yaml"), c.Type)
		assert.Nil(t, c.Cfg)
	})

	t.Run("missing type", func(t *testing.T) {
		data := []byte("cfg:\n  Name: x\n")
		c := &Cfg{}
		err := c.UnmarshalYAML(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get step type failed")
	})

	t.Run("non-string type", func(t *testing.T) {
		data := []byte("type: 42\ncfg:\n  Name: x\n")
		c := &Cfg{}
		err := c.UnmarshalYAML(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid step type")
	})
}

func TestBaseInit(t *testing.T) {
	b := &Base{}
	err := b.Init(nil) // nolint:staticcheck // testing nil context
	require.NoError(t, err)

	// Verify channels are initialized
	select {
	case <-b.Started():
		t.Fatal("Started channel should not be closed yet")
	default:
	}

	select {
	case <-b.Stopping():
		t.Fatal("Stopping channel should not be closed yet")
	default:
	}

	select {
	case <-b.Done():
		t.Fatal("Done channel should not be closed yet")
	default:
	}
}

func TestBase_StoreLoad(t *testing.T) {
	b := &Base{}
	b.Store("key1", "value1")
	v, ok := b.Load("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", v)
}

func TestBase_LoadMissing(t *testing.T) {
	b := &Base{}
	v, ok := b.Load("nonexistent")
	assert.False(t, ok)
	assert.Empty(t, v)
}

func TestBase_Delete(t *testing.T) {
	b := &Base{}
	b.Store("key", "val")
	v, ok := b.Load("key")
	assert.True(t, ok)
	assert.Equal(t, "val", v)

	b.Delete("key")
	v, ok = b.Load("key")
	assert.False(t, ok)
	assert.Empty(t, v)
}

func TestBase_Range(t *testing.T) {
	b := &Base{}
	b.Store("a", 1)
	b.Store("b", 2)
	b.Store("c", 3)

	count := 0
	b.Range(func(k string, v any) bool {
		count++
		return true
	})
	assert.Equal(t, 3, count)

	// Early stop
	count = 0
	b.Range(func(k string, v any) bool {
		count++
		return count < 2
	})
	assert.Equal(t, 2, count)
}

func TestBase_LoadOrStore(t *testing.T) {
	b := &Base{}

	// First call stores
	v, loaded := b.LoadOrStore("k", "v1")
	assert.False(t, loaded)
	assert.Equal(t, "v1", v)

	// Second call loads existing
	v, loaded = b.LoadOrStore("k", "v2")
	assert.True(t, loaded)
	assert.Equal(t, "v1", v)
}

func TestBase_LoadAndDelete(t *testing.T) {
	b := &Base{}
	b.Store("k", "val")

	v, existed := b.LoadAndDelete("k")
	assert.True(t, existed)
	assert.Equal(t, "val", v)

	v, existed = b.LoadAndDelete("k")
	assert.False(t, existed)
	assert.Empty(t, v)
}
