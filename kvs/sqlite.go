package kvs

import (
	"context"
	"fmt"
	"time"
	"unicode"

	"github.com/donkeywon/golib/errs"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

const (
	defaultPoolSize = 4 // allow concurrent readers; SQLite WAL supports this
	defaultTable    = "kv"
	defaultPageSize = 100

	// SQL templates — %s is the table name, substituted once in Open().
	ddlTpl = `CREATE TABLE IF NOT EXISTS %s (
    k          VARCHAR(255) NOT NULL,
    v          TEXT         NOT NULL,
    updated_at INTEGER      NOT NULL,
    PRIMARY KEY (k)
);
CREATE INDEX IF NOT EXISTS %s_idx_updated_at ON %s (updated_at);`

	// index name includes table name to avoid collisions when multiple
	// SQLiteKVS instances share the same database file.

	insertOrUpdateSQLTpl = `INSERT INTO %s (k, v, updated_at) VALUES (?, ?, ?)
                             ON CONFLICT (k) DO UPDATE SET
                                 v          = excluded.v,
                                 updated_at = excluded.updated_at`
	insertOrIgnoreSQLTpl  = `INSERT OR IGNORE INTO %s (k, v, updated_at) VALUES (?, ?, ?)`
	deleteSQLTpl          = `DELETE FROM %s WHERE k = ?`
	deleteReturningSQLTpl = `DELETE FROM %s WHERE k = ? RETURNING v` // atomic load-and-delete
	querySQLTpl           = `SELECT rowid, v, updated_at FROM %s WHERE k = ?`
	pageQuerySQLTpl       = `SELECT rowid, k, v, updated_at FROM %s WHERE rowid > ? LIMIT ?`
	cleanOutdatedSQLTpl   = `DELETE FROM %s WHERE updated_at < ?`
)

// DBModel is the raw row returned by SQLite queries.
type DBModel struct {
	RowID     int64
	K         string
	V         string
	UpdatedAt int64
}

// SQLiteKVSCfg holds configuration for a SQLiteKVS.
type SQLiteKVSCfg struct {
	Path     string `json:"path"     yaml:"path"     validate:"required"`
	Table    string `json:"table"    yaml:"table"`
	PoolSize int    `json:"poolSize" yaml:"poolSize"`
}

func NewSQLiteKVSCfg() *SQLiteKVSCfg {
	return &SQLiteKVSCfg{
		Table:    defaultTable,
		PoolSize: defaultPoolSize,
	}
}

func (c *SQLiteKVSCfg) setDefaults() {
	if c.Table == "" {
		c.Table = defaultTable
	}
	if c.PoolSize == 0 {
		c.PoolSize = defaultPoolSize
	}
}

// SQLiteKVS is a KVS backed by a SQLite database.
// Call Open before use and Close when done.
type SQLiteKVS struct {
	cfg *SQLiteKVSCfg

	pool *sqlitex.Pool

	// pre-formatted SQL strings — computed once in Open, never change thereafter.
	sqlInsertOrUpdate  string
	sqlInsertOrIgnore  string
	sqlDelete          string
	sqlDeleteReturning string
	sqlQuery           string
	sqlPageQuery       string
	sqlCleanOutdated   string
}

func NewSQLiteKVS(cfg *SQLiteKVSCfg) *SQLiteKVS {
	cfg.setDefaults()
	return &SQLiteKVS{
		cfg: cfg,
	}
}

// Open initialises the connection pool and creates the table/index if absent.
func (s *SQLiteKVS) Open(ctx context.Context) error {
	// Validate table name before any SQL is built — prevents identifier injection.
	if err := validateTableName(s.cfg.Table); err != nil {
		return errs.Wrap(err, "invalid table name")
	}

	// Pre-format all SQL once so that per-operation paths are allocation-free.
	s.sqlInsertOrUpdate = fmt.Sprintf(insertOrUpdateSQLTpl, s.cfg.Table)
	s.sqlInsertOrIgnore = fmt.Sprintf(insertOrIgnoreSQLTpl, s.cfg.Table)
	s.sqlDelete = fmt.Sprintf(deleteSQLTpl, s.cfg.Table)
	s.sqlDeleteReturning = fmt.Sprintf(deleteReturningSQLTpl, s.cfg.Table)
	s.sqlQuery = fmt.Sprintf(querySQLTpl, s.cfg.Table)
	s.sqlPageQuery = fmt.Sprintf(pageQuerySQLTpl, s.cfg.Table)
	s.sqlCleanOutdated = fmt.Sprintf(cleanOutdatedSQLTpl, s.cfg.Table)

	var err error
	s.pool, err = sqlitex.NewPool(s.cfg.Path, sqlitex.PoolOptions{PoolSize: s.cfg.PoolSize})
	if err != nil {
		return errs.Wrap(err, "open sqlite3 db failed")
	}

	conn, err := s.getConn(ctx)
	if err != nil {
		return errs.Wrap(err, "get conn failed")
	}
	defer s.putConn(conn)

	// Three %s: table name, index name prefix (same), table name in ON clause.
	ddl := fmt.Sprintf(ddlTpl, s.cfg.Table, s.cfg.Table, s.cfg.Table)
	if err = sqlitex.ExecuteScript(conn, ddl, &sqlitex.ExecOptions{}); err != nil {
		return errs.Wrap(err, "exec ddl failed")
	}
	return nil
}

// Close shuts down the connection pool.
func (s *SQLiteKVS) Close() error {
	if err := s.pool.Close(); err != nil {
		return errs.Wrap(err, "close sqlite3 db failed")
	}
	return nil
}

// validateTableName rejects names that contain characters other than letters,
// digits and underscores, preventing SQL injection via identifier interpolation.
func validateTableName(name string) error {
	if name == "" {
		return errs.New("table name must not be empty")
	}
	for _, c := range name {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
			return errs.Errorf("table name contains invalid character %q", c)
		}
	}
	return nil
}

