package argp

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Var struct {
	Value   reflect.Value
	Kind    reflect.Kind
	Long    string
	Short   rune
	Default interface{}
}

func (v *Var) SetString(s string) error {
	i, err := parseVar(v.Kind, s)
	if err != nil {
		return err
	}
	v.Set(i)
	return nil
}

func (v *Var) Set(i interface{}) {
	v.Value.Set(reflect.ValueOf(i).Convert(v.Value.Type()))
}

type Argp struct {
	cmds map[string]*Argp
	vars []*Var
}

func NewArgp() *Argp {
	return &Argp{
		cmds: map[string]*Argp{},
	}
}

func (argp *Argp) Add(i interface{}, short, long string, def interface{}) error {
	v := reflect.ValueOf(i)
	if v.Type().Kind() != reflect.Ptr {
		return fmt.Errorf("must pass pointer")
	}
	v = v.Elem()

	variable := &Var{}
	variable.Value = v
	variable.Kind = v.Kind()
	if long != "" {
		if !isValidName(long) {
			return fmt.Errorf("option names must be unicode letters or numbers")
		}
		variable.Long = strings.ToLower(long)
	}
	if short != "" {
		if !isValidName(short) {
			return fmt.Errorf("option names must be unicode letters or numbers")
		}
		r, n := utf8.DecodeRuneInString(short)
		if len(short) != n || n == 0 {
			return fmt.Errorf("short option names must be one character long")
		}
		variable.Short = r
	}
	if def != nil {
		variable.Default = def
	}
	argp.vars = append(argp.vars, variable)
	return nil
}

func (argp *Argp) AddStruct(i interface{}) error {
	v := reflect.ValueOf(i)
	if v.Type().Kind() != reflect.Ptr {
		return fmt.Errorf("must pass pointer to struct")
	}
	v = v.Elem()
	if v.Type().Kind() != reflect.Struct {
		return fmt.Errorf("must pass pointer to struct")
	}

	for j := 0; j < v.NumField(); j++ {
		tfield := v.Type().Field(j)
		vfield := v.Field(j)
		if vfield.IsValid() {
			variable := &Var{}
			variable.Value = vfield
			variable.Kind = vfield.Kind()
			variable.Long = strings.ToLower(tfield.Name)
			if long := tfield.Tag.Get("long"); long != "" {
				if !isValidName(long) {
					return fmt.Errorf("option names must be unicode letters or numbers")
				}
				variable.Long = strings.ToLower(long)
			}
			if short := tfield.Tag.Get("short"); short != "" {
				if !isValidName(short) {
					return fmt.Errorf("option names must be unicode letters or numbers")
				}
				r, n := utf8.DecodeRuneInString(short)
				if len(short) != n || n == 0 {
					return fmt.Errorf("short option names must be one character long")
				}
				variable.Short = r
			}
			if def := tfield.Tag.Get("default"); def != "" {
				iDef, err := parseVar(variable.Kind, def)
				if err != nil {
					return fmt.Errorf("bad option default: %w", err)
				}
				variable.Default = iDef
			}
			argp.vars = append(argp.vars, variable)
		}
	}
	return nil
}

func (argp *Argp) AddCommand(cmd string, sub *Argp) {
	argp.cmds[cmd] = sub
}

func (argp *Argp) Parse() []string {
	rest, err := argp.parse(os.Args[1:])
	if err != nil {
		fmt.Printf(err.Error())
		os.Exit(1)
		return nil
	}
	return rest
}

func (argp *Argp) findShort(short rune) *Var {
	short = unicode.ToLower(short)
	for _, v := range argp.vars {
		if v.Short != 0 && v.Short == short {
			return v
		}
	}
	return nil
}

func (argp *Argp) findLong(long string) *Var {
	long = strings.ToLower(long)
	for _, v := range argp.vars {
		if v.Long != "" && v.Long == long {
			return v
		}
	}
	return nil
}

