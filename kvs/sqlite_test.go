package kvs

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// openTestKVS opens a fresh SQLiteKVS backed by a temp file.
// The store is automatically closed when the test ends.
func openTestKVS(t *testing.T) *SQLiteKVS {
	t.Helper()
	cfg := NewSQLiteKVSCfg()
	cfg.Path = filepath.Join(t.TempDir(), "test.db")
	s := NewSQLiteKVS(cfg)
	require.NoError(t, s.Open(context.Background()))
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// closedKVS returns a SQLiteKVS whose pool has already been closed.
// After pool.Close(), pool.Take returns an error immediately, which lets us
// cover every "get conn failed" branch without mocking.
func closedKVS(t *testing.T) *SQLiteKVS {
	t.Helper()
	cfg := NewSQLiteKVSCfg()
	cfg.Path = filepath.Join(t.TempDir(), "test.db")
	s := NewSQLiteKVS(cfg)
	require.NoError(t, s.Open(context.Background()))
	require.NoError(t, s.Close())
	return s
}

// ---------------------------------------------------------------------------
// NewSQLiteKVSCfg
// ---------------------------------------------------------------------------

func TestNewSQLiteKVSCfg(t *testing.T) {
	cfg := NewSQLiteKVSCfg()
	assert.Equal(t, defaultTable, cfg.Table)
	assert.Equal(t, defaultPoolSize, cfg.PoolSize)
	assert.Empty(t, cfg.Path)
}

// ---------------------------------------------------------------------------
// setDefaults — tested indirectly through NewSQLiteKVS which calls it.
// ---------------------------------------------------------------------------

// TestSetDefaults_ZeroValues covers both if-body branches:
// Table=="" → defaultTable, PoolSize==0 → defaultPoolSize.
func TestSetDefaults_ZeroValues(t *testing.T) {
	cfg := &SQLiteKVSCfg{Path: "/tmp/x.db"} // Table="", PoolSize=0
	NewSQLiteKVS(cfg)
	assert.Equal(t, defaultTable, cfg.Table)
	assert.Equal(t, defaultPoolSize, cfg.PoolSize)
}

// TestSetDefaults_AlreadySet covers both if-condition-false branches:
// when Table and PoolSize are already non-zero the assignments are skipped.
func TestSetDefaults_AlreadySet(t *testing.T) {
	cfg := &SQLiteKVSCfg{Path: "/tmp/x.db", Table: "custom", PoolSize: 8}
	NewSQLiteKVS(cfg)
	assert.Equal(t, "custom", cfg.Table)
	assert.Equal(t, 8, cfg.PoolSize)
}

// ---------------------------------------------------------------------------
// validateTableName
// ---------------------------------------------------------------------------

func TestValidateTableName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// empty name branch
		{"empty", "", true},
		// invalid-character branch (loop body returns error)
		{"hyphen", "my-table", true},
		{"space", "my table", true},
		{"dot", "t.b", true},
		{"semicolon", "t;b", true},
		// valid names (return nil)
		{"letters only", "mytable", false},
		{"with digits", "table123", false},
		{"with underscore", "my_table_1", false},
		// unicode letters are accepted by unicode.IsLetter
		{"unicode letters", "表格", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTableName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Open / Close
// ---------------------------------------------------------------------------

// TestOpen_InvalidTableName covers the validateTableName-failure branch in Open.
func TestOpen_InvalidTableName(t *testing.T) {
	cfg := NewSQLiteKVSCfg()
	cfg.Path = filepath.Join(t.TempDir(), "test.db")
	cfg.Table = "bad-table"
	s := NewSQLiteKVS(cfg)

	err := s.Open(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid table name")
}

// TestOpen_InvalidPath covers the sqlitex.NewPool-failure branch in Open:
// a path whose parent directory does not exist cannot be created by SQLite.
func TestOpen_InvalidPath(t *testing.T) {
	cfg := NewSQLiteKVSCfg()
	cfg.Path = "/nonexistent_dir_kvs_test/test.db"
	s := NewSQLiteKVS(cfg)

	err := s.Open(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open sqlite3 db failed")
}

// TestOpen_Close covers the happy path for both Open and Close.
func TestOpen_Close(t *testing.T) {
	cfg := NewSQLiteKVSCfg()
	cfg.Path = filepath.Join(t.TempDir(), "test.db")
	s := NewSQLiteKVS(cfg)
	require.NoError(t, s.Open(context.Background()))
	require.NoError(t, s.Close())
}

// ---------------------------------------------------------------------------
// withImmediateTx
// ---------------------------------------------------------------------------

// TestWithImmediateTx_Success covers the normal path: fn returns nil → COMMIT.
func TestWithImmediateTx_Success(t *testing.T) {
	conn, err := sqlite.OpenConn(":memory:")
	require.NoError(t, err)
	defer conn.Close()

	called := false
	err = withImmediateTx(conn, func() error {
		called = true
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, called)
}

// TestWithImmediateTx_FnError covers the fn-returns-error branch:
// the deferred handler rolls back and propagates the error.
func TestWithImmediateTx_FnError(t *testing.T) {
	conn, err := sqlite.OpenConn(":memory:")
	require.NoError(t, err)
	defer conn.Close()

	sentinel := errors.New("fn failed")
	err = withImmediateTx(conn, func() error { return sentinel })
	require.Error(t, err)
	assert.ErrorContains(t, err, "fn failed")
}

// TestWithImmediateTx_FnPanic covers the recover() branch in the deferred
// handler: fn panics → ROLLBACK → re-panic.  After assert.Panics returns the
// connection must still be clean and reusable.
func TestWithImmediateTx_FnPanic(t *testing.T) {
	conn, err := sqlite.OpenConn(":memory:")
	require.NoError(t, err)
	defer conn.Close()

	assert.Panics(t, func() {
		_ = withImmediateTx(conn, func() error {
			panic("boom")
		})
	})

	// Connection must be clean after rollback.
	err = withImmediateTx(conn, func() error { return nil })
	assert.NoError(t, err)
}

// TestWithImmediateTx_BeginFails covers the first error branch: BEGIN IMMEDIATE
// fails when the connection is already inside a transaction.
func TestWithImmediateTx_BeginFails(t *testing.T) {
	conn, err := sqlite.OpenConn(":memory:")
	require.NoError(t, err)
	defer conn.Close()

	// Put the connection into an explicit transaction so BEGIN IMMEDIATE fails.
	require.NoError(t, sqlitex.Execute(conn, "BEGIN", nil))
	defer func() { _ = sqlitex.Execute(conn, "ROLLBACK", nil) }()

	err = withImmediateTx(conn, func() error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "begin transaction failed")
}

// ---------------------------------------------------------------------------
// Store / Load
// ---------------------------------------------------------------------------

func TestStore_Load_NewKey(t *testing.T) {
	s := openTestKVS(t)
	require.NoError(t, s.Store("k", "hello"))
	v, found, err := s.Load("k")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "hello", v)
}

// TestStore_Upsert covers the ON CONFLICT DO UPDATE branch (existing key).
func TestStore_Upsert(t *testing.T) {
	s := openTestKVS(t)
	require.NoError(t, s.Store("k", "first"))
	require.NoError(t, s.Store("k", "second"))
	v, found, err := s.Load("k")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "second", v)
}

// TestLoad_Missing covers the !found branch (returns "", false, nil).
func TestLoad_Missing(t *testing.T) {
	s := openTestKVS(t)
	v, found, err := s.Load("missing")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, v)
}

// TestStore_ConnFailed and TestLoad_ConnFailed cover the getConn-error branch
// by operating on a store whose pool has already been closed.
func TestStore_ConnFailed(t *testing.T) {
	s := closedKVS(t)
	err := s.Store("k", "v")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get conn failed")
}

func TestLoad_ConnFailed(t *testing.T) {
	s := closedKVS(t)
	_, _, err := s.Load("k")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get conn failed")
}

// ---------------------------------------------------------------------------
// LoadOrStore
// ---------------------------------------------------------------------------

// TestLoadOrStore_Missing covers the key-absent path inside the transaction:
// SELECT finds nothing → INSERT → loaded=false.
func TestLoadOrStore_Missing(t *testing.T) {
	s := openTestKVS(t)
	v, loaded, err := s.LoadOrStore("k", "stored")
	require.NoError(t, err)
	assert.False(t, loaded)
	assert.Equal(t, "stored", v)

	got, found, err := s.Load("k")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "stored", got)
}

// TestLoadOrStore_Existing covers the key-present path inside the transaction:
// SELECT finds the row → skip INSERT → loaded=true.
func TestLoadOrStore_Existing(t *testing.T) {
	s := openTestKVS(t)
	require.NoError(t, s.Store("k", "original"))

	v, loaded, err := s.LoadOrStore("k", "new")
	require.NoError(t, err)
	assert.True(t, loaded)
	assert.Equal(t, "original", v)
}

func TestLoadOrStore_ConnFailed(t *testing.T) {
	s := closedKVS(t)
	_, _, err := s.LoadOrStore("k", "v")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get conn failed")
}

// ---------------------------------------------------------------------------
// LoadAndDelete
// ---------------------------------------------------------------------------

// TestLoadAndDelete_Missing covers !found → ("", false, nil).
func TestLoadAndDelete_Missing(t *testing.T) {
	s := openTestKVS(t)
	v, found, err := s.LoadAndDelete("missing")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, v)
}

