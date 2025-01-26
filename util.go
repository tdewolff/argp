package argp

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v3"
)

// LoadConfigFile loads .cf, .cfg, .toml, and .yaml files.
func LoadConfigFile(dst interface{}, filename string) error {
	b, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	switch filepath.Ext(filename) {
	case ".cf", ".cfg":
		if err := UnmarshalConfig(b, dst); err != nil {
			return err
		}
	case ".toml":
		if err := toml.Unmarshal(b, dst); err != nil {
			return err
		}
	case ".yaml":
		if err := yaml.Unmarshal(b, dst); err != nil {
			return err
		}
	}
	return nil
}

// UnmarshalConfig parses simple .cf or .cfg files.
func UnmarshalConfig(b []byte, dst interface{}) error {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("destination must be a pointer to struct")
	}
	v = v.Elem()

	n := 0
	s := bufio.NewScanner(bytes.NewReader(b))
	for s.Scan() {
		n++
		line := s.Text()
		if len(line) == 0 || line[0] == '#' {
			// empty line or comment
			continue
		}
		is := strings.IndexByte(line, '=')
		if is == -1 {
			return fmt.Errorf("line %v: missing =", n)
		}
		key := strings.TrimSpace(line[:is])
		val := strings.TrimSpace(line[is+1:])
		if len(key) == 0 {
			return fmt.Errorf("line %v: empty key", n)
		}
		field := v.FieldByName(key)
		if !field.IsValid() {
			field = v.FieldByName(strings.ToUpper(key[:1]) + key[1:])
			if !field.IsValid() {
				continue
			}
		}
		switch field.Kind() {
		case reflect.String:
			field.SetString(val)
		case reflect.Bool:
			i, err := strconv.ParseBool(val)
			if err != nil {
				return fmt.Errorf("invalid boolean '%v'", val)
			}
			field.SetBool(i)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid integer '%v'", val)
			}
			field.SetInt(i)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			i, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid positive integer '%v'", val)
			}
			field.SetUint(i)
		case reflect.Float32, reflect.Float64:
			i, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return fmt.Errorf("invalid number '%v'", val)
			}
			field.SetFloat(i)
		default:
			return fmt.Errorf("type of field in destination not supported: %v", field.Type())
		}
	}
	return s.Err()
}
