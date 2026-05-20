package kvs

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewMapKVS verifies that the constructor returns a ready-to-use instance.
func TestNewMapKVS(t *testing.T) {
	m := NewMapKVS[string, int]()
	assert.NotNil(t, m)
	assert.NotNil(t, m.m)
}

// TestMapKVS_Load_Missing covers the !exists branch (zero value + false).
func TestMapKVS_Load_Missing(t *testing.T) {
	m := NewMapKVS[string, int]()
	v, ok := m.Load("missing")
	assert.False(t, ok)
	assert.Zero(t, v)
}

// TestMapKVS_Store_Load covers basic write then read.
func TestMapKVS_Store_Load(t *testing.T) {
	m := NewMapKVS[string, int]()
	m.Store("k", 42)
	v, ok := m.Load("k")
	assert.True(t, ok)
	assert.Equal(t, 42, v)
}

// TestMapKVS_Store_Overwrite covers the upsert path (exists = true after overwrite).
func TestMapKVS_Store_Overwrite(t *testing.T) {
	m := NewMapKVS[string, int]()
	m.Store("k", 1)
	m.Store("k", 99)
	v, ok := m.Load("k")
	assert.True(t, ok)
	assert.Equal(t, 99, v)
}

// TestMapKVS_LoadOrStore_Missing covers the path where the key is absent:
// store succeeds and loaded=false is returned.
func TestMapKVS_LoadOrStore_Missing(t *testing.T) {
	m := NewMapKVS[string, int]()
	v, loaded := m.LoadOrStore("k", 10)
	assert.False(t, loaded)
	assert.Equal(t, 10, v)

	got, ok := m.Load("k")
	assert.True(t, ok)
	assert.Equal(t, 10, got)
}

// TestMapKVS_LoadOrStore_Existing covers the fast path: key already present on
// the first read-lock check, so loaded=true is returned without acquiring the
// write lock.
func TestMapKVS_LoadOrStore_Existing(t *testing.T) {
	m := NewMapKVS[string, int]()
	m.Store("k", 10)

	v, loaded := m.LoadOrStore("k", 99)
	assert.True(t, loaded)
	assert.Equal(t, 10, v)

	// Value must not be overwritten.
	got, _ := m.Load("k")
	assert.Equal(t, 10, got)
}

// TestMapKVS_LoadOrStore_DoubleCheck exercises the second existence check that
// runs under the write lock.  This branch is reached when two goroutines both
// observe the key as absent under their individual read locks, then race to
// acquire the write lock.  The loser sees the key already inserted and must
// return loaded=true without overwriting.
//
// With N=500 concurrent callers on a single key, the scheduler reliably
// produces the required interleaving on multi-core hosts.
func TestMapKVS_LoadOrStore_DoubleCheck(t *testing.T) {
	m := NewMapKVS[string, int]()

	const N = 500
	var wg sync.WaitGroup
	var storeCount int64

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			_, loaded := m.LoadOrStore("shared", val)
			if !loaded {
				atomic.AddInt64(&storeCount, 1)
			}
		}(i)
	}
	wg.Wait()

	// Exactly one goroutine must have performed the actual insert.
	assert.Equal(t, int64(1), storeCount)
	_, ok := m.Load("shared")
	assert.True(t, ok)
}

// TestMapKVS_LoadAndDelete_Missing covers the !exists branch:
// early-unlock + zero value + false.
func TestMapKVS_LoadAndDelete_Missing(t *testing.T) {
	m := NewMapKVS[string, int]()
	v, ok := m.LoadAndDelete("missing")
	assert.False(t, ok)
	assert.Zero(t, v)
}

// TestMapKVS_LoadAndDelete_Existing covers the exists branch:
// delete, return value, true.
func TestMapKVS_LoadAndDelete_Existing(t *testing.T) {
	m := NewMapKVS[string, int]()
	m.Store("k", 7)

	v, ok := m.LoadAndDelete("k")
	assert.True(t, ok)
	assert.Equal(t, 7, v)

	_, ok = m.Load("k")
	assert.False(t, ok)
}

// TestMapKVS_Delete_Absent covers delete on a missing key (no-op, no panic).
func TestMapKVS_Delete_Absent(t *testing.T) {
	m := NewMapKVS[string, int]()
	m.Delete("missing") // must not panic
}

// TestMapKVS_Delete_Existing covers delete on a present key.
func TestMapKVS_Delete_Existing(t *testing.T) {
	m := NewMapKVS[string, int]()
	m.Store("k", 5)
	m.Delete("k")
	_, ok := m.Load("k")
	assert.False(t, ok)
}

// TestMapKVS_Range_Empty covers the case where the map is empty and f is never
// called (the for-range body is never entered).
func TestMapKVS_Range_Empty(t *testing.T) {
	m := NewMapKVS[string, int]()
	called := false
	m.Range(func(_ string, _ int) bool {
		called = true
		return true
	})
	assert.False(t, called)
}

// TestMapKVS_Range_All covers the path where f always returns true and every
// entry is visited.
func TestMapKVS_Range_All(t *testing.T) {
	m := NewMapKVS[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)
	m.Store("c", 3)

	got := make(map[string]int)
	m.Range(func(k string, v int) bool {
		got[k] = v
		return true
	})
	assert.Equal(t, map[string]int{"a": 1, "b": 2, "c": 3}, got)
}

// TestMapKVS_Range_EarlyStop covers the break branch: f returns false and
// iteration stops after the first call.
func TestMapKVS_Range_EarlyStop(t *testing.T) {
	m := NewMapKVS[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)
	m.Store("c", 3)

	count := 0
	m.Range(func(_ string, _ int) bool {
		count++
		return false
	})
	assert.Equal(t, 1, count)
}

// TestMapKVS_Concurrent is a data-race smoke-test: concurrent Store, Load and
// Delete must not trigger the race detector.
func TestMapKVS_Concurrent(t *testing.T) {
	m := NewMapKVS[int, int]()
	var wg sync.WaitGroup

	for i := range 200 {
		wg.Add(3)
		go func(i int) {
			defer wg.Done()
			m.Store(i, i)
		}(i)
		go func(i int) {
			defer wg.Done()
			m.Load(i)
		}(i)
		go func(i int) {
			defer wg.Done()
			m.Delete(i)
		}(i)
	}
	wg.Wait()
}
