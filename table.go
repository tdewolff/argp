package argp

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/pelletier/go-toml"
)

type TableSource interface {
	Has(string) bool
	Get(string) string
	Close() error
}

// Table is an option that loads key-value map from a source (such as mysql).
type Table struct {
	TableSource
	Values []string
}

func (table *Table) Help() (string, string) {
	return strings.Join(table.Values, " "), "type:table"
}

func (table *Table) Scan(name string, s []string) (int, error) {
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
	table.Values = vals

	var err error
	typ := vals[0][:colon]
	vals[0] = vals[0][colon+1:]
	switch typ {
	case "static":
		table.TableSource, err = newStaticTable(vals)
	case "inline":
		table.TableSource, err = newInlineTable(vals)
	case "sqlite":
		table.TableSource, err = newSQLiteTable(vals)
	case "mysql":
		table.TableSource, err = newMySQLTable(vals)
	default:
		return 0, fmt.Errorf("unknown table type: %s", typ)
	}
	if err != nil {
		return 0, err
	}
	return len(vals), nil
}

type staticTable struct {
	value string
}

func newStaticTable(s []string) (TableSource, error) {
	return &staticTable{strings.Join(s, " ")}, nil
}

func (t *staticTable) Has(key string) bool {
	return true
}

func (t *staticTable) Get(key string) string {
	return t.value
}

func (t *staticTable) Close() error {
	return nil
}

type inlineTable struct {
	table map[string]string
}

func newInlineTable(s []string) (TableSource, error) {
	table := map[string]string{}
	if 0 < len(s) {
		if _, err := scanValue(reflect.ValueOf(&table).Elem(), s); err != nil {
			return nil, err
		}
	}
	return &inlineTable{table}, nil
}

func (t *inlineTable) Has(key string) bool {
	_, ok := t.table[key]
	return ok
}

func (t *inlineTable) Get(key string) string {
	return t.table[key]
}

func (t *inlineTable) Close() error {
	return nil
}

type sqlTable struct {
	db   *sqlx.DB
	stmt *sqlx.Stmt
}

func (t *sqlTable) Has(key string) bool {
	return t.stmt.QueryRow(key).Err() == nil
}

func (t *sqlTable) Get(key string) string {
	var val string // TODO: does this work for ints? Or should we use interface{}?
	if err := t.stmt.QueryRow(key).Scan(&val); err != nil {
		return ""
	}
	return val
}

func (t *sqlTable) Close() error {
	return t.db.Close()
}

type sqliteTable struct {
	Path  string // can be :memory:
	Query string
}

func newSQLiteTable(s []string) (TableSource, error) {
	if len(s) != 1 {
		return nil, fmt.Errorf("invalid path")
	}

	b, err := os.ReadFile(s[0])
	if err != nil {
		return nil, err
	}

	t := sqliteTable{}
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
	return &sqlTable{db, stmt}, nil
}

type mysqlTable struct {
	Host     string
	User     string
	Password string
	Dbname   string
	Query    string
}

func newMySQLTable(s []string) (TableSource, error) {
	if len(s) != 1 {
		return nil, fmt.Errorf("invalid path")
	}

	b, err := os.ReadFile(s[0])
	if err != nil {
		return nil, err
	}

	t := mysqlTable{}
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
	return &sqlTable{db, stmt}, nil
}
