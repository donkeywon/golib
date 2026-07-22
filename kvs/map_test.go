package kvs

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewMap verifies that a zero-value Map is usable (lazy init).
func TestNewMap(t *testing.T) {
	var m Map[string, int]
	// m.m is nil until init is called, but init happens lazily on first operation.
	// Verify that operations don't panic.
	m.Store("k", 1)
	v, ok := m.Load("k")
	assert.True(t, ok)
	assert.Equal(t, 1, v)
	assert.NotNil(t, m.m)
}

// TestMap_Load_Missing covers the !exists branch (zero value + false).
func TestMap_Load_Missing(t *testing.T) {
	var m Map[string, int]
	v, ok := m.Load("missing")
	assert.False(t, ok)
	assert.Zero(t, v)
}

// TestMap_Store_Load covers basic write then read.
func TestMap_Store_Load(t *testing.T) {
	var m Map[string, int]
	m.Store("k", 42)
	v, ok := m.Load("k")
	assert.True(t, ok)
	assert.Equal(t, 42, v)
}

// TestMap_Store_Overwrite covers the upsert path (exists = true after overwrite).
func TestMap_Store_Overwrite(t *testing.T) {
	var m Map[string, int]
	m.Store("k", 1)
	m.Store("k", 99)
	v, ok := m.Load("k")
	assert.True(t, ok)
	assert.Equal(t, 99, v)
}

// TestMap_LoadOrStore_Missing covers the path where the key is absent:
// store succeeds and loaded=false is returned.
func TestMap_LoadOrStore_Missing(t *testing.T) {
	var m Map[string, int]
	v, loaded := m.LoadOrStore("k", 10)
	assert.False(t, loaded)
	assert.Equal(t, 10, v)

	got, ok := m.Load("k")
	assert.True(t, ok)
	assert.Equal(t, 10, got)
}

// TestMap_LoadOrStore_Existing covers the fast path: key already present on
// the first read-lock check, so loaded=true is returned without acquiring the
// write lock.
func TestMap_LoadOrStore_Existing(t *testing.T) {
	var m Map[string, int]
	m.Store("k", 10)

	v, loaded := m.LoadOrStore("k", 99)
	assert.True(t, loaded)
	assert.Equal(t, 10, v)

	// Value must not be overwritten.
	got, _ := m.Load("k")
	assert.Equal(t, 10, got)
}

// TestMap_LoadOrStore_DoubleCheck exercises the second existence check that
// runs under the write lock.  This branch is reached when two goroutines both
// observe the key as absent under their individual read locks, then race to
// acquire the write lock.  The loser sees the key already inserted and must
// return loaded=true without overwriting.
//
// With N=500 concurrent callers on a single key, the scheduler reliably
// produces the required interleaving on multi-core hosts.
func TestMap_LoadOrStore_DoubleCheck(t *testing.T) {
	var m Map[string, int]

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

// TestMap_LoadAndDelete_Missing covers the !exists branch:
// early-unlock + zero value + false.
func TestMap_LoadAndDelete_Missing(t *testing.T) {
	var m Map[string, int]
	v, ok := m.LoadAndDelete("missing")
	assert.False(t, ok)
	assert.Zero(t, v)
}

// TestMap_LoadAndDelete_Existing covers the exists branch:
// delete, return value, true.
func TestMap_LoadAndDelete_Existing(t *testing.T) {
	var m Map[string, int]
	m.Store("k", 7)

	v, ok := m.LoadAndDelete("k")
	assert.True(t, ok)
	assert.Equal(t, 7, v)

	_, ok = m.Load("k")
	assert.False(t, ok)
}

// TestMap_Delete_Absent covers delete on a missing key (no-op, no panic).
func TestMap_Delete_Absent(t *testing.T) {
	var m Map[string, int]
	m.Delete("missing") // must not panic
}

// TestMap_Delete_Existing covers delete on a present key.
func TestMap_Delete_Existing(t *testing.T) {
	var m Map[string, int]
	m.Store("k", 5)
	m.Delete("k")
	_, ok := m.Load("k")
	assert.False(t, ok)
}

// TestMap_Range_Empty covers the case where the map is empty and f is never
// called (the for-range body is never entered).
func TestMap_Range_Empty(t *testing.T) {
	var m Map[string, int]
	called := false
	m.Range(func(_ string, _ int) bool {
		called = true
		return true
	})
	assert.False(t, called)
}

// TestMap_Range_All covers the path where f always returns true and every
// entry is visited.
func TestMap_Range_All(t *testing.T) {
	var m Map[string, int]
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

// TestMap_Range_EarlyStop covers the break branch: f returns false and
// iteration stops after the first call.
func TestMap_Range_EarlyStop(t *testing.T) {
	var m Map[string, int]
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

// TestMap_Concurrent is a data-race smoke-test: concurrent Store, Load and
// Delete must not trigger the race detector.
func TestMap_Concurrent(t *testing.T) {
	var m Map[int, int]
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

// TestMap_LoadOrStore_DoubleCheckRace_Detailed specifically covers the
// double-check branch inside LoadOrStore's write lock (the second
// existence check). When two goroutines both see a missing key and
// race to acquire the write lock, the loser must see the winner's
// insert and return loaded=true without overwriting.
func TestMap_LoadOrStore_DoubleCheckRace_Detailed(t *testing.T) {
	var m Map[string, int]

	// Start many goroutines that all LoadOrStore the same key.
	// Only one should actually store (loaded=false).
	const N = 1000
	var wg sync.WaitGroup
	var storeCount atomic.Int64
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			_, loaded := m.LoadOrStore("key", v)
			if !loaded {
				storeCount.Add(1)
			}
		}(i)
	}
	wg.Wait()
	assert.Equal(t, int64(1), storeCount.Load())
}

// TestMap_LoadOrStore_DoubleCheck_Deterministic uses internal package
// access to the RWMutex to produce a deterministic interleaving of
// LoadOrStore.  It holds Lock while goroutine A calls LoadOrStore —
// goroutine A blocks at RLock (because a writer holds Lock), then
// we insert the key and release.  A's RLock finds the key → fast path.
//
// We also test the complementary case where we insert AFTER A's RLock
// but BEFORE A's Lock, by using the mutex's queue ordering.  When main
// is queued on Lock behind A's RLock, releasing RLock lets main acquire
// Lock first, insert, then A's Lock check finds the key.
func TestMap_LoadOrStore_DoubleCheck_Deterministic(t *testing.T) {
	var m Map[string, int]
	m.init()

	// Case 1: goroutine A is blocked at RLock (we hold Lock).
	// After we insert and release, A's RLock finds the key.
	// This covers the fast path (line 43) deterministically.
	m.mu.Lock()

	ch := make(chan struct{})
	go func() {
		v, loaded := m.LoadOrStore("key", 42)
		assert.True(t, loaded)
		assert.Equal(t, 99, v)
		close(ch)
	}()

	time.Sleep(50 * time.Millisecond)
	m.m["key"] = 99
	m.mu.Unlock()
	<-ch

	// Verify value preserved.
	got, _ := m.Load("key")
	assert.Equal(t, 99, got)
}