func (argp *Argp) parse(args []string) ([]string, error) {
	// sub commands
	if 0 < len(args) {
		for cmd, sub := range argp.cmds {
			if cmd == args[0] {
				return sub.parse(args[1:])
			}
		}
	}

	// set default
	for _, v := range argp.vars {
		if v.Default != nil {
			v.Set(v.Default)
		}
	}

	rest := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if 1 < len(arg) && arg[0] == '-' {
			if 1 < len(arg) && arg[1] == '-' {
				name := arg[2:]
				if idx := strings.IndexByte(name, '='); idx != -1 {
					value := name[idx+1:]
					name = name[:idx]

					v := argp.findLong(name)
					if v == nil {
						return nil, fmt.Errorf("invalid option: %s", name)
					}
					if err := v.SetString(value); err != nil {
						return nil, fmt.Errorf("bad option %s: %v", name, err)
					}
				} else {
					v := argp.findLong(name)
					if v == nil {
						return nil, fmt.Errorf("invalid option: %s", name)
					} else if v.Kind == reflect.Bool {
						if err := v.SetString(""); err != nil {
							return nil, fmt.Errorf("bad option %s: %v", name, err)
						}
					} else if len(args) <= i+1 {
						return nil, fmt.Errorf("bad option %s: must have value", name)
					} else {
						i++
						if err := v.SetString(args[i]); err != nil {
							return nil, fmt.Errorf("bad option %s: %v", name, err)
						}
					}
				}
			} else {
				for j := 1; j < len(arg); {
					name, n := utf8.DecodeRuneInString(arg[j:])
					j += n

					v := argp.findShort(name)
					if v == nil {
						return nil, fmt.Errorf("invalid option: %c", name)
					} else if v.Kind == reflect.Bool {
						if err := v.SetString(""); err != nil {
							return nil, fmt.Errorf("bad option %c: %v", name, err)
						}
					} else {
						if j < len(arg) {
							if arg[j] == '=' {
								j++
							}
							if err := v.SetString(arg[j:]); err != nil {
								return nil, fmt.Errorf("bad option %c: %v", name, err)
							}
						} else if len(args) <= i+1 {
							return nil, fmt.Errorf("bad option %c: must have value", name)
						} else {
							i++
							if err := v.SetString(args[i]); err != nil {
								return nil, fmt.Errorf("bad option %c: %v", name, err)
							}
						}
						break
					}
				}
			}
		} else if 0 < len(arg) {
			rest = append(rest, arg)
		}
	}
	return rest, nil
}

func parseVar(kind reflect.Kind, s string) (interface{}, error) {
	switch kind {
	case reflect.String:
		return s, nil
	case reflect.Bool:
		if s == "" {
			return true, nil
		}
		return strconv.ParseBool(s)
	case reflect.Int:
		return strconv.ParseInt(s, 10, 0)
	case reflect.Int8:
		return strconv.ParseInt(s, 10, 8)
	case reflect.Int16:
		return strconv.ParseInt(s, 10, 16)
	case reflect.Int32:
		return strconv.ParseInt(s, 10, 32)
	case reflect.Int64:
		return strconv.ParseInt(s, 10, 64)
	case reflect.Uint:
		return strconv.ParseUint(s, 10, 0)
	case reflect.Uint8:
		return strconv.ParseUint(s, 10, 8)
	case reflect.Uint16:
		return strconv.ParseUint(s, 10, 16)
	case reflect.Uint32:
		return strconv.ParseUint(s, 10, 32)
	case reflect.Uint64:
		return strconv.ParseUint(s, 10, 64)
	case reflect.Float32:
		return strconv.ParseFloat(s, 32)
	case reflect.Float64:
		return strconv.ParseFloat(s, 64)
	}
	return nil, fmt.Errorf("unsupported type %s", kind)
}

func isValidName(s string) bool {
	for i, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' && 0 < i || r == '_') {
			return false
		}
	}
	return true
}
