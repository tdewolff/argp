package argp

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jmoiron/sqlx"
)

type ListSource interface {
	Has(string) bool
	List() []string
	Close() error
}

type ListSourceFunc func([]string) (ListSource, error)

// List is an option that loads a list of values from a source (such as mysql).
type List struct {
	ListSource
	Sources map[string]ListSourceFunc
	Values  []string
}

func NewList(values []string) *List {
	return &List{
		Sources: map[string]ListSourceFunc{
			"inline": NewInlineList,
		},
		Values: values,
	}
}

func (list *List) AddSource(typ string, f ListSourceFunc) {
	list.Sources[typ] = f
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
		return 0, fmt.Errorf("invalid value, expected type:list where type is e.g. inline")
	}
	list.Values = vals

	var err error
	typ := vals[0][:colon]
	vals[0] = vals[0][colon+1:]
	if ls, ok := list.Sources[typ]; !ok {
		return 0, fmt.Errorf("unknown list type: %s", typ)
	} else if list.ListSource, err = ls(vals); err != nil {
		return 0, err
	}
	return len(vals), nil
}

type InlineList struct {
	list []string
}

func NewInlineList(s []string) (ListSource, error) {
	list := []string{}
	if 0 < len(s) {
		if _, err := scanValue(reflect.ValueOf(&list).Elem(), s); err != nil {
			return nil, err
		}
	}
	return &InlineList{list}, nil
}

func (t *InlineList) Has(val string) bool {
	for _, item := range t.list {
		if item == val {
			return true
		}
	}
	return false
}

func (t *InlineList) List() []string {
	return t.list
}

func (t *InlineList) Close() error {
	return nil
}

type SQLList struct {
	db      *sqlx.DB
	stmt    *sqlx.Stmt
	stmtHas *sqlx.Stmt
}

func NewSQLList(db *sqlx.DB, query, queryHas string) (*SQLList, error) {
	stmt, err := db.Preparex(query)
	if err != nil {
		return nil, err
	}
	var stmtHas *sqlx.Stmt
	if queryHas != "" {
		if stmtHas, err = db.Preparex(queryHas); err != nil {
			return nil, err
		}
	}
	return &SQLList{
		db:      db,
		stmt:    stmt,
		stmtHas: stmtHas,
	}, nil
}

func (t *SQLList) Has(val string) bool {
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

func (t *SQLList) List() []string {
	var list []string
	if err := t.stmt.Select(&list); err != nil {
		return nil
	}
	return list
}

func (t *SQLList) Close() error {
	return t.db.Close()
}

//type sqliteList struct {
//	Path     string // can be :memory:
//	Query    string
//	QueryHas string
//}
//
//func newSQLiteList(s []string) (ListSource, error) {
//	if len(s) != 1 {
//		return nil, fmt.Errorf("invalid path")
//	}
//
//	t := sqliteList{}
//	if err := LoadConfigFile(&t, s[0]); err != nil {
//		return nil, err
//	}
//
//	db, err := sqlx.Open("sqlite", t.Path)
//	if err != nil {
//		return nil, err
//	}
//	return &sqlList{db, t.Query, t.QueryHas}, nil
//}
//
//type mysqlList struct {
//	Host     string
//	User     string
//	Password string
//	Dbname   string
//	Query    string
//	QueryHas string
//}
//
//func newMySQLList(s []string) (ListSource, error) {
//	if len(s) != 1 {
//		return nil, fmt.Errorf("invalid path")
//	}
//
//	t := mysqlList{}
//	if err := LoadConfigFile(&t, s[0]); err != nil {
//		return nil, err
//	}
//
//	uri := fmt.Sprintf("%s:%s@%s/%s", t.User, t.Password, t.Host, t.Dbname)
//	db, err := sqlx.Open("mysql", uri)
//	if err != nil {
//		return nil, err
//	}
//	db.SetConnMaxLifetime(time.Minute)
//	db.SetConnMaxIdleTime(time.Minute)
//	db.SetMaxOpenConns(10)
//	db.SetMaxIdleConns(10)
//	return &sqlList{db, t.Query, t.QueryHas}, nil
//}
