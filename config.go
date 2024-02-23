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
		fmt.Printf("%T %s - %T %s\n", key, key, ival, ival)
		switch val := ival.(type) {
		case map[string]interface{}:
			if err := config.unmarshal(name, val); err != nil {
				return err
			}
		default:
			if v := config.Argp.findLong(name); v != nil {
				fmt.Println("set", name, ival)
				vval := reflect.ValueOf(ival)
				if !vval.CanConvert(v.Value.Type()) {
					return fmt.Errorf("invalid type for %s, expected %v", name, v.Value.Type())
				}
				v.Value.Set(vval.Convert(v.Value.Type()))
			}
		}
	}
	return nil
}
