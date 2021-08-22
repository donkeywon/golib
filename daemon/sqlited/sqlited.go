package sqlited

import (
	"plugin"

	"github.com/donkeywon/golib/errs"
	"github.com/donkeywon/golib/runner"
	"github.com/jmoiron/sqlx"
)

const DaemonTypeSQLited = "sqlited"

var (
	_s = &SQLited{
		Runner: runner.Create(string(DaemonTypeSQLited)),
	}
)

type SQLited struct {
	runner.Runner
	plugin.Plugin
	*Cfg
	*sqlx.DB

	initSQLs []string
}

func New() *SQLited {
	return _s
}

func (s *SQLited) Init() error {
	db, err := sqlx.Connect("sqlite3", s.Cfg.Path)
	if err != nil {
		return errs.Wrap(err, "open sqlite3 db fail")
	}

	s.DB = db

	for _, initSQL := range s.initSQLs {
		db.MustExec(initSQL)
	}

	return s.Runner.Init()
}

func (s *SQLited) Start() error {
	return s.Runner.Start()
}

func (s *SQLited) Stop() error {
	return s.DB.Close()
}

func (s *SQLited) Type() interface{} {
	return DaemonTypeSQLited
}

func (s *SQLited) GetCfg() interface{} {
	return s.Cfg
}

func (s *SQLited) InitSQL(sql ...string) {
	s.initSQLs = append(s.initSQLs, sql...)
}

func InitSQL(sql ...string) {
	_s.InitSQL(sql...)
}

func DB() *sqlx.DB {
	return _s.DB
}
