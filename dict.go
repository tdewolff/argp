package argp

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"github.com/jmoiron/sqlx"
)

type DictSource interface {
	Has(string) (bool, error)
	Get(string) (string, error)
	Close() error
}

type DictSourceFunc func([]string) (DictSource, error)

// Dict is an option that loads key-value map from a source (such as mysql).
type Dict struct {
	DictSource
	Sources map[string]DictSourceFunc
	Values  []string
}

func NewDict(values []string) *Dict {
	return &Dict{
		Sources: map[string]DictSourceFunc{
			"static": NewStaticDict,
			"inline": NewInlineDict,
		},
		Values: values,
	}
}

func (dict *Dict) AddSource(typ string, f DictSourceFunc) {
	dict.Sources[typ] = f
}

func (dict *Dict) Help() (string, string) {
	return strings.Join(dict.Values, " "), "type:dict"
}

func (dict *Dict) Scan(name string, s []string) (int, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("missing value")
	}
	vals, _, split := truncEnd(s)
	if len(vals) == 0 || split {
		return 0, fmt.Errorf("invalid value")
	}

	colon := strings.IndexByte(vals[0], ':')
	if colon == -1 || (vals[0][0] < 'a' || 'z' < vals[0][0]) && (vals[0][0] < 'A' || 'Z' < vals[0][0]) {
		return 0, fmt.Errorf("invalid value, expected type:dict where type is e.g. inline")
	}
	dict.Values = vals

	var err error
	typ := vals[0][:colon]
	vals[0] = vals[0][colon+1:]
	if ts, ok := dict.Sources[typ]; !ok {
		return 0, fmt.Errorf("unknown dict type: %s", typ)
	} else if dict.DictSource, err = ts(vals); err != nil {
		return 0, err
	}
	return len(vals), nil
}

func (dict *Dict) Close() error {
	if dict.DictSource != nil {
		return dict.DictSource.Close()
	}
	return nil
}

type StaticDict struct {
	value string
}

func NewStaticDict(s []string) (DictSource, error) {
	return &StaticDict{strings.Join(s, " ")}, nil
}

func (t *StaticDict) Has(key string) (bool, error) {
	return true, nil
}

func (t *StaticDict) Get(key string) (string, error) {
	return t.value, nil
}

func (t *StaticDict) Close() error {
	return nil
}

type InlineDict struct {
	dict map[string]string
}

func NewInlineDict(s []string) (DictSource, error) {
	dict := map[string]string{}
	if 0 < len(s) {
		if _, err := scanValue(reflect.ValueOf(&dict).Elem(), s); err != nil {
			return nil, err
		}
	}
	return &InlineDict{dict}, nil
}

func (t *InlineDict) Has(key string) (bool, error) {
	_, ok := t.dict[key]
	return ok, nil
}

func (t *InlineDict) Get(key string) (string, error) {
	v, ok := t.dict[key]
	if !ok {
		return key, nil
	}
	return v, nil
}

func (t *InlineDict) Close() error {
	return nil
}

type SQLDict struct {
	db    *sqlx.DB
	query string
}

func NewSQLDict(db *sqlx.DB, query string) (*SQLDict, error) {
	return &SQLDict{
		db:    db,
		query: query,
	}, nil
}

func (t *SQLDict) Has(key string) (bool, error) {
	if t.query == "" {
		return false, nil
	} else if err := t.db.QueryRow(t.query, key).Err(); err != nil && err != sql.ErrNoRows {
		return false, err
	}
	return true, nil
}

func (t *SQLDict) Get(key string) (string, error) {
	var val string // TODO: does this work for ints? Or should we use interface{}?
	if t.query == "" {
		return "", nil
	} else if err := t.db.Get(&val, t.query, key); err != nil && err != sql.ErrNoRows {
		return "", err
	} else if err == sql.ErrNoRows {
		return key, nil
	} else {
		return val, nil
	}
}

func (t *SQLDict) Close() error {
	return t.db.Close()
}

//type sqliteDict struct {
//	Path  string // can be :memory:
//	Query string
//}
//
//func newSQLiteDict(s []string) (DictSource, error) {
//	if len(s) != 1 {
//		return nil, fmt.Errorf("invalid path")
//	}
//
//	t := sqliteDict{}
//	if err := LoadConfigFile(&t, s[0]); err != nil {
//		return nil, err
//	}
//
//	db, err := sqlx.Open("sqlite", t.Path)
//	if err != nil {
//		return nil, err
//	}
//	return &sqlDict{db, t.Query}, nil
//}
//
//type mysqlDict struct {
//	Host    string
//	User     string
//	Password string
//	Dbname   string
//	Query    string
//}
//
//func newMySQLDict(s []string) (DictSource, error) {
//	if len(s) != 1 {
//		return nil, fmt.Errorf("invalid path")
//	}
//
//	t := mysqlDict{}
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
//	return &sqlDict{db, t.Query}, nil
//}