func (s *SQLiteKVS) getConn(ctx context.Context) (*sqlite.Conn, error) {
	return s.pool.Take(ctx)
}

func (s *SQLiteKVS) putConn(c *sqlite.Conn) {
	s.pool.Put(c)
}

// withImmediateTx runs fn inside a BEGIN IMMEDIATE transaction on conn.
// Commits on success, rolls back (and re-panics) on failure.
func withImmediateTx(conn *sqlite.Conn, fn func() error) (err error) {
	if err = sqlitex.Execute(conn, "BEGIN IMMEDIATE", nil); err != nil {
		return errs.Wrap(err, "begin transaction failed")
	}
	defer func() {
		if p := recover(); p != nil {
			_ = sqlitex.Execute(conn, "ROLLBACK", nil)
			panic(p)
		}
		if err != nil {
			_ = sqlitex.Execute(conn, "ROLLBACK", nil)
			return
		}
		if cerr := sqlitex.Execute(conn, "COMMIT", nil); cerr != nil {
			_ = sqlitex.Execute(conn, "ROLLBACK", nil)
			err = errs.Wrap(cerr, "commit transaction failed")
		}
	}()
	return fn()
}

// Store converts v to a string and upserts it under k.
func (s *SQLiteKVS) Store(k string, v string) error {
	conn, err := s.getConn(context.Background())
	if err != nil {
		return errs.Wrap(err, "get conn failed")
	}
	defer s.putConn(conn)

	return sqlitex.Execute(conn, s.sqlInsertOrUpdate, &sqlitex.ExecOptions{
		Args: []any{k, v, time.Now().UnixNano()},
	})
}

// Load fetches the value stored under k.
func (s *SQLiteKVS) Load(k string) (string, bool, error) {
	conn, err := s.getConn(context.Background())
	if err != nil {
		return "", false, errs.Wrap(err, "get conn failed")
	}
	defer s.putConn(conn)

	var v string
	var found bool
	err = sqlitex.Execute(conn, s.sqlQuery, &sqlitex.ExecOptions{
		Args: []any{k},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			v = stmt.ColumnText(1)
			found = true
			return nil
		},
	})
	if err != nil {
		return "", false, errs.Wrap(err, "sqlite3 query failed")
	}
	if !found {
		return "", false, nil
	}
	return v, true, nil
}

