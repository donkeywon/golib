package sqlitekvsloader

import (
	"context"
	"fmt"
	"time"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/kvs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/conv"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func init() {
	Load()
}

func Load() {
	plugin.RegisterWithCfg(TypeSQLite, func() interface{} { return NewSQLiteKVS() }, func() interface{} { return NewSQLiteKVSCfg() })
}

const (
	defaultPoolSize = 1
	defaultTable    = "kv"
	defaultDDL      = `CREATE TABLE IF NOT EXISTS %s (
    k          VARCHAR (255) NOT NULL,
    v          TEXT          NOT NULL,
    updated_at INTEGER       NOT NULL,
    PRIMARY KEY (
        k
    )
);`

	insertOrUpdateSQL = `INSERT INTO %s (
                   k,
                   v,
                   updated_at
               )
               VALUES (
                   ?,
                   ?,
                   ?
               )
               ON CONFLICT (
                   k
               )
               DO UPDATE SET k = excluded.k,
               v = excluded.v,
               updated_at = excluded.updated_at;`
	insertOrIgnoreSQL = `INSERT OR IGNORE INTO %s (k, v, updated_at) VALUES (?, ?, ?)`
	deleteSQL         = "DELETE FROM %s where k = ?"
	querySQL          = "SELECT rowid, v, updated_at FROM %s where k = ?"
	pageQuerySQL      = "SELECT rowid, k, v, updated_at FROM %s where rowid > ? LIMIT ?"

	TypeSQLite kvs.Type = "sqlite"
)

type DBModel struct {
	RowID     int64
	K         string
	V         string
	UpdatedAt int64
}

type SQLiteKVSCfg struct {
	Path     string `json:"path"     yaml:"path" validate:"required"`
	Table    string `json:"table"    yaml:"table"`
	PoolSize int    `json:"poolSize" yaml:"poolSize"`
}

func NewSQLiteKVSCfg() *SQLiteKVSCfg {
	return &SQLiteKVSCfg{
		Table:    defaultTable,
		PoolSize: defaultPoolSize,
	}
}

type SQLiteKVS struct {
	*SQLiteKVSCfg

	pool *sqlitex.Pool
}

func NewSQLiteKVS() *SQLiteKVS {
	return &SQLiteKVS{}
}

func (s *SQLiteKVS) Open() error {
	var err error
	s.pool, err = sqlitex.NewPool(s.Path, sqlitex.PoolOptions{PoolSize: s.PoolSize})
	if err != nil {
		return errs.Wrap(err, "open sqlite3 db fail")
	}
	conn, err := s.getConn()
	if err != nil {
		return errs.Wrap(err, "get conn fail")
	}
	defer s.putConn(conn)
	err = sqlitex.Execute(conn, s.prepareSQL(defaultDDL), &sqlitex.ExecOptions{})
	if err != nil {
		return errs.Wrap(err, "exec ddl fail")
	}
	return nil
}

func (s *SQLiteKVS) Close() error {
	err := s.pool.Close()
	if err != nil {
		return errs.Wrap(err, "close sqlite3 db fail")
	}
	return nil
}

func (s *SQLiteKVS) insert(k string, v any, insertSQL string) (int64, error) {
	str, err := conv.ToString(v)
	if err != nil {
		return 0, errs.Wrap(err, "convert value to string fail")
	}

	conn, err := s.getConn()
	if err != nil {
		return 0, errs.Wrap(err, "get conn fail")
	}
	defer s.putConn(conn)

	err = sqlitex.Execute(conn, s.prepareSQL(insertSQL), &sqlitex.ExecOptions{
		Args: []any{k, str, time.Now().UnixNano()},
	})
	if err != nil {
		return 0, errs.Wrap(err, "sqlite3 insert fail")
	}
	return int64(conn.Changes()), nil
}

func (s *SQLiteKVS) query(k string) (*DBModel, error) {
	conn, err := s.getConn()
	if err != nil {
		return nil, errs.Wrap(err, "get conn fail")
	}
	defer s.putConn(conn)

	var m *DBModel
	err = sqlitex.Execute(conn, s.prepareSQL(querySQL), &sqlitex.ExecOptions{
		Args: []any{k},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			m = &DBModel{
				RowID:     stmt.ColumnInt64(0),
				K:         k,
				V:         stmt.ColumnText(1),
				UpdatedAt: stmt.ColumnInt64(2),
			}
			return nil
		},
	})
	if err != nil {
		return nil, errs.Wrap(err, "query fail")
	}

	return m, nil
}

func (s *SQLiteKVS) getConn() (*sqlite.Conn, error) {
	return s.pool.Take(context.Background())
}

func (s *SQLiteKVS) putConn(c *sqlite.Conn) {
	s.pool.Put(c)
}

