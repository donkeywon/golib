package kvs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewMapKVS(t *testing.T) {
	m := NewMapKVS()
	require.NotNil(t, m)
	require.NotNil(t, m.m)
}

func TestStoreAndLoad(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := "testValue"

	m.Store(testKey, testValue)
	v, exists, _ := m.Load(testKey)
	require.True(t, exists)
	require.Equal(t, testValue, v)
}

func TestStoreAsString(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := 123 // This will be converted to string using conv.ToString

	MustStoreAsString(m, testKey, testValue)
	v, exists, _ := m.Load(testKey)
	require.True(t, exists)
	require.Equal(t, "123", v)
}

func TestLoadOrStore(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := "initialValue"
	newValue := "newValue"

	m.Store(testKey, testValue)
	v, loaded, _ := m.LoadOrStore(testKey, newValue)
	require.Equal(t, testValue, v)
	require.True(t, loaded)

	m.Del(testKey)

	v, loaded, _ = m.LoadOrStore(testKey, newValue)
	require.Equal(t, newValue, v)
	require.False(t, loaded)
}

func TestLoadAndDelete(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := "testValue"

	m.Store(testKey, testValue)
	v, deleted, _ := m.LoadAndDelete(testKey)
	require.Equal(t, testValue, v)
	require.True(t, deleted)

	_, deleted, _ = m.LoadAndDelete(testKey)
	require.False(t, deleted)
}

func TestDel(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := "testValue"

	m.Store(testKey, testValue)
	m.Del(testKey)
	_, exists, _ := m.Load(testKey)
	require.False(t, exists)
}

func TestLoadAsBool(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := true
	m.Store(testKey, testValue)

	v, _ := MustLoadAsBool(m, testKey)
	require.Equal(t, testValue, v)
}

func TestLoadAsString(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := "testValue"
	m.Store(testKey, testValue)

	v, _ := MustLoadAsString(m, testKey)
	require.Equal(t, testValue, v)
}

func TestLoadAsStringOr(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	defaultValue := "defaultValue"

	v, _ := MustLoadAsStringOr(m, testKey, defaultValue)
	require.Equal(t, defaultValue, v)

	testValue := "testValue"
	m.Store(testKey, testValue)

	v, _ = MustLoadAsStringOr(m, testKey, defaultValue)
	require.Equal(t, testValue, v)
}

func TestLoadAsInt(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := 42
	m.Store(testKey, testValue)

	v, _ := MustLoadAsInt(m, testKey)
	require.Equal(t, testValue, v)
}

func TestLoadAsIntOr(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	defaultValue := 42

	v, _ := MustLoadAsIntOr(m, testKey, defaultValue)
	require.Equal(t, defaultValue, v)

	testValue := 123
	m.Store(testKey, testValue)

	v, _ = MustLoadAsIntOr(m, testKey, defaultValue)
	require.Equal(t, testValue, v)
}

func TestLoadAsUint(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := uint(42)
	m.Store(testKey, testValue)

	v, _ := MustLoadAsUint(m, testKey)
	require.Equal(t, testValue, v)
}

func TestLoadAsUintOr(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	defaultValue := uint(42)

	v, _ := MustLoadAsUintOr(m, testKey, defaultValue)
	require.Equal(t, defaultValue, v)

	testValue := uint(123)
	m.Store(testKey, testValue)

	v, _ = MustLoadAsUintOr(m, testKey, defaultValue)
	require.Equal(t, testValue, v)
}

func TestLoadAsFloat(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := 42.5
	m.Store(testKey, testValue)

	v, _ := MustLoadAsFloat(m, testKey)
	require.Equal(t, testValue, v)
}

func TestLoadAsFloatOr(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	defaultValue := 42.5

	v, _ := MustLoadAsFloatOr(m, testKey, defaultValue)
	require.Equal(t, defaultValue, v)

	testValue := 123.5
	m.Store(testKey, testValue)

	v, _ = MustLoadAsFloatOr(m, testKey, defaultValue)
	require.Equal(t, testValue, v)
}

func TestLoadAll(t *testing.T) {
	m := NewMapKVS()

	testData := map[string]any{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	for k, v := range testData {
		m.Store(k, v)
	}

	all := m.LoadAll()
	require.Len(t, all, len(testData))

	for k, expectedV := range testData {
		actualV, exists := all[k]
		require.True(t, exists)
		require.Equal(t, expectedV, actualV)
	}
}

func TestLoadAllAsString(t *testing.T) {
	m := NewMapKVS()

	testData := map[string]string{
		"key1": "value1",
		"key2": "42",
		"key3": "true",
	}

	for k, v := range testData {
		m.Store(k, v)
	}

	all := MustLoadAllAsString(m)
	require.Len(t, all, len(testData))

	for k, expectedV := range testData {
		actualV, exists := all[k]
		require.True(t, exists)
		require.Equal(t, expectedV, actualV)
	}
}

func TestRange(t *testing.T) {
	m := NewMapKVS()

	testData := map[string]any{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	for k, v := range testData {
		m.Store(k, v)
	}

	keysVisited := make([]string, 0)
	m.Range(func(k string, _ any) bool {
		keysVisited = append(keysVisited, k)
		return true
	})

	require.ElementsMatch(t, []string{"key1", "key2", "key3"}, keysVisited)
}
