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
	v, exists := m.Load(testKey)
	require.True(t, exists)
	require.Equal(t, testValue, v)
}

func TestStoreAsString(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := 123 // This will be converted to string using conv.ToString

	m.StoreAsString(testKey, testValue)
	v, exists := m.Load(testKey)
	require.True(t, exists)
	require.Equal(t, "123", v)
}

func TestLoadOrStore(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := "initialValue"
	newValue := "newValue"

	m.Store(testKey, testValue)
	v, loaded := m.LoadOrStore(testKey, newValue)
	require.Equal(t, testValue, v)
	require.True(t, loaded)

	m.Del(testKey)

	v, loaded = m.LoadOrStore(testKey, newValue)
	require.Equal(t, newValue, v)
	require.False(t, loaded)
}

func TestLoadAndDelete(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := "testValue"

	m.Store(testKey, testValue)
	v, deleted := m.LoadAndDelete(testKey)
	require.Equal(t, testValue, v)
	require.True(t, deleted)

	_, deleted = m.LoadAndDelete(testKey)
	require.False(t, deleted)
}

func TestDel(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := "testValue"

	m.Store(testKey, testValue)
	m.Del(testKey)
	_, exists := m.Load(testKey)
	require.False(t, exists)
}

func TestLoadAsBool(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := true
	m.Store(testKey, testValue)

	v := m.LoadAsBool(testKey)
	require.Equal(t, testValue, v)
}

func TestLoadAsString(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := "testValue"
	m.Store(testKey, testValue)

	v := m.LoadAsString(testKey)
	require.Equal(t, testValue, v)
}

func TestLoadAsStringOr(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	defaultValue := "defaultValue"

	v := m.LoadAsStringOr(testKey, defaultValue)
	require.Equal(t, defaultValue, v)

	testValue := "testValue"
	m.Store(testKey, testValue)

	v = m.LoadAsStringOr(testKey, defaultValue)
	require.Equal(t, testValue, v)
}

func TestLoadAsInt(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := 42
	m.Store(testKey, testValue)

	v := m.LoadAsInt(testKey)
	require.Equal(t, testValue, v)
}

func TestLoadAsIntOr(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	defaultValue := 42

	v := m.LoadAsIntOr(testKey, defaultValue)
	require.Equal(t, defaultValue, v)

	testValue := 123
	m.Store(testKey, testValue)

	v = m.LoadAsIntOr(testKey, defaultValue)
	require.Equal(t, testValue, v)
}

func TestLoadAsUint(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := uint(42)
	m.Store(testKey, testValue)

	v := m.LoadAsUint(testKey)
	require.Equal(t, testValue, v)
}

func TestLoadAsUintOr(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	defaultValue := uint(42)

	v := m.LoadAsUintOr(testKey, defaultValue)
	require.Equal(t, defaultValue, v)

	testValue := uint(123)
	m.Store(testKey, testValue)

	v = m.LoadAsUintOr(testKey, defaultValue)
	require.Equal(t, testValue, v)
}

func TestLoadAsFloat(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	testValue := 42.5
	m.Store(testKey, testValue)

	v := m.LoadAsFloat(testKey)
	require.Equal(t, testValue, v)
}

func TestLoadAsFloatOr(t *testing.T) {
	m := NewMapKVS()

	testKey := "testKey"
	defaultValue := 42.5

	v := m.LoadAsFloatOr(testKey, defaultValue)
	require.Equal(t, defaultValue, v)

	testValue := 123.5
	m.Store(testKey, testValue)

	v = m.LoadAsFloatOr(testKey, defaultValue)
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

	all := m.LoadAllAsString()
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