// TestLoadAndDelete_Existing covers found → (v, true, nil) and verifies the
// row is gone afterwards.
func TestLoadAndDelete_Existing(t *testing.T) {
	s := openTestKVS(t)
	require.NoError(t, s.Store("k", "bye"))

	v, found, err := s.LoadAndDelete("k")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "bye", v)

	_, found, err = s.Load("k")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestLoadAndDelete_ConnFailed(t *testing.T) {
	s := closedKVS(t)
	_, _, err := s.LoadAndDelete("k")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get conn failed")
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

// TestDelete_Absent covers the no-op path (0 rows affected, no error).
func TestDelete_Absent(t *testing.T) {
	s := openTestKVS(t)
	require.NoError(t, s.Delete("missing"))
}

// TestDelete_Existing covers the delete-existing-key path.
func TestDelete_Existing(t *testing.T) {
	s := openTestKVS(t)
	require.NoError(t, s.Store("k", "v"))
	require.NoError(t, s.Delete("k"))

	_, found, err := s.Load("k")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestDelete_ConnFailed(t *testing.T) {
	s := closedKVS(t)
	err := s.Delete("k")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get conn failed")
}

// ---------------------------------------------------------------------------
// Range
// ---------------------------------------------------------------------------

// TestRange_Empty covers the first queryPage returning 0 rows → immediate nil.
func TestRange_Empty(t *testing.T) {
	s := openTestKVS(t)
	called := false
	err := s.Range(func(_, _ string) bool {
		called = true
		return true
	})
	require.NoError(t, err)
	assert.False(t, called)
}

// TestRange_All covers the single-page path where f always returns true;
// the startID is advanced once and the second queryPage returns 0 rows.
func TestRange_All(t *testing.T) {
	s := openTestKVS(t)
	input := map[string]string{"a": "1", "b": "2", "c": "3"}
	for k, v := range input {
		require.NoError(t, s.Store(k, v))
	}

	got := make(map[string]string)
	err := s.Range(func(k, v string) bool {
		got[k] = v
		return true
	})
	require.NoError(t, err)
	assert.Equal(t, input, got)
}

// TestRange_EarlyStop covers the f-returns-false branch inside the inner loop.
func TestRange_EarlyStop(t *testing.T) {
	s := openTestKVS(t)
	for _, k := range []string{"a", "b", "c"} {
		require.NoError(t, s.Store(k, k))
	}

	count := 0
	err := s.Range(func(_, _ string) bool {
		count++
		return false
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestRange_MultiPage inserts more than defaultPageSize rows so that the outer
// loop in Range iterates more than once (startID advances across pages).
func TestRange_MultiPage(t *testing.T) {
	s := openTestKVS(t)
	const total = defaultPageSize + 5
	for i := 0; i < total; i++ {
		require.NoError(t, s.Store(fmt.Sprintf("key%04d", i), "v"))
	}

	count := 0
	err := s.Range(func(_, _ string) bool {
		count++
		return true
	})
	require.NoError(t, err)
	assert.Equal(t, total, count)
}

// TestRange_ConnFailed covers the queryPage getConn-error branch in Range:
// the pool is closed so the first queryPage call fails and Range propagates
// the error.
func TestRange_ConnFailed(t *testing.T) {
	s := closedKVS(t)
	err := s.Range(func(_, _ string) bool { return true })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get conn failed")
}

// ---------------------------------------------------------------------------
// CleanupOutdated
// ---------------------------------------------------------------------------

// TestCleanupOutdated covers both outcomes:
//   - rows older than the threshold are deleted
//   - rows newer than the threshold are kept
func TestCleanupOutdated(t *testing.T) {
	s := openTestKVS(t)

	require.NoError(t, s.Store("old", "v"))
	time.Sleep(60 * time.Millisecond)
	cutoff := time.Now()
	time.Sleep(60 * time.Millisecond)
	require.NoError(t, s.Store("new", "v"))

	// sinceOld ≈ 60 ms; cutoff ≈ (now − 60 ms).
	// CleanupOutdated deletes rows where updated_at < time.Now().Add(-sinceOld)
	// ≈ time.Now() − 60 ms ≈ cutoff, which is after "old" and before "new".
	sinceOld := time.Since(cutoff)
	require.NoError(t, s.CleanupOutdated(sinceOld))

	_, found, err := s.Load("old")
	require.NoError(t, err)
	assert.False(t, found, `"old" should have been cleaned up`)

	_, found, err = s.Load("new")
	require.NoError(t, err)
	assert.True(t, found, `"new" should still be present`)
}

func TestCleanupOutdated_ConnFailed(t *testing.T) {
	s := closedKVS(t)
	err := s.CleanupOutdated(time.Hour)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get conn failed")
}
