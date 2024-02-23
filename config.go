package argp

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/pelletier/go-toml"
)

// Config is an option that sets all options from a configuration file.
type Config struct {
	Argp     *Argp
	Filename string
}

func (config *Config) Help() (string, string) {
	return config.Filename, "string"
}

func (config *Config) Scan(name string, s []string) (int, error) {
	n, err := scanValue(reflect.ValueOf(&config.Filename).Elem(), s)
	if err != nil {
		return n, err
	}

	f, err := os.Open(config.Filename)
	if err != nil {
		return n, err
	}
	defer f.Close()

	values := map[string]interface{}{}
	switch ext := filepath.Ext(config.Filename); ext {
	case ".toml":
		if err := toml.NewDecoder(f).Decode(&values); err != nil {
			return n, fmt.Errorf("toml: %v", err)
		}
	default:
		return n, fmt.Errorf("unknown configuration file extension: %s", ext)
	}

	if err := config.unmarshal("", values); err != nil {
		return n, err
	}
	return n, nil
}

func (config *Config) unmarshal(prefix string, values map[string]interface{}) error {
	for key, ival := range values {
		name := key
		if prefix != "" {
			name = prefix + "." + name
		}
		if val, ok := ival.(map[string]interface{}); ok {
			if err := config.unmarshal(name, val); err != nil {
				return err
			}
			continue
		}

		v := config.Argp.findLong(name)
		if v == nil {
			continue
		}

		vals := []string{}
		switch val := ival.(type) {
		case string:
			vals = splitArguments(val)
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, bool:
			vals = []string{fmt.Sprintf("%v", ival)}
		case []interface{}:
			vals = append(vals, "[")
			for _, v := range val {
				vals = append(vals, fmt.Sprintf("%v", v))
			}
			vals = append(vals, "]")
		default:
			return fmt.Errorf("%s: unknown type", name)
		}
		if n, err := scanVar(v.Value, name, vals); err != nil {
			return fmt.Errorf("%s: %v", name, err)
		} else if n != len(vals) {
			return fmt.Errorf("%s: invalid value", name)
		}
	}
	return nil
}