// LoadOrStore atomically loads the existing value for k or stores v if absent.
//
// The entire check-and-insert runs inside a single BEGIN IMMEDIATE transaction
// on one connection, eliminating the TOCTOU race of separate Load+Store calls.
func (s *SQLiteKVS) LoadOrStore(k string, v string) (string, bool, error) {
	conn, err := s.getConn(context.Background())
	if err != nil {
		return "", false, errs.Wrap(err, "get conn failed")
	}
	defer s.putConn(conn)

	var actual string
	var loaded bool

	err = withImmediateTx(conn, func() error {
		// Check inside the transaction — no concurrent write can occur because
		// BEGIN IMMEDIATE holds a reserved lock on the database.
		qErr := sqlitex.Execute(conn, s.sqlQuery, &sqlitex.ExecOptions{
			Args: []any{k},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				actual = stmt.ColumnText(1)
				loaded = true
				return nil
			},
		})
		if qErr != nil {
			return qErr
		}
		if loaded {
			return nil // key already exists, skip insert
		}

		// Key is absent; insert it. Under IMMEDIATE lock, Changes() is always 1.
		iErr := sqlitex.Execute(conn, s.sqlInsertOrIgnore, &sqlitex.ExecOptions{
			Args: []any{k, v, time.Now().UnixNano()},
		})
		if iErr != nil {
			return iErr
		}
		actual = v
		return nil
	})
	if err != nil {
		return "", false, errs.Wrap(err, "sqlite3 load-or-store failed")
	}
	return actual, loaded, nil
}

// LoadAndDelete atomically removes k and returns its previous value.
//
// Uses DELETE ... RETURNING (SQLite ≥ 3.35) so the read and delete happen
// in a single statement, making it inherently atomic.
func (s *SQLiteKVS) LoadAndDelete(k string) (string, bool, error) {
	conn, err := s.getConn(context.Background())
	if err != nil {
		return "", false, errs.Wrap(err, "get conn failed")
	}
	defer s.putConn(conn)

	var v string
	var found bool
	err = sqlitex.Execute(conn, s.sqlDeleteReturning, &sqlitex.ExecOptions{
		Args: []any{k},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			v = stmt.ColumnText(0)
			found = true
			return nil
		},
	})
	if err != nil {
		return "", false, errs.Wrap(err, "sqlite3 load-and-delete failed")
	}
	return v, found, nil
}

// Del removes k from the store.
func (s *SQLiteKVS) Delete(k string) error {
	conn, err := s.getConn(context.Background())
	if err != nil {
		return errs.Wrap(err, "get conn failed")
	}
	defer s.putConn(conn)

	return sqlitex.Execute(conn, s.sqlDelete, &sqlitex.ExecOptions{
		Args: []any{k},
	})
}

// Range iterates over all key-value pairs in rowid order, page by page.
// Iteration stops when f returns false or all rows have been visited.
func (s *SQLiteKVS) Range(f func(k string, v string) bool) error {
	var startID int64 = -1
	for {
		rows, err := s.queryPage(startID, defaultPageSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for i := range rows {
			if !f(rows[i].K, rows[i].V) {
				return nil
			}
		}
		startID = rows[len(rows)-1].RowID
	}
}

// queryPage fetches one page of rows starting after startID.
// The connection is acquired and released with defer inside this helper,
// so a panic in sqlitex.Execute cannot leak the connection.
func (s *SQLiteKVS) queryPage(startID int64, pageSize int) ([]DBModel, error) {
	conn, err := s.getConn(context.Background())
	if err != nil {
		return nil, errs.Wrap(err, "get conn failed")
	}
	defer s.putConn(conn) // guaranteed even on panic

	// Pre-allocate with value type ([]DBModel, not []*DBModel) to avoid
	// one heap allocation per row.
	result := make([]DBModel, 0, pageSize)
	err = sqlitex.Execute(conn, s.sqlPageQuery, &sqlitex.ExecOptions{
		Args: []any{startID, pageSize},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			result = append(result, DBModel{
				RowID:     stmt.ColumnInt64(0),
				K:         stmt.ColumnText(1),
				V:         stmt.ColumnText(2),
				UpdatedAt: stmt.ColumnInt64(3),
			})
			return nil
		},
	})
	if err != nil {
		return nil, errs.Wrap(err, "sqlite3 page query failed")
	}
	return result, nil
}

// CleanupOutdated deletes rows whose updated_at is older than the given duration.
func (s *SQLiteKVS) CleanupOutdated(duration time.Duration) error {
	conn, err := s.getConn(context.Background())
	if err != nil {
		return errs.Wrap(err, "get conn failed")
	}
	defer s.putConn(conn)

	return sqlitex.Execute(conn, s.sqlCleanOutdated, &sqlitex.ExecOptions{
		Args: []any{time.Now().Add(-duration).UnixNano()},
	})
}