func (s *SQLiteKVS) Store(k string, v any) error {
	return s.StoreAsString(k, v)
}

func (s *SQLiteKVS) StoreAsString(k string, v any) error {
	_, err := s.insert(k, v, insertOrUpdateSQL)
	return err
}

func (s *SQLiteKVS) Load(k string) (any, bool, error) {
	m, err := s.query(k)
	if err != nil {
		return nil, false, errs.Wrap(err, "sqlite3 get fail")
	}
	if m == nil {
		return nil, false, nil
	}
	return m.V, true, nil
}

func (s *SQLiteKVS) LoadOrStore(k string, v any) (any, bool, error) {
	vv, loaded, err := s.Load(k)
	if err != nil {
		return nil, false, err
	}
	if loaded {
		return vv, loaded, nil
	}

	affected, err := s.insert(k, v, insertOrIgnoreSQL)
	if err != nil {
		return nil, false, err
	}
	if affected == 1 {
		return v, false, nil
	}
	return s.Load(k)
}

func (s *SQLiteKVS) LoadAndDelete(k string) (any, bool, error) {
	vv, loaded, err := s.Load(k)
	if err != nil {
		return nil, false, err
	}
	if !loaded {
		return nil, loaded, nil
	}

	return vv, loaded, s.Del(k)
}

func (s *SQLiteKVS) Del(k string) error {
	conn, err := s.getConn()
	if err != nil {
		return errs.Wrap(err, "get conn fail")
	}
	defer s.putConn(conn)
	return sqlitex.Execute(conn, s.prepareSQL(deleteSQL), &sqlitex.ExecOptions{
		Args: []any{k},
	})
}

func (s *SQLiteKVS) LoadAsBool(k string) (bool, error) {
	return kvs.LoadAsBool(s, k)
}

func (s *SQLiteKVS) LoadAsString(k string) (string, error) {
	return kvs.LoadAsString(s, k)
}

func (s *SQLiteKVS) LoadAsStringOr(k string, d string) (string, error) {
	return kvs.LoadAsStringOr(s, k, d)
}

func (s *SQLiteKVS) LoadAsInt(k string) (int, error) {
	return kvs.LoadAsInt(s, k)
}

func (s *SQLiteKVS) LoadAsIntOr(k string, d int) (int, error) {
	return kvs.LoadAsIntOr(s, k, d)
}

func (s *SQLiteKVS) LoadAsUint(k string) (uint, error) {
	return kvs.LoadAsUint(s, k)
}

func (s *SQLiteKVS) LoadAsUintOr(k string, d uint) (uint, error) {
	return kvs.LoadAsUintOr(s, k, d)
}

func (s *SQLiteKVS) LoadAsFloat(k string) (float64, error) {
	return kvs.LoadAsFloat(s, k)
}

func (s *SQLiteKVS) LoadAsFloatOr(k string, d float64) (float64, error) {
	return kvs.LoadAsFloatOr(s, k, d)
}

func (s *SQLiteKVS) LoadAll() (map[string]any, error) {
	c := make(map[string]any)
	err := s.Range(func(k string, v any) bool {
		c[k] = v
		return true
	})
	return c, err
}

func (s *SQLiteKVS) Range(f func(k string, v any) bool) error {
	var startID int64 = -1
	pageSize := 100
	result := make([]*DBModel, 0, pageSize)
	for {
		conn, err := s.getConn()
		if err != nil {
			return errs.Wrap(err, "get conn fail")
		}

		result = result[:0]
		err = sqlitex.Execute(conn, s.prepareSQL(pageQuerySQL), &sqlitex.ExecOptions{
			Args: []any{startID, pageSize},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				result = append(result, &DBModel{
					RowID:     stmt.ColumnInt64(0),
					K:         stmt.ColumnText(1),
					V:         stmt.ColumnText(2),
					UpdatedAt: stmt.ColumnInt64(3),
				})
				return nil
			},
		})
		s.putConn(conn)

		if err != nil {
			return errs.Wrap(err, "sqlite3 page query fail")
		}

		if len(result) == 0 {
			break
		}

		for _, r := range result {
			if !f(r.K, r.V) {
				break
			}
		}

		startID = result[len(result)-1].RowID
	}

	return nil
}

func (s *SQLiteKVS) LoadAllAsString() (map[string]string, error) {
	var er error
	result := make(map[string]string)
	err := s.Range(func(k string, v any) bool {
		result[k], er = conv.ToString(v)
		return er == nil
	})
	if err == nil {
		err = er
	}
	return result, err
}

func (s *SQLiteKVS) Type() interface{} {
	return TypeSQLite
}

func (s *SQLiteKVS) GetCfg() interface{} {
	return s.SQLiteKVSCfg
}

func (s *SQLiteKVS) prepareSQL(sql string) string {
	return fmt.Sprintf(sql, s.Table)
}
