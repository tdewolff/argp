package argp

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pelletier/go-toml"
)

type ListSource interface {
	Has(string) bool
	List() []string
	Close() error
}

// List is an option that loads a list of values from a source (such as mysql).
type List struct {
	ListSource
	Values []string
}

func (list *List) Help() (string, string) {
	return strings.Join(list.Values, " "), "type:list"
}

func (list *List) Scan(name string, s []string) (int, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("missing value")
	}
	vals, _, split := truncEnd(s)
	if len(vals) == 0 || split {
		return 0, fmt.Errorf("invalid value")
	}

	colon := strings.IndexByte(vals[0], ':')
	if colon == -1 || (vals[0][0] < 'a' || 'z' < vals[0][0]) && (vals[0][0] < 'A' || 'Z' < vals[0][0]) {
		return 0, fmt.Errorf("invalid value, expected type:table where type is e.g. mysql")
	}
	list.Values = vals

	var err error
	typ := vals[0][:colon]
	vals[0] = vals[0][colon+1:]
	switch typ {
	case "inline":
		list.ListSource, err = newInlineList(vals)
	case "sqlite":
		list.ListSource, err = newSQLiteList(vals)
	case "mysql":
		list.ListSource, err = newMySQLList(vals)
	default:
		return 0, fmt.Errorf("unknown table type: %s", typ)
	}
	if err != nil {
		return 0, err
	}
	return len(vals), nil
}

type inlineList struct {
	list []string
}

func newInlineList(s []string) (ListSource, error) {
	list := []string{}
	if 0 < len(s) {
		if _, err := scanValue(reflect.ValueOf(&list).Elem(), s); err != nil {
			return nil, err
		}
	}
	return &inlineList{list}, nil
}

func (t *inlineList) Has(val string) bool {
	for _, item := range t.list {
		if item == val {
			return true
		}
	}
	return false
}

func (t *inlineList) List() []string {
	return t.list
}

func (t *inlineList) Close() error {
	return nil
}

type sqlList struct {
	db      *sqlx.DB
	stmt    *sqlx.Stmt
	stmtHas *sqlx.Stmt
}

func (t *sqlList) Has(val string) bool {
	if t.stmtHas != nil {
		return t.stmtHas.QueryRow(val).Err() == nil
	}

	list := t.List()
	for _, item := range list {
		if item == val {
			return true
		}
	}
	return false
}

func (t *sqlList) List() []string {
	var list []string
	if err := t.stmt.Select(&list); err != nil {
		return nil
	}
	return list
}

func (t *sqlList) Close() error {
	return t.db.Close()
}

type sqliteList struct {
	Path     string // can be :memory:
	Query    string
	QueryHas string
}

func newSQLiteList(s []string) (ListSource, error) {
	if len(s) != 1 {
		return nil, fmt.Errorf("invalid path")
	}

	b, err := os.ReadFile(s[0])
	if err != nil {
		return nil, err
	}

	t := sqliteList{}
	if err := toml.Unmarshal(b, &t); err != nil {
		return nil, err
	}

	db, err := sqlx.Open("sqlite", t.Path)
	if err != nil {
		return nil, err
	}

	stmt, err := db.Preparex(t.Query)
	if err != nil {
		return nil, err
	}
	var stmtHas *sqlx.Stmt
	if t.QueryHas != "" {
		if stmtHas, err = db.Preparex(t.QueryHas); err != nil {
			return nil, err
		}
	}
	return &sqlList{db, stmt, stmtHas}, nil
}

type mysqlList struct {
	Host     string
	User     string
	Password string
	Dbname   string
	Query    string
	QueryHas string
}

func newMySQLList(s []string) (ListSource, error) {
	if len(s) != 1 {
		return nil, fmt.Errorf("invalid path")
	}

	b, err := os.ReadFile(s[0])
	if err != nil {
		return nil, err
	}

	t := mysqlList{}
	if err := toml.Unmarshal(b, &t); err != nil {
		return nil, err
	}

	uri := fmt.Sprintf("%s:%s@%s/%s", t.User, t.Password, t.Host, t.Dbname)
	db, err := sqlx.Open("mysql", uri)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(time.Minute)
	db.SetConnMaxIdleTime(time.Minute)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	stmt, err := db.Preparex(t.Query)
	if err != nil {
		return nil, err
	}
	var stmtHas *sqlx.Stmt
	if t.QueryHas != "" {
		if stmtHas, err = db.Preparex(t.QueryHas); err != nil {
			return nil, err
		}
	}
	return &sqlList{db, stmt, stmtHas}, nil
}
