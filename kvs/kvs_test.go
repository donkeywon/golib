package kvs

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Map tests
// =============================================================================

func TestMap_ZeroValueUsability(t *testing.T) {
	t.Run("store on zero-value map without init", func(t *testing.T) {
		var m Map[string, int]
		// Must not panic — lazy init via sync.Once
		m.Store("k", 42)
		v, ok := m.Load("k")
		assert.True(t, ok)
		assert.Equal(t, 42, v)
	})

	t.Run("load on zero-value map without init", func(t *testing.T) {
		var m Map[string, int]
		v, ok := m.Load("missing")
		assert.False(t, ok)
		assert.Zero(t, v)
	})

	t.Run("delete on zero-value map without init", func(t *testing.T) {
		var m Map[string, int]
		// Must not panic
		m.Delete("missing")
	})

	t.Run("range on zero-value map is empty", func(t *testing.T) {
		var m Map[string, int]
		count := 0
		m.Range(func(_ string, _ int) bool {
			count++
			return true
		})
		assert.Equal(t, 0, count)
	})

	t.Run("loadAndDelete on zero-value map", func(t *testing.T) {
		var m Map[string, int]
		v, ok := m.LoadAndDelete("missing")
		assert.False(t, ok)
		assert.Zero(t, v)
	})

	t.Run("loadOrStore on zero-value map", func(t *testing.T) {
		var m Map[string, int]
		v, loaded := m.LoadOrStore("k", 100)
		assert.False(t, loaded)
		assert.Equal(t, 100, v)
	})
}

func TestMap_Store(t *testing.T) {
	t.Run("stores string keys and values", func(t *testing.T) {
		m := Map[string, string]{}
		m.Store("hello", "world")
		v, ok := m.Load("hello")
		assert.True(t, ok)
		assert.Equal(t, "world", v)
	})

	t.Run("stores multiple entries", func(t *testing.T) {
		m := Map[string, int]{}
		for i := 0; i < 100; i++ {
			m.Store(fmt.Sprintf("key%d", i), i)
		}
		for i := 0; i < 100; i++ {
			v, ok := m.Load(fmt.Sprintf("key%d", i))
			assert.True(t, ok)
			assert.Equal(t, i, v)
		}
	})

	t.Run("stores struct values", func(t *testing.T) {
		type item struct {
			Name  string
			Value int
		}
		m := Map[string, item]{}
		expected := item{Name: "test", Value: 42}
		m.Store("a", expected)
		v, ok := m.Load("a")
		assert.True(t, ok)
		assert.Equal(t, expected, v)
	})
}

func TestMap_Load(t *testing.T) {
	t.Run("load missing key", func(t *testing.T) {
		var m Map[string, int]
		m.Store("exists", 1)
		v, ok := m.Load("missing")
		assert.False(t, ok)
		assert.Zero(t, v)
	})

	t.Run("load on empty map", func(t *testing.T) {
		var m Map[string, int]
		v, ok := m.Load("anything")
		assert.False(t, ok)
		assert.Zero(t, v)
	})
}

func TestMap_LoadOrStore(t *testing.T) {
	t.Run("stores when key missing", func(t *testing.T) {
		var m Map[string, int]
		v, loaded := m.LoadOrStore("newkey", 50)
		assert.False(t, loaded)
		assert.Equal(t, 50, v)

		got, ok := m.Load("newkey")
		assert.True(t, ok)
		assert.Equal(t, 50, got)
	})

	t.Run("returns existing value without overwriting", func(t *testing.T) {
		var m Map[string, int]
		m.Store("key", 10)
		v, loaded := m.LoadOrStore("key", 99)
		assert.True(t, loaded)
		assert.Equal(t, 10, v)

		got, _ := m.Load("key")
		assert.Equal(t, 10, got)
	})
}

func TestMap_LoadAndDelete(t *testing.T) {
	t.Run("deletes and returns existing value", func(t *testing.T) {
		var m Map[string, int]
		m.Store("del", 77)
		v, ok := m.LoadAndDelete("del")
		assert.True(t, ok)
		assert.Equal(t, 77, v)

		_, ok = m.Load("del")
		assert.False(t, ok)
	})

	t.Run("missing key returns zero", func(t *testing.T) {
		var m Map[string, int]
		v, ok := m.LoadAndDelete("nope")
		assert.False(t, ok)
		assert.Zero(t, v)
	})
}

