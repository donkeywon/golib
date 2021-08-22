package kvs

import (
	"database/sql"
	"errors"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/plugin"
	"github.com/donkeywon/golib/util/conv"
	"github.com/jmoiron/sqlx"
)

func init() {
	plugin.RegisterWithCfg(TypeSQLite3, func() interface{} { return NewSQLite3KVS() }, func() interface{} { return NewSQLite3KVSCfg() })
}

const (
	defaultTable = "kv"
	defaultDDL   = `CREATE TABLE IF NOT EXISTS ? (
    k          VARCHAR (255) NOT NULL,
    v          TEXT          NOT NULL,
    updated_at TIMESTAMP     NOT NULL
                             DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (
        k
    )
);`

	insertOrUpdateSQL = `INSERT INTO ? (
                   k,
                   v,
                   updated_at
               )
               VALUES (
                   ?,
                   ?,
                   CURRENT_TIMESTAMP
               )
               ON CONFLICT (
                   k
               )
               DO UPDATE SET k = excluded.k,
               v = excluded.v,
               updated_at = excluded.updated_at;`
	insertOrIgnoreSQL = `INSERT OR IGNORE INTO ? (k, v, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`
	deleteSQL         = "DELETE FROM ? where k = ?"
	selectSQL         = "SELECT rowid, v, updated_at FROM ? where k = ?"
	pageSelectSQL     = "SELECT rowid, k, v, updated_at FROM ? where rowid > ? LIMIT ?"

	TypeSQLite3 Type = "sqlite3"
)

type SQLite3KVSCfg struct {
	Path  string `json:"path"  yaml:"path" validate:"required"`
	Table string `json:"table" yaml:"table"`
	DDL   string `json:"ddl"   yaml:"ddl"`
}

func NewSQLite3KVSCfg() *SQLite3KVSCfg {
	return &SQLite3KVSCfg{
		Table: defaultTable,
		DDL:   defaultDDL,
	}
}

type SQLite3KVS struct {
	*SQLite3KVSCfg

	db *sqlx.DB
}

func NewSQLite3KVS() KVS {
	return &SQLite3KVS{}
}

func (s *SQLite3KVS) Open() error {
	var err error
	s.db, err = sqlx.Open("sqlite3", s.SQLite3KVSCfg.Path)
	if err != nil {
		return errs.Wrap(err, "open sqlite3 db fail")
	}
	s.db.SetMaxOpenConns(1)
	s.db.SetMaxIdleConns(1)
	s.db.MustExec(s.SQLite3KVSCfg.DDL, s.SQLite3KVSCfg.Table)
	return nil
}

func (s *SQLite3KVS) Close() error {
	err := s.db.Close()
	if err != nil {
		return errs.Wrap(err, "close sqlite3 db fail")
	}
	return nil
}

func (s *SQLite3KVS) insert(k string, v any, insertSQL string) (int64, error) {
	str, err := conv.AnyToString(v)
	if err != nil {
		return 0, errs.Wrap(err, "convert value to string fail")
	}

	r, err := s.db.Exec(insertSQL, s.SQLite3KVSCfg.Table, k, str)
	if err != nil {
		return 0, errs.Wrap(err, "sqlite3 insert fail")
	}
	return r.RowsAffected()
}

func (s *SQLite3KVS) get(k string) (*DBModel, error) {
	m := &DBModel{}
	err := s.db.Get(m, selectSQL, s.SQLite3KVSCfg.Table, k)
	if err == nil {
		return m, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return nil, errs.Wrap(err, "select from sqlite3 db fail")
}

func (s *SQLite3KVS) Store(k string, v any) error {
	return s.StoreAsString(k, v)
}

func (s *SQLite3KVS) StoreAsString(k string, v any) error {
	_, err := s.insert(k, v, insertOrUpdateSQL)
	return err
}

func (s *SQLite3KVS) Load(k string) (any, bool, error) {
	m, err := s.get(k)
	if err != nil {
		return nil, false, errs.Wrap(err, "sqlite3 get fail")
	}
	if m == nil {
		return nil, false, nil
	}
	return m.V, true, nil
}

func (s *SQLite3KVS) LoadOrStore(k string, v any) (any, bool, error) {
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

func (s *SQLite3KVS) LoadAndDelete(k string) (any, bool, error) {
	vv, loaded, err := s.Load(k)
	if err != nil {
		return nil, false, err
	}
	if !loaded {
		return nil, loaded, nil
	}

	return vv, loaded, s.Del(k)
}

func (s *SQLite3KVS) Del(k string) error {
	_, err := s.db.Exec(deleteSQL, s.SQLite3KVSCfg.Table, k)
	return err
}

func (s *SQLite3KVS) LoadAsBool(k string) (bool, error) {
	return LoadAsBool(s, k)
}

func (s *SQLite3KVS) LoadAsString(k string) (string, error) {
	return LoadAsString(s, k)
}

func (s *SQLite3KVS) LoadAsStringOr(k string, d string) (string, error) {
	return LoadAsStringOr(s, k, d)
}

func (s *SQLite3KVS) LoadAsInt(k string) (int, error) {
	return LoadAsInt(s, k)
}

func (s *SQLite3KVS) LoadAsIntOr(k string, d int) (int, error) {
	return LoadAsIntOr(s, k, d)
}

func (s *SQLite3KVS) LoadAsUint(k string) (uint, error) {
	return LoadAsUint(s, k)
}

func (s *SQLite3KVS) LoadAsUintOr(k string, d uint) (uint, error) {
	return LoadAsUintOr(s, k, d)
}

func (s *SQLite3KVS) LoadAsFloat(k string) (float64, error) {
	return LoadAsFloat(s, k)
}

func (s *SQLite3KVS) LoadAsFloatOr(k string, d float64) (float64, error) {
	return LoadAsFloatOr(s, k, d)
}

func (s *SQLite3KVS) LoadTo(k string, to any) error {
	return LoadTo(s, k, to)
}

func (s *SQLite3KVS) Collect() (map[string]any, error) {
	c := make(map[string]any)
	err := s.Range(func(k string, v any) bool {
		c[k] = v
		return true
	})
	return c, err
}

func (s *SQLite3KVS) Range(f func(k string, v any) bool) error {
	var startID int64 = -1
	pageSize := 100
	result := []DBModel{}
	for {
		err := s.db.Select(&result, pageSelectSQL, s.SQLite3KVSCfg.Table, startID, pageSize)
		if err != nil {
			return errs.Wrap(err, "sqlite3 page select fail")
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

func (s *SQLite3KVS) CollectAsString() (map[string]string, error) {
	var er error
	result := make(map[string]string)
	err := s.Range(func(k string, v any) bool {
		result[k], er = conv.AnyToString(v)
		return er == nil
	})
	if err == nil {
		err = er
	}
	return result, err
}

func (s *SQLite3KVS) Type() interface{} {
	return TypeSQLite3
}

func (s *SQLite3KVS) GetCfg() interface{} {
	return s.SQLite3KVSCfg
}
