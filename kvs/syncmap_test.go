package kvs

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSyncMapKVS_Load_Missing covers the !ok branch: zero value + false.
func TestSyncMapKVS_Load_Missing(t *testing.T) {
	var s SyncMapKVS[string, int]
	v, ok := s.Load("missing")
	assert.False(t, ok)
	assert.Zero(t, v)
}

// TestSyncMapKVS_Store_Load covers the ok=true branch after a Store.
func TestSyncMapKVS_Store_Load(t *testing.T) {
	var s SyncMapKVS[string, int]
	s.Store("k", 42)
	v, ok := s.Load("k")
	assert.True(t, ok)
	assert.Equal(t, 42, v)
}

// TestSyncMapKVS_Store_Overwrite verifies that a second Store replaces the
// previous value and the type assertion still resolves correctly.
func TestSyncMapKVS_Store_Overwrite(t *testing.T) {
	var s SyncMapKVS[string, int]
	s.Store("k", 1)
	s.Store("k", 99)
	v, ok := s.Load("k")
	assert.True(t, ok)
	assert.Equal(t, 99, v)
}

// TestSyncMapKVS_LoadOrStore_Missing covers loaded=false: key is absent so the
// provided value is stored and returned.
func TestSyncMapKVS_LoadOrStore_Missing(t *testing.T) {
	var s SyncMapKVS[string, int]
	v, loaded := s.LoadOrStore("k", 10)
	assert.False(t, loaded)
	assert.Equal(t, 10, v)
}

// TestSyncMapKVS_LoadOrStore_Existing covers loaded=true: key is already
// present, original value is returned and nothing is overwritten.
func TestSyncMapKVS_LoadOrStore_Existing(t *testing.T) {
	var s SyncMapKVS[string, int]
	s.Store("k", 10)

	v, loaded := s.LoadOrStore("k", 99)
	assert.True(t, loaded)
	assert.Equal(t, 10, v)

	got, _ := s.Load("k")
	assert.Equal(t, 10, got)
}

// TestSyncMapKVS_LoadAndDelete_Missing covers the !loaded branch:
// zero value + false when the key does not exist.
func TestSyncMapKVS_LoadAndDelete_Missing(t *testing.T) {
	var s SyncMapKVS[string, int]
	v, ok := s.LoadAndDelete("missing")
	assert.False(t, ok)
	assert.Zero(t, v)
}

// TestSyncMapKVS_LoadAndDelete_Existing covers the loaded=true branch:
// the value is returned and the key is removed.
func TestSyncMapKVS_LoadAndDelete_Existing(t *testing.T) {
	var s SyncMapKVS[string, int]
	s.Store("k", 7)

	v, ok := s.LoadAndDelete("k")
	assert.True(t, ok)
	assert.Equal(t, 7, v)

	_, ok = s.Load("k")
	assert.False(t, ok)
}

// TestSyncMapKVS_Delete_Absent covers delete on a missing key (no-op).
func TestSyncMapKVS_Delete_Absent(t *testing.T) {
	var s SyncMapKVS[string, int]
	s.Delete("missing") // must not panic
}

// TestSyncMapKVS_Delete_Existing covers delete on a present key.
func TestSyncMapKVS_Delete_Existing(t *testing.T) {
	var s SyncMapKVS[string, int]
	s.Store("k", 5)
	s.Delete("k")
	_, ok := s.Load("k")
	assert.False(t, ok)
}

// TestSyncMapKVS_Range_Empty covers the path where f is never called because
// the map is empty (the inner wrapper closure is never entered).
func TestSyncMapKVS_Range_Empty(t *testing.T) {
	var s SyncMapKVS[string, int]
	called := false
	s.Range(func(_ string, _ int) bool {
		called = true
		return true
	})
	assert.False(t, called)
}

// TestSyncMapKVS_Range_All covers the path where f returns true for every
// entry and every entry is visited; the inner closure's type assertions are
// exercised for both K and V.
func TestSyncMapKVS_Range_All(t *testing.T) {
	var s SyncMapKVS[string, int]
	s.Store("a", 1)
	s.Store("b", 2)
	s.Store("c", 3)

	got := make(map[string]int)
	s.Range(func(k string, v int) bool {
		got[k] = v
		return true
	})
	assert.Equal(t, map[string]int{"a": 1, "b": 2, "c": 3}, got)
}

// TestSyncMapKVS_Range_EarlyStop covers the path where f returns false and
// sync.Map stops iterating.
func TestSyncMapKVS_Range_EarlyStop(t *testing.T) {
	var s SyncMapKVS[string, int]
	s.Store("a", 1)
	s.Store("b", 2)
	s.Store("c", 3)

	count := 0
	s.Range(func(_ string, _ int) bool {
		count++
		return false
	})
	assert.Equal(t, 1, count)
}

// TestSyncMapKVS_Concurrent is a data-race smoke-test that exercises all
// methods concurrently under the race detector.
func TestSyncMapKVS_Concurrent(t *testing.T) {
	var s SyncMapKVS[int, int]
	var wg sync.WaitGroup
	const N = 200

	for i := 0; i < N; i++ {
		wg.Add(3)
		go func(i int) {
			defer wg.Done()
			s.Store(i, i)
		}(i)
		go func(i int) {
			defer wg.Done()
			s.Load(i)
		}(i)
		go func(i int) {
			defer wg.Done()
			s.Delete(i)
		}(i)
	}
	wg.Wait()
}