func TestMap_Delete(t *testing.T) {
	t.Run("delete existing key", func(t *testing.T) {
		var m Map[string, int]
		m.Store("x", 1)
		m.Delete("x")
		_, ok := m.Load("x")
		assert.False(t, ok)
	})

	t.Run("delete on empty map does not panic", func(t *testing.T) {
		var m Map[string, int]
		m.Delete("phantom")
	})

	t.Run("delete missing key does not panic", func(t *testing.T) {
		var m Map[string, int]
		m.Store("real", 1)
		m.Delete("notreal")
		_, ok := m.Load("real")
		assert.True(t, ok)
	})
}

func TestMap_Range(t *testing.T) {
	t.Run("empty map does not call f", func(t *testing.T) {
		var m Map[string, int]
		called := false
		m.Range(func(_ string, _ int) bool {
			called = true
			return true
		})
		assert.False(t, called)
	})

	t.Run("visits all entries", func(t *testing.T) {
		var m Map[string, int]
		m.Store("a", 1)
		m.Store("b", 2)
		m.Store("c", 3)

		visited := make(map[string]int)
		m.Range(func(k string, v int) bool {
			visited[k] = v
			return true
		})
		assert.Equal(t, map[string]int{"a": 1, "b": 2, "c": 3}, visited)
	})

	t.Run("stops early when f returns false", func(t *testing.T) {
		var m Map[string, int]
		m.Store("a", 1)
		m.Store("b", 2)

		count := 0
		m.Range(func(_ string, _ int) bool {
			count++
			return false
		})
		assert.Equal(t, 1, count)
	})

	t.Run("f returns false on first call, single entry map", func(t *testing.T) {
		var m Map[string, int]
		m.Store("only", 1)

		count := 0
		m.Range(func(_ string, _ int) bool {
			count++
			return false
		})
		assert.Equal(t, 1, count)
	})
}

func TestMap_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent store and load", func(t *testing.T) {
		var m Map[int64, int64]
		var wg sync.WaitGroup

		const N = 500
		for i := int64(0); i < N; i++ {
			wg.Add(2)
			go func(i int64) {
				defer wg.Done()
				m.Store(i, i*10)
			}(i)
			go func(i int64) {
				defer wg.Done()
				m.Load(i)
			}(i)
		}
		wg.Wait()

		for i := int64(0); i < N; i++ {
			v, ok := m.Load(i)
			if ok {
				assert.Equal(t, i*10, v)
			}
		}
	})

	t.Run("concurrent loadOrStore on same key", func(t *testing.T) {
		var m Map[string, int]
		var wg sync.WaitGroup
		var storeCount int64

		const N = 200
		for i := 0; i < N; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, loaded := m.LoadOrStore("shared", i)
				if !loaded {
					atomic.AddInt64(&storeCount, 1)
				}
			}(i)
		}
		wg.Wait()
		assert.Equal(t, int64(1), storeCount)
	})

	t.Run("concurrent range and store", func(t *testing.T) {
		var m Map[int, int]
		// Pre-populate
		for i := 0; i < 100; i++ {
			m.Store(i, i)
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Range(func(k, v int) bool {
				_ = k
				_ = v
				return true
			})
		}()

		for i := 100; i < 200; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				m.Store(i, i)
			}(i)
		}
		wg.Wait()
	})

	t.Run("multiple operations interleaved", func(t *testing.T) {
		var m Map[string, int]
		var wg sync.WaitGroup

		ops := []func(){
			func() { m.Store("a", 1) },
			func() { m.Store("b", 2) },
			func() { m.Load("a") },
			func() { m.Load("c") },
			func() { m.Delete("b") },
			func() { m.Delete("z") },
			func() { m.LoadOrStore("d", 4) },
			func() { m.LoadOrStore("a", 10) },
			func() { m.LoadAndDelete("a") },
			func() { m.LoadAndDelete("nonexistent") },
			func() {
				m.Range(func(k string, v int) bool {
					_ = k
					_ = v
					return true
				})
			},
		}

		for _, op := range ops {
			wg.Add(1)
			go func(op func()) {
				defer wg.Done()
				op()
			}(op)
		}
		wg.Wait()
	})
}

// =============================================================================
// SyncMap tests
// =============================================================================

