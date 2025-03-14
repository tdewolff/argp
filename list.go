package argp

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type ListSource interface {
	Has(string) (bool, error)
	List() ([]string, error)
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

func (list *List) Valid() bool {
	return list.ListSource != nil
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

func (list *List) Close() error {
	if list.ListSource != nil {
		return list.ListSource.Close()
	}
	return nil
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

func (t *InlineList) Has(val string) (bool, error) {
	for _, item := range t.list {
		if item == val {
			return true, nil
		}
	}
	return false, nil
}

func (t *InlineList) List() ([]string, error) {
	return t.list, nil
}

func (t *InlineList) Close() error {
	return nil
}

type SQLList struct {
	db       *sqlx.DB
	query    string
	queryHas string
	cacheDur time.Duration

	cache     []string
	lastQuery time.Time
}

func NewSQLList(db *sqlx.DB, query, queryHas string, cacheDur time.Duration) (*SQLList, error) {
	return &SQLList{
		db:       db,
		query:    query,
		queryHas: queryHas,
		cacheDur: cacheDur,
	}, nil
}

func (t *SQLList) Has(val string) (bool, error) {
	if t.queryHas != "" {
		if err := t.db.QueryRow(t.queryHas, val).Err(); err != nil && err != sql.ErrNoRows {
			return false, err
		} else {
			return err != sql.ErrNoRows, nil
		}
	}
	list, err := t.List()
	if err != nil {
		return false, err
	}
	for _, item := range list {
		if item == val {
			return true, nil
		}
	}
	return false, nil
}

func (t *SQLList) List() ([]string, error) {
	var list []string
	if t.query == "" {
		return nil, nil
	} else if time.Since(t.lastQuery) < t.cacheDur || t.cacheDur < 0 && !t.lastQuery.IsZero() {
		return t.cache, nil
	} else if err := t.db.Select(&list, t.query); err != nil {
		return nil, err
	}
	t.cache = list
	t.lastQuery = time.Now()
	return list, nil
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
