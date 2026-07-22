package ratelimit

import (
	"testing"

	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/jsons"
	"github.com/donkeywon/golib/util/yamls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertFixedCfg asserts that cfg is FixedCfg or *FixedCfg with expected N and Burst.
func assertFixedCfg(t *testing.T, cfg any, expectedN, expectedBurst int) {
	t.Helper()
	require.NotNil(t, cfg)
	switch c := cfg.(type) {
	case FixedCfg:
		assert.Equal(t, expectedN, c.N)
		assert.Equal(t, expectedBurst, c.Burst)
	case *FixedCfg:
		assert.Equal(t, expectedN, c.N)
		assert.Equal(t, expectedBurst, c.Burst)
	default:
		t.Fatalf("expected FixedCfg or *FixedCfg, got %T", cfg)
	}
}

func TestCfg_UnmarshalJSON(t *testing.T) {
	t.Run("valid with registered type", func(t *testing.T) {
		testType := Type("test_json_type")
		plugin.Reg(testType, func() RxTxRateLimiter { return NewFixed() }, func() any { return NewFixedCfg() })

		data := []byte(`{"type":"test_json_type","cfg":{"N":100,"Burst":200}}`)
		c := &Cfg{}
		err := c.UnmarshalJSON(data)
		require.NoError(t, err)
		assert.Equal(t, testType, c.Type)
		assertFixedCfg(t, c.Cfg, 100, 200)
	})

	t.Run("valid with unregistered type", func(t *testing.T) {
		data := []byte(`{"type":"unknown_type","cfg":{"N":100}}`)
		c := &Cfg{}
		err := c.UnmarshalJSON(data)
		require.NoError(t, err)
		assert.Equal(t, Type("unknown_type"), c.Type)
		assert.Nil(t, c.Cfg)
	})

	t.Run("empty type", func(t *testing.T) {
		data := []byte(`{}`)
		c := &Cfg{}
		err := c.UnmarshalJSON(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty ratelimiter type")
	})

	t.Run("invalid type field", func(t *testing.T) {
		data := []byte(`{"type":123}`)
		c := &Cfg{}
		err := c.UnmarshalJSON(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid ratelimiter type")
	})

	t.Run("no cfg field", func(t *testing.T) {
		testType := Type("test_json_no_cfg")
		plugin.Reg(testType, func() RxTxRateLimiter { return NewFixed() }, func() any { return NewFixedCfg() })

		data := []byte(`{"type":"test_json_no_cfg"}`)
		c := &Cfg{}
		err := c.UnmarshalJSON(data)
		require.NoError(t, err)
		assert.Equal(t, testType, c.Type)
		// CreateCfg always returns a non-nil config for registered types (zero value)
		assert.NotNil(t, c.Cfg)
	})
}

func TestCfg_UnmarshalYAML(t *testing.T) {
	t.Run("valid with registered type", func(t *testing.T) {
		testType := Type("test_yaml_type")
		plugin.Reg(testType, func() RxTxRateLimiter { return NewFixed() }, func() any { return NewFixedCfg() })

		data := []byte("type: test_yaml_type\ncfg:\n  N: 50\n  Burst: 100\n")
		c := &Cfg{}
		err := c.UnmarshalYAML(data)
		require.NoError(t, err)
		assert.Equal(t, testType, c.Type)
		// YAML unmarshal into any field produces map[string]interface{}
		assert.NotNil(t, c.Cfg)
	})

	t.Run("valid with unregistered type", func(t *testing.T) {
		data := []byte("type: unknown_yaml_type\ncfg:\n  N: 10\n")
		c := &Cfg{}
		err := c.UnmarshalYAML(data)
		require.NoError(t, err)
		assert.Equal(t, Type("unknown_yaml_type"), c.Type)
		assert.Nil(t, c.Cfg)
	})

	t.Run("invalid yaml missing type", func(t *testing.T) {
		data := []byte("cfg:\n  N: 10\n")
		c := &Cfg{}
		err := c.UnmarshalYAML(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get ratelimiter type failed")
	})

	t.Run("invalid type field not a string", func(t *testing.T) {
		data := []byte("type: 123\ncfg:\n  N: 10\n")
		c := &Cfg{}
		err := c.UnmarshalYAML(data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid ratelimiter type")
	})

	t.Run("no cfg field with registered type", func(t *testing.T) {
		testType := Type("test_yaml_no_cfg")
		plugin.Reg(testType, func() RxTxRateLimiter { return NewFixed() }, func() any { return NewFixedCfg() })

		data := []byte("type: test_yaml_no_cfg\n")
		c := &Cfg{}
		err := c.UnmarshalYAML(data)
		require.NoError(t, err)
		assert.Equal(t, testType, c.Type)
		// Cfg is non-nil due to CreateCfg even when cfg key absent
		assert.NotNil(t, c.Cfg)
	})
}

func TestCfg_customUnmarshal(t *testing.T) {
	t.Run("registered type with JSON", func(t *testing.T) {
		testType := Type("test_custom_type")
		plugin.Reg(testType, func() RxTxRateLimiter { return NewFixed() }, func() any { return NewFixedCfg() })

		data := []byte(`{"type":"test_custom_type","cfg":{"N":42,"Burst":84}}`)
		c := &Cfg{Type: testType}
		err := c.customUnmarshal(data, jsons.Unmarshal)
		require.NoError(t, err)
		assertFixedCfg(t, c.Cfg, 42, 84)
	})

	t.Run("unregistered type", func(t *testing.T) {
		data := []byte(`{"cfg":{"N":10}}`)
		c := &Cfg{Type: "nonexistent"}
		err := c.customUnmarshal(data, jsons.Unmarshal)
		require.NoError(t, err)
		assert.Nil(t, c.Cfg)
	})

	t.Run("invalid json data", func(t *testing.T) {
		testType := Type("test_invalid_json")
		plugin.Reg(testType, func() RxTxRateLimiter { return NewFixed() }, func() any { return NewFixedCfg() })

		data := []byte(`{invalid`)
		c := &Cfg{Type: testType}
		err := c.customUnmarshal(data, jsons.Unmarshal)
		require.Error(t, err)
	})

	t.Run("registered type with YAML", func(t *testing.T) {
		testType := Type("test_custom_yaml_type")
		plugin.Reg(testType, func() RxTxRateLimiter { return NewFixed() }, func() any { return NewFixedCfg() })

		data := []byte("cfg:\n  N: 7\n  Burst: 14\n")
		c := &Cfg{Type: testType}
		err := c.customUnmarshal(data, yamls.Unmarshal)
		require.NoError(t, err)
		// YAML unmarshal into any field produces map[string]interface{}
		assert.NotNil(t, c.Cfg)
	})

	t.Run("invalid yaml data", func(t *testing.T) {
		testType := Type("test_invalid_yaml")
		plugin.Reg(testType, func() RxTxRateLimiter { return NewFixed() }, func() any { return NewFixedCfg() })

		data := []byte(":\n")
		c := &Cfg{Type: testType}
		err := c.customUnmarshal(data, yamls.Unmarshal)
		require.Error(t, err)
	})
}