func TestSyncMap_ZeroValueUsability(t *testing.T) {
	t.Run("store on zero-value syncmap", func(t *testing.T) {
		var s SyncMap[string, int]
		s.Store("k", 42)
		v, ok := s.Load("k")
		assert.True(t, ok)
		assert.Equal(t, 42, v)
	})

	t.Run("load on zero-value syncmap", func(t *testing.T) {
		var s SyncMap[string, int]
		v, ok := s.Load("missing")
		assert.False(t, ok)
		assert.Zero(t, v)
	})

	t.Run("delete on zero-value syncmap", func(t *testing.T) {
		var s SyncMap[string, int]
		s.Delete("missing")
	})

	t.Run("range on zero-value syncmap is empty", func(t *testing.T) {
		var s SyncMap[string, int]
		count := 0
		s.Range(func(_ string, _ int) bool {
			count++
			return true
		})
		assert.Equal(t, 0, count)
	})

	t.Run("loadAndDelete on zero-value syncmap", func(t *testing.T) {
		var s SyncMap[string, int]
		v, ok := s.LoadAndDelete("missing")
		assert.False(t, ok)
		assert.Zero(t, v)
	})

	t.Run("loadOrStore on zero-value syncmap", func(t *testing.T) {
		var s SyncMap[string, int]
		v, loaded := s.LoadOrStore("k", 100)
		assert.False(t, loaded)
		assert.Equal(t, 100, v)
	})
}

func TestSyncMap_Store(t *testing.T) {
	t.Run("stores string keys and values", func(t *testing.T) {
		var s SyncMap[string, string]
		s.Store("hello", "world")
		v, ok := s.Load("hello")
		assert.True(t, ok)
		assert.Equal(t, "world", v)
	})

	t.Run("stores multiple entries", func(t *testing.T) {
		var s SyncMap[string, int]
		for i := 0; i < 100; i++ {
			s.Store(fmt.Sprintf("key%d", i), i)
		}
		for i := 0; i < 100; i++ {
			v, ok := s.Load(fmt.Sprintf("key%d", i))
			assert.True(t, ok)
			assert.Equal(t, i, v)
		}
	})

	t.Run("stores struct values", func(t *testing.T) {
		type item struct {
			Name  string
			Value int
		}
		var s SyncMap[string, item]
		expected := item{Name: "test", Value: 42}
		s.Store("a", expected)
		v, ok := s.Load("a")
		assert.True(t, ok)
		assert.Equal(t, expected, v)
	})
}

func TestSyncMap_Load(t *testing.T) {
	t.Run("load missing key", func(t *testing.T) {
		var s SyncMap[string, int]
		s.Store("exists", 1)
		v, ok := s.Load("missing")
		assert.False(t, ok)
		assert.Zero(t, v)
	})

	t.Run("load on empty map", func(t *testing.T) {
		var s SyncMap[string, int]
		v, ok := s.Load("anything")
		assert.False(t, ok)
		assert.Zero(t, v)
	})
}

func TestSyncMap_LoadOrStore(t *testing.T) {
	t.Run("stores when key missing", func(t *testing.T) {
		var s SyncMap[string, int]
		v, loaded := s.LoadOrStore("newkey", 50)
		assert.False(t, loaded)
		assert.Equal(t, 50, v)

		got, ok := s.Load("newkey")
		assert.True(t, ok)
		assert.Equal(t, 50, got)
	})

	t.Run("returns existing value without overwriting", func(t *testing.T) {
		var s SyncMap[string, int]
		s.Store("key", 10)
		v, loaded := s.LoadOrStore("key", 99)
		assert.True(t, loaded)
		assert.Equal(t, 10, v)

		got, _ := s.Load("key")
		assert.Equal(t, 10, got)
	})
}

func TestSyncMap_LoadAndDelete(t *testing.T) {
	t.Run("deletes and returns existing value", func(t *testing.T) {
		var s SyncMap[string, int]
		s.Store("del", 77)
		v, ok := s.LoadAndDelete("del")
		assert.True(t, ok)
		assert.Equal(t, 77, v)

		_, ok = s.Load("del")
		assert.False(t, ok)
	})

	t.Run("missing key returns zero", func(t *testing.T) {
		var s SyncMap[string, int]
		v, ok := s.LoadAndDelete("nope")
		assert.False(t, ok)
		assert.Zero(t, v)
	})
}

func TestSyncMap_Delete(t *testing.T) {
	t.Run("delete existing key", func(t *testing.T) {
		var s SyncMap[string, int]
		s.Store("x", 1)
		s.Delete("x")
		_, ok := s.Load("x")
		assert.False(t, ok)
	})

	t.Run("delete on empty map does not panic", func(t *testing.T) {
		var s SyncMap[string, int]
		s.Delete("phantom")
	})

	t.Run("delete missing key does not panic", func(t *testing.T) {
		var s SyncMap[string, int]
		s.Store("real", 1)
		s.Delete("notreal")
		_, ok := s.Load("real")
		assert.True(t, ok)
	})
}

func TestSyncMap_Range(t *testing.T) {
	t.Run("empty map does not call f", func(t *testing.T) {
		var s SyncMap[string, int]
		called := false
		s.Range(func(_ string, _ int) bool {
			called = true
			return true
		})
		assert.False(t, called)
	})

	t.Run("visits all entries", func(t *testing.T) {
		var s SyncMap[string, int]
		s.Store("a", 1)
		s.Store("b", 2)
		s.Store("c", 3)

		visited := make(map[string]int)
		s.Range(func(k string, v int) bool {
			visited[k] = v
			return true
		})
		assert.Equal(t, map[string]int{"a": 1, "b": 2, "c": 3}, visited)
	})

	t.Run("stops early when f returns false", func(t *testing.T) {
		var s SyncMap[string, int]
		s.Store("a", 1)
		s.Store("b", 2)

		count := 0
		s.Range(func(_ string, _ int) bool {
			count++
			return false
		})
		assert.Equal(t, 1, count)
	})

	t.Run("f returns false on first call, single entry map", func(t *testing.T) {
		var s SyncMap[string, int]
		s.Store("only", 1)

		count := 0
		s.Range(func(_ string, _ int) bool {
			count++
			return false
		})
		assert.Equal(t, 1, count)
	})
}

func TestSyncMap_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent store and load", func(t *testing.T) {
		var s SyncMap[int64, int64]
		var wg sync.WaitGroup

		const N = 500
		for i := int64(0); i < N; i++ {
			wg.Add(2)
			go func(i int64) {
				defer wg.Done()
				s.Store(i, i*10)
			}(i)
			go func(i int64) {
				defer wg.Done()
				s.Load(i)
			}(i)
		}
		wg.Wait()

		for i := int64(0); i < N; i++ {
			v, ok := s.Load(i)
			if ok {
				assert.Equal(t, i*10, v)
			}
		}
	})

	t.Run("multiple operations interleaved", func(t *testing.T) {
		var s SyncMap[string, int]
		var wg sync.WaitGroup

		ops := []func(){
			func() { s.Store("a", 1) },
			func() { s.Store("b", 2) },
			func() { s.Load("a") },
			func() { s.Load("c") },
			func() { s.Delete("b") },
			func() { s.Delete("z") },
			func() { s.LoadOrStore("d", 4) },
			func() { s.LoadOrStore("a", 10) },
			func() { s.LoadAndDelete("a") },
			func() { s.LoadAndDelete("nonexistent") },
			func() {
				s.Range(func(k string, v int) bool {
					_ = k
					_ = v
					return true
				})
			},
		}

		for _, op := range ops {
			wg.Add(1)
			go func(op func()) {
				defer wg.Done()
				op()
			}(op)
		}
		wg.Wait()
	})
}

func TestMap_LoadOrStore_DoubleCheckRace(t *testing.T) {
	t.Run("deterministic double-check with manual lock", func(t *testing.T) {
		var m Map[string, int]

		// Manually init the map
		m.init()

		// Pre-insert under manual lock to simulate interleaved write
		m.mu.Lock()
		m.m["interleaved"] = 999
		m.mu.Unlock()

		// LoadOrStore on a pre-existing key hits the first exists check
		v, loaded := m.LoadOrStore("interleaved", 1)
		assert.True(t, loaded)
		assert.Equal(t, 999, v)
	})
}

// =============================================================================
// KVS interface satisfaction check
// =============================================================================

func TestKVSInterface(t *testing.T) {
	t.Run("Map satisfies KVS interface", func(t *testing.T) {
		var _ KVS[string, int] = (*Map[string, int])(nil)
		require.True(t, true)
	})

	t.Run("SyncMap satisfies KVS interface", func(t *testing.T) {
		var _ KVS[string, int] = (*SyncMap[string, int])(nil)
		require.True(t, true)
	})
}
